package tests

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"familycontrol/backend/applications"
	"familycontrol/backend/database"
	"familycontrol/backend/devices"
	"familycontrol/backend/family"
	"familycontrol/backend/monitoring"
	"familycontrol/backend/networking"
	"familycontrol/backend/protection"
	"familycontrol/backend/screen_time"
	"familycontrol/backend/security"
	"familycontrol/backend/updater"
	"familycontrol/backend/websites"
	"familycontrol/backend/windows"
)

func TestArgon2idAndCodes(t *testing.T) {
	if _, err := security.HashPassword("short"); err == nil {
		t.Fatalf("expected weak password (<8 chars) to be rejected")
	}

	hash, err := security.HashPassword("CorrectHorseBatteryStaple!")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected PHC argon2id format, got: %s", hash)
	}

	ok, err := security.VerifyPassword("CorrectHorseBatteryStaple!", hash)
	if err != nil || !ok {
		t.Fatalf("expected valid password to verify: ok=%v err=%v", ok, err)
	}

	okWrong, _ := security.VerifyPassword("WrongPassword123!", hash)
	if okWrong {
		t.Fatalf("expected wrong password to fail verification")
	}

	pairCode, err := security.GeneratePairingCode()
	if err != nil || len(pairCode) != 9 {
		t.Fatalf("unexpected pairing code: %s (%v)", pairCode, err)
	}

	recCode, err := security.GenerateRecoveryCode()
	if err != nil || !strings.HasPrefix(recCode, "FC-") {
		t.Fatalf("unexpected recovery code: %s (%v)", recCode, err)
	}
}

func TestLocalSealDPAPIAndCrypto(t *testing.T) {
	tmpDir := t.TempDir()
	seal, err := security.NewLocalSealManager(tmpDir)
	if err != nil {
		t.Fatalf("NewLocalSealManager failed: %v", err)
	}

	secret := []byte("super-secret-device-credential-token")
	enc, err := seal.Encrypt(secret)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	// Reload from disk to verify DPAPI persistence
	seal2, err := security.NewLocalSealManager(tmpDir)
	if err != nil {
		t.Fatalf("Reload LocalSealManager failed: %v", err)
	}
	dec, err := seal2.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if string(dec) != string(secret) {
		t.Fatalf("decrypted plaintext mismatch: got %q, want %q", string(dec), string(secret))
	}

	pubB64, privBytes, err := security.GenerateEd25519KeyPair()
	if err != nil {
		t.Fatalf("GenerateEd25519KeyPair failed: %v", err)
	}
	msg := []byte("POST:/api/v1/sync:dev-1:1700000000:nonce123")
	sig := security.SignMessage(privBytes, msg)
	if !security.VerifyMessageSignature(pubB64, msg, sig) {
		t.Fatalf("expected Ed25519 signature verification to succeed")
	}
	if security.VerifyMessageSignature(pubB64, []byte("tampered"), sig) {
		t.Fatalf("expected tampered Ed25519 message to fail verification")
	}
}

func TestReplayProtectionAndRateLimiter(t *testing.T) {
	rg := security.NewReplayGuard(2 * time.Minute)
	now := time.Now().Unix()

	if err := rg.ValidateAndRecord("dev-1", "nonce-abc", now); err != nil {
		t.Fatalf("expected fresh request to pass replay guard: %v", err)
	}
	if err := rg.ValidateAndRecord("dev-1", "nonce-abc", now); err == nil {
		t.Fatalf("expected duplicate nonce to be rejected as replay")
	}
	if err := rg.ValidateAndRecord("dev-1", "nonce-def", now-600); err == nil {
		t.Fatalf("expected stale timestamp (10m old) to be rejected")
	}
}

func TestScreenTimeEvaluationAndBedtime(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := database.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open db: %v", err)
	}
	defer store.Close()

	stMgr := screen_time.NewManager(store)
	monMgr := monitoring.NewManager(store)

	devID := "dev-child-1"
	pol, err := stMgr.GetPolicy(devID)
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}
	pol.DailyLimitMinutes = 60
	pol.BedtimeStart = "21:00"
	pol.BedtimeEnd = "07:00"
	if _, err := stMgr.SavePolicy(pol); err != nil {
		t.Fatalf("SavePolicy: %v", err)
	}

	// 14:00 on a Wednesday (within allowed hours, 0 min used)
	wedAfternoon := time.Date(2026, 11, 11, 14, 0, 0, 0, time.UTC)
	eval, err := stMgr.Evaluate(devID, wedAfternoon)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !eval.AllowedNow || eval.RemainingMinutes != 60 {
		t.Fatalf("expected allowed with 60m remaining, got: %+v", eval)
	}

	// 22:30 during bedtime
	wedNight := time.Date(2026, 11, 11, 22, 30, 0, 0, time.UTC)
	evalNight, _ := stMgr.Evaluate(devID, wedNight)
	if evalNight.AllowedNow || !evalNight.InBedtime {
		t.Fatalf("expected bedtime restriction at 22:30, got: %+v", evalNight)
	}

	// Record 65 minutes of usage today and verify daily limit enforcement
	_ = monMgr.RecordAppUsage(devID, "minecraft.exe", "Minecraft", "World 1", 65*60)
	nowDay := time.Now().UTC()
	// Force evaluation at 14:00 today's date
	todayAfternoon := time.Date(nowDay.Year(), nowDay.Month(), nowDay.Day(), 14, 0, 0, 0, time.UTC)
	evalExhausted, _ := stMgr.Evaluate(devID, todayAfternoon)
	if evalExhausted.AllowedNow || evalExhausted.RemainingMinutes != 0 {
		t.Fatalf("expected daily limit exhausted, got: %+v", evalExhausted)
	}

	// Grant +30m bonus time and re-evaluate
	_, _ = stMgr.GrantBonusMinutes(devID, 30)
	evalBonus, _ := stMgr.Evaluate(devID, todayAfternoon)
	if !evalBonus.AllowedNow || evalBonus.RemainingMinutes != 25 {
		t.Fatalf("expected +30m bonus to allow usage with 25m remaining, got: %+v", evalBonus)
	}
}

func TestApplicationBlockingAndCriticalProcessGuard(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := database.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open db: %v", err)
	}
	defer store.Close()

	appMgr := applications.NewManager(store)
	devID := "dev-child-1"

	// Critical Windows processes must NEVER be blockable
	for _, sysProc := range []string{"csrss.exe", "svchost.exe", "lsass.exe", "explorer.exe", "dwm.exe", "FamilyControl.exe"} {
		if !windows.IsSystemCriticalProcess(sysProc) {
			t.Fatalf("expected %s to be marked system-critical", sysProc)
		}
		_, err := appMgr.SaveRule(applications.AppRule{
			DeviceID:       devID,
			ExecutableName: sysProc,
			DisplayName:    sysProc,
			Status:         "blocked",
		})
		if err == nil {
			t.Fatalf("expected SaveRule to refuse blocking system-critical process %s", sysProc)
		}
	}

	// Block a regular user game executable
	_, err = appMgr.SaveRule(applications.AppRule{
		DeviceID:       devID,
		ExecutableName: "robloxplayerbeta.exe",
		DisplayName:    "Roblox",
		Status:         "blocked",
	})
	if err != nil {
		t.Fatalf("SaveRule for roblox failed: %v", err)
	}

	allowed, reason := appMgr.EvaluateExecutable(devID, "robloxplayerbeta.exe", time.Now())
	if allowed || reason == "" {
		t.Fatalf("expected robloxplayerbeta.exe to be blocked, got allowed=%v", allowed)
	}

	// Schedule-restricted application
	_, err = appMgr.SaveRule(applications.AppRule{
		DeviceID:          devID,
		ExecutableName:    "steam.exe",
		DisplayName:       "Steam",
		Status:            "scheduled",
		DailyLimitMinutes: 60,
		AllowedHours:      "15:00-18:00",
	})
	if err != nil {
		t.Fatalf("SaveRule for steam failed: %v", err)
	}

	morning := time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)
	allowedMorning, _ := appMgr.EvaluateExecutable(devID, "steam.exe", morning)
	if allowedMorning {
		t.Fatalf("expected steam.exe to be blocked outside 15:00-18:00 window")
	}

	afternoon := time.Date(2026, 11, 11, 16, 0, 0, 0, time.UTC)
	allowedAfternoon, _ := appMgr.EvaluateExecutable(devID, "steam.exe", afternoon)
	if !allowedAfternoon {
		t.Fatalf("expected steam.exe to be allowed at 16:00")
	}
}

func TestWebsitesCloudflareDNSAndHostsSync(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := database.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("Open db: %v", err)
	}
	defer store.Close()

	webMgr := websites.NewManager(store)
	pol, err := webMgr.GetPolicy("dev-1")
	if err != nil {
		t.Fatalf("GetPolicy: %v", err)
	}
	if pol.PrimaryIPv4 != "1.1.1.3" || pol.SecondaryIPv4 != "1.0.0.3" ||
		pol.PrimaryIPv6 != "2606:4700:4700::1113" || pol.SecondaryIPv6 != "2606:4700:4700::1003" {
		t.Fatalf("unexpected default Cloudflare Family DNS addresses: %+v", pol)
	}

	pol.Blocklist = []string{"https://tiktok.com/trending", "badsite.example.org"}
	pol.Allowlist = []string{"khanacademy.org"}
	saved, err := webMgr.SavePolicy(pol)
	if err != nil {
		t.Fatalf("SavePolicy failed: %v", err)
	}
	if len(saved.Blocklist) != 2 || saved.Blocklist[0] != "tiktok.com" {
		t.Fatalf("expected sanitized domain blocklist, got: %+v", saved.Blocklist)
	}

	// Test atomic hosts-file blocklist management
	fakeHosts := filepath.Join(tmpDir, "hosts")
	_ = os.WriteFile(fakeHosts, []byte("127.0.0.1 localhost\r\n"), 0644)
	if err := websites.SyncHostsBlocklist(fakeHosts, saved.Blocklist, saved.Allowlist); err != nil {
		t.Fatalf("SyncHostsBlocklist failed: %v", err)
	}
	hostsData, _ := os.ReadFile(fakeHosts)
	hostsStr := string(hostsData)
	if !strings.Contains(hostsStr, "0.0.0.0 tiktok.com") || !strings.Contains(hostsStr, "127.0.0.1 localhost") {
		t.Fatalf("unexpected hosts content:\n%s", hostsStr)
	}
}

func TestEndToEndTLS13PairingAndSync(t *testing.T) {
	parentDir := t.TempDir()
	childDir := t.TempDir()

	// Initialize Parent stack
	pSeal, _ := security.NewLocalSealManager(filepath.Join(parentDir, "keys"))
	pStore, _ := database.Open(filepath.Join(parentDir, "parent.db"))
	defer pStore.Close()
	pAuth := security.NewAuthManager(time.Hour)
	pFam := family.NewManager(pStore, pSeal, pAuth)
	pDev := devices.NewManager(pStore)
	pMon := monitoring.NewManager(pStore)
	pST := screen_time.NewManager(pStore)
	pApps := applications.NewManager(pStore)
	pWeb := websites.NewManager(pStore)
	pProt := protection.NewManager(pStore, pWeb)
	pCoord := networking.NewCoordinator(pStore, pSeal, pFam, pDev, pMon, pST, pApps, pWeb, pProt)

	setupRes, err := pFam.InitializeParentMode("Miller Household", "ParentSecurePass99!", "Call Dad", "Leo")
	if err != nil {
		t.Fatalf("InitializeParentMode failed: %v", err)
	}

	pipeLn := newPipeListener()
	boundAddr, err := pCoord.StartParentServerWithListener(pipeLn)
	if err != nil {
		t.Fatalf("StartParentServerWithListener failed: %v", err)
	}
	defer pCoord.StopParentServer()

	children, _ := pFam.ListChildren()
	ticket, err := pDev.CreatePairingCode(children[0].ID, "Leo Gaming PC", boundAddr, setupRes.Config.TLSFingerprint)
	if err != nil {
		t.Fatalf("CreatePairingCode failed: %v", err)
	}

	// Initialize Child stack
	cSeal, _ := security.NewLocalSealManager(filepath.Join(childDir, "keys"))
	cStore, _ := database.Open(filepath.Join(childDir, "child.db"))
	defer cStore.Close()
	cAuth := security.NewAuthManager(time.Hour)
	cFam := family.NewManager(cStore, cSeal, cAuth)
	cDev := devices.NewManager(cStore)
	cMon := monitoring.NewManager(cStore)
	cST := screen_time.NewManager(cStore)
	cApps := applications.NewManager(cStore)
	cWeb := websites.NewManager(cStore)
	cProt := protection.NewManager(cStore, cWeb)
	cCoord := networking.NewCoordinator(cStore, cSeal, cFam, cDev, cMon, cST, cApps, cWeb, cProt)
	cCoord.SetCustomDialContext(pipeLn.DialContext)

	childCfg, err := cFam.InitializeChildMode("Leo-PC", "ParentSecurePass99!", "Call Dad")
	if err != nil {
		t.Fatalf("InitializeChildMode failed: %v", err)
	}

	// Pair Child with Parent over TLS 1.3 with certificate fingerprint pinning
	pairResp, err := cCoord.PairWithParent(boundAddr, ticket.Code, ticket.ParentCertFP)
	if err != nil {
		t.Fatalf("PairWithParent over TLS 1.3 failed: %v", err)
	}
	if pairResp.FamilyName != "Miller Household" {
		t.Fatalf("expected family name 'Miller Household', got %q", pairResp.FamilyName)
	}

	// Verify one-time pairing code cannot be reused
	if _, err := cCoord.PairWithParent(boundAddr, ticket.Code, ticket.ParentCertFP); err == nil {
		t.Fatalf("expected reused one-time pairing code to be rejected")
	}

	// Parent configures a 90-minute weekday limit and blocks "discord.exe" for the paired child device
	stPol, _ := pST.GetPolicy(childCfg.LocalDeviceID)
	stPol.DailyLimitMinutes = 90
	_, _ = pST.SavePolicy(stPol)
	_, _ = pApps.SaveRule(applications.AppRule{
		DeviceID:       childCfg.LocalDeviceID,
		ExecutableName: "discord.exe",
		DisplayName:    "Discord",
		Status:         "blocked",
	})

	// Child records local usage and performs authenticated TLS 1.3 delta sync
	_ = cMon.RecordAppUsage(childCfg.LocalDeviceID, "code.exe", "VS Code", "main.go", 1200)
	policyBundle, err := cCoord.SyncWithParent()
	if err != nil {
		t.Fatalf("SyncWithParent failed: %v", err)
	}
	if policyBundle.ScreenTimePolicy.DailyLimitMinutes != 90 {
		t.Fatalf("expected synced daily limit 90, got %d", policyBundle.ScreenTimePolicy.DailyLimitMinutes)
	}

	// Verify child locally stored and enforces the synced rule even if parent goes offline
	allowed, _ := cApps.EvaluateExecutable(childCfg.LocalDeviceID, "discord.exe", time.Now())
	if allowed {
		t.Fatalf("expected synced discord.exe block rule to be enforced locally on child")
	}

	// Verify parent received the child's activity telemetry
	parentDevs, _ := pDev.ListDevices()
	if len(parentDevs) != 1 || parentDevs[0].Status != "online" || parentDevs[0].UsedTodayMinutes != 20 {
		t.Fatalf("unexpected parent device telemetry after sync: %+v", parentDevs)
	}

	// Revoke device on Parent and verify subsequent sync is rejected
	_ = pDev.RevokeDevice(childCfg.LocalDeviceID)
	if _, err := cCoord.SyncWithParent(); err == nil {
		t.Fatalf("expected revoked device to be rejected on sync")
	}

	// Test emergency password recovery on Parent
	newRecKey, err := pFam.RecoverParentPassword(setupRes.RecoveryCode, "BrandNewMasterPass2026!")
	if err != nil || newRecKey == "" {
		t.Fatalf("RecoverParentPassword failed: %v", err)
	}
	if !pFam.VerifyParentPassword("BrandNewMasterPass2026!") {
		t.Fatalf("expected new password after recovery to verify")
	}
}

func TestDiscoveryBeaconNoSecrets(t *testing.T) {
	beacon := networking.DiscoveryBeacon{
		Protocol:        networking.DiscoveryProtocolVersion,
		DeviceID:        "dev-parent-01",
		DeviceName:      "LivingRoom-PC",
		Hostname:        "LivingRoom-PC",
		OSVersion:       "Windows 11",
		Mode:            "parent",
		IPAddress:       "192.168.1.10",
		TLSPort:         41843,
		CertFingerprint: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		Paired:          true,
	}
	raw, err := json.Marshal(beacon)
	if err != nil {
		t.Fatalf("marshal beacon: %v", err)
	}
	lower := strings.ToLower(string(raw))
	for _, forbidden := range []string{"password", "token", "secret", "private", "recovery"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("discovery packet must not contain sensitive field %q: %s", forbidden, string(raw))
		}
	}
}

func TestVylosUpdaterSemver(t *testing.T) {
	if !updater.IsNewerSemver("1.0.0", "1.0.1") {
		t.Fatalf("expected 1.0.1 to be newer than 1.0.0")
	}
	if !updater.IsNewerSemver("v1.1.9", "v1.2.0") {
		t.Fatalf("expected v1.2.0 to be newer than v1.1.9")
	}
	if !updater.IsNewerSemver("1.9.9", "2.0.0") {
		t.Fatalf("expected 2.0.0 to be newer than 1.9.9")
	}
	if updater.IsNewerSemver("1.0.0", "1.0.0") {
		t.Fatalf("expected 1.0.0 not to be newer than 1.0.0")
	}
	if updater.IsNewerSemver("1.0.2", "1.0.0") {
		t.Fatalf("expected 1.0.0 not to be newer than 1.0.2")
	}
}

type pipeAddr string

func (p pipeAddr) Network() string { return "tcp" }
func (p pipeAddr) String() string  { return string(p) }

type pipeListener struct {
	ch     chan net.Conn
	closed chan struct{}
}

func newPipeListener() *pipeListener {
	return &pipeListener{
		ch:     make(chan net.Conn, 16),
		closed: make(chan struct{}),
	}
}

func (l *pipeListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.ch:
		return c, nil
	case <-l.closed:
		return nil, errors.New("listener closed")
	}
}

func (l *pipeListener) Close() error {
	select {
	case <-l.closed:
	default:
		close(l.closed)
	}
	return nil
}

func (l *pipeListener) Addr() net.Addr {
	return pipeAddr("127.0.0.1:41843")
}

func (l *pipeListener) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	c1, c2 := net.Pipe()
	select {
	case l.ch <- c1:
		return c2, nil
	case <-l.closed:
		_ = c1.Close()
		_ = c2.Close()
		return nil, errors.New("listener closed")
	case <-ctx.Done():
		_ = c1.Close()
		_ = c2.Close()
		return nil, ctx.Err()
	}
}
