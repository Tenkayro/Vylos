package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
	"familycontrol/backend/websites"
	"familycontrol/backend/windows"
	"familycontrol/service"
)

type OverviewSummary struct {
	ConnectedChildrenCount int                           `json:"connectedChildrenCount"`
	OnlineDevicesCount     int                           `json:"onlineDevicesCount"`
	TotalDevicesCount      int                           `json:"totalDevicesCount"`
	TodayTotalMinutes      int                           `json:"todayTotalMinutes"`
	BlockedAttemptsToday   int                           `json:"blockedAttemptsToday"`
	BlockedDomainsCount    int                           `json:"blockedDomainsCount"`
	NewlyInstalledCount    int                           `json:"newlyInstalledCount"`
	SecurityHealthy        bool                          `json:"securityHealthy"`
	RecentApps             []monitoring.AppUsageEntry    `json:"recentApps"`
	RecentBlockedDomains   []websites.BlockedDomainEvent `json:"recentBlockedDomains"`
	Notifications          []protection.SecurityEvent    `json:"notifications"`
}

type ChildModeStatus struct {
	DeviceID         string                       `json:"deviceId"`
	DeviceName       string                       `json:"deviceName"`
	FamilyName       string                       `json:"familyName"`
	ParentContact    string                       `json:"parentContact"`
	ParentEndpoint   string                       `json:"parentEndpoint"`
	ConnectedToLAN   bool                         `json:"connectedToLan"`
	ProtectionActive bool                         `json:"protectionActive"`
	ScreenTimeEval   screen_time.EvaluationStatus `json:"screenTimeEval"`
	ScreenTimePolicy screen_time.Policy           `json:"screenTimePolicy"`
	AppRules         []applications.AppRule       `json:"appRules"`
	WebsitePolicy    websites.WebsitePolicy       `json:"websitePolicy"`
	LastSyncAt       string                       `json:"lastSyncAt"`
}

type BootstrapState struct {
	Config          family.Config                `json:"config"`
	Children        []family.ChildProfile        `json:"children"`
	Devices         []devices.Device             `json:"devices"`
	Overview        OverviewSummary              `json:"overview"`
	ChildStatus     ChildModeStatus              `json:"childStatus"`
	DiscoveredPeers []networking.DiscoveryBeacon `json:"discoveredPeers"`
	LocalLANIPs     []string                     `json:"localLanIps"`
}

type ActivityBundle struct {
	DailySummary monitoring.DailySummary    `json:"dailySummary"`
	History7Days []monitoring.DailySummary  `json:"history7Days"`
	RecentEvents []monitoring.ActivityEvent `json:"recentEvents"`
}

type ScreenTimeBundle struct {
	Policy     screen_time.Policy           `json:"policy"`
	Evaluation screen_time.EvaluationStatus `json:"evaluation"`
}

type ApplicationsBundle struct {
	InstalledApps []applications.InstalledApp `json:"installedApps"`
	Rules         []applications.AppRule      `json:"rules"`
	RecentUsage   []monitoring.AppUsageEntry  `json:"recentUsage"`
}

type WebsitesBundle struct {
	Policy        websites.WebsitePolicy         `json:"policy"`
	Verification  websites.DNSVerificationStatus `json:"verification"`
	BlockedEvents []websites.BlockedDomainEvent  `json:"blockedEvents"`
}

// App is the primary application controller bound to the Wails 3 / Svelte desktop frontend.
type App struct {
	dataDir     string
	store       *database.Store
	seal        *security.LocalSealManager
	auth        *security.AuthManager
	family      *family.Manager
	devices     *devices.Manager
	monitoring  *monitoring.Manager
	screenTime  *screen_time.Manager
	apps        *applications.Manager
	websites    *websites.Manager
	protection  *protection.Manager
	coordinator *networking.Coordinator
	discovery   *networking.DiscoveryService
	tray        *windows.TrayManager
	agent       *service.BackgroundAgent
	lastSyncAt  string
	onOpenUI    func()
}

// NewApp initializes the FamilyControl application stack in the given data directory.
func NewApp(dataDir string, enforceOSLock bool) (*App, error) {
	if dataDir == "" {
		appData, err := os.UserConfigDir()
		if err != nil || appData == "" {
			appData = "."
		}
		dataDir = filepath.Join(appData, "FamilyControl")
	}
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	seal, err := security.NewLocalSealManager(filepath.Join(dataDir, "keys"))
	if err != nil {
		return nil, fmt.Errorf("init seal manager: %w", err)
	}

	store, err := database.Open(filepath.Join(dataDir, "familycontrol.db"))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	auth := security.NewAuthManager(8 * time.Hour)
	famMgr := family.NewManager(store, seal, auth)
	_ = famMgr.EnsureCryptographicIdentity("")

	devMgr := devices.NewManager(store)
	monMgr := monitoring.NewManager(store)
	stMgr := screen_time.NewManager(store)
	appMgr := applications.NewManager(store)
	webMgr := websites.NewManager(store)
	protMgr := protection.NewManager(store, webMgr)

	coord := networking.NewCoordinator(store, seal, famMgr, devMgr, monMgr, stMgr, appMgr, webMgr, protMgr)

	app := &App{
		dataDir:     dataDir,
		store:       store,
		seal:        seal,
		auth:        auth,
		family:      famMgr,
		devices:     devMgr,
		monitoring:  monMgr,
		screenTime:  stMgr,
		apps:        appMgr,
		websites:    webMgr,
		protection:  protMgr,
		coordinator: coord,
	}

	cfg, _ := famMgr.GetConfig()
	disc := networking.NewDiscoveryService(cfg.DiscoveryPort, func() networking.DiscoveryBeacon {
		c, _ := famMgr.GetConfig()
		ips := protection.GetLocalLANIPs()
		return networking.DiscoveryBeacon{
			Protocol:        networking.DiscoveryProtocolVersion,
			DeviceID:        c.LocalDeviceID,
			DeviceName:      c.LocalDeviceName,
			Mode:            string(c.Mode),
			IPAddress:       ips[0],
			TLSPort:         c.ListenPort,
			CertFingerprint: c.TLSFingerprint,
			Paired:          c.ParentEndpoint != "" || c.Mode == family.ModeParent,
		}
	})
	app.discovery = disc

	tray := windows.NewTrayManager(
		"FamilyControl — Local Parental Control",
		func() {
			if app.onOpenUI != nil {
				app.onOpenUI()
			}
		},
		func() {
			_, _ = app.TriggerChildSync()
		},
		func() {},
	)
	app.tray = tray

	app.agent = service.NewBackgroundAgent(
		store, famMgr, devMgr, monMgr, stMgr, appMgr, webMgr, protMgr,
		coord, disc, tray, enforceOSLock,
	)

	return app, nil
}

func (a *App) StartBackgroundRuntime(withTray bool) {
	if withTray && a.tray != nil {
		a.tray.Start()
	}
	if a.agent != nil {
		a.agent.Start()
	}
}

func (a *App) Shutdown() {
	if a.agent != nil {
		a.agent.Stop()
	}
	if a.tray != nil {
		a.tray.Stop()
	}
	if a.store != nil {
		_ = a.store.Close()
	}
}

func (a *App) Agent() *service.BackgroundAgent {
	return a.agent
}

func (a *App) Coordinator() *networking.Coordinator {
	return a.coordinator
}

func (a *App) SetOnOpenUI(fn func()) {
	a.onOpenUI = fn
}

// GetBootstrapState returns the complete initial state required by the Svelte UI.
func (a *App) GetBootstrapState() (BootstrapState, error) {
	cfg, err := a.family.GetConfig()
	if err != nil {
		return BootstrapState{}, err
	}
	children, _ := a.family.ListChildren()
	devs, _ := a.devices.ListDevices()

	// Ensure local machine appears in device list or has policies initialized
	targetDevID := cfg.LocalDeviceID
	if cfg.Mode == family.ModeParent && len(devs) > 0 {
		targetDevID = devs[0].ID
	}

	overview := a.buildOverviewSummary(cfg, children, devs, targetDevID)
	childStatus := a.buildChildModeStatus(cfg)

	var peers []networking.DiscoveryBeacon
	if a.discovery != nil {
		peers = a.discovery.ListDiscoveredPeers("")
	} else {
		peers = []networking.DiscoveryBeacon{}
	}

	return BootstrapState{
		Config:          cfg,
		Children:        children,
		Devices:         devs,
		Overview:        overview,
		ChildStatus:     childStatus,
		DiscoveredPeers: peers,
		LocalLANIPs:     protection.GetLocalLANIPs(),
	}, nil
}

func (a *App) buildOverviewSummary(cfg family.Config, children []family.ChildProfile, devs []devices.Device, primaryDeviceID string) OverviewSummary {
	onlineCount := 0
	for _, d := range devs {
		if d.Status == "online" {
			onlineCount++
		}
	}
	todayStr := time.Now().UTC().Format("2006-01-02")
	daily, _ := a.monitoring.GetDailySummary("all", todayStr)
	if daily.TotalMinutes == 0 && cfg.LocalDeviceID != "" {
		localDaily, _ := a.monitoring.GetDailySummary(cfg.LocalDeviceID, todayStr)
		daily = localDaily
	}

	blockedDomains, _ := a.websites.ListBlockedDomainEvents("all", 8)
	secEvents, _ := a.protection.ListSecurityEvents("all", 8)

	var newAppsCount int
	_ = a.store.DB().QueryRow(`SELECT COUNT(1) FROM installed_apps WHERE newly_installed = 1`).Scan(&newAppsCount)

	return OverviewSummary{
		ConnectedChildrenCount: len(children),
		OnlineDevicesCount:     onlineCount,
		TotalDevicesCount:      len(devs),
		TodayTotalMinutes:      daily.TotalMinutes,
		BlockedAttemptsToday:   daily.BlockedAttempts,
		BlockedDomainsCount:    len(blockedDomains),
		NewlyInstalledCount:    newAppsCount,
		SecurityHealthy:        cfg.TLSFingerprint != "",
		RecentApps:             daily.TopApps,
		RecentBlockedDomains:   blockedDomains,
		Notifications:          secEvents,
	}
}

func (a *App) buildChildModeStatus(cfg family.Config) ChildModeStatus {
	devID := cfg.LocalDeviceID
	if devID == "" {
		devID = "local"
	}
	eval, _ := a.screenTime.Evaluate(devID, time.Now())
	pol, _ := a.screenTime.GetPolicy(devID)
	rules, _ := a.apps.ListRules(devID)
	webPol, _ := a.websites.GetPolicy(devID)

	return ChildModeStatus{
		DeviceID:         devID,
		DeviceName:       cfg.LocalDeviceName,
		FamilyName:       cfg.FamilyName,
		ParentContact:    cfg.ParentContact,
		ParentEndpoint:   cfg.ParentEndpoint,
		ConnectedToLAN:   cfg.ParentEndpoint != "",
		ProtectionActive: cfg.SetupCompleted,
		ScreenTimeEval:   eval,
		ScreenTimePolicy: pol,
		AppRules:         rules,
		WebsitePolicy:    webPol,
		LastSyncAt:       a.lastSyncAt,
	}
}

func (a *App) SetupParentMode(familyName, password, parentContact, firstChildName string) (family.SetupParentResult, error) {
	res, err := a.family.InitializeParentMode(familyName, password, parentContact, firstChildName)
	if err != nil {
		return res, err
	}
	_, _ = a.coordinator.StartParentServer(fmt.Sprintf(":%d", res.Config.ListenPort))
	_ = a.protection.LogSecurityEvent(res.Config.LocalDeviceID, "info", "setup", "Configured Parent Mode endpoint", res.Config.FamilyName)
	return res, nil
}

func (a *App) SetupChildMode(deviceName, parentPassword, parentContact string) (family.Config, error) {
	cfg, err := a.family.InitializeChildMode(deviceName, parentPassword, parentContact)
	if err != nil {
		return cfg, err
	}
	_ = a.protection.LogSecurityEvent(cfg.LocalDeviceID, "info", "setup", "Configured Child Mode agent", cfg.LocalDeviceName)
	return cfg, nil
}

func (a *App) LoginParent(password string) (security.Session, error) {
	sess, err := a.family.AuthenticateParent(password)
	if err != nil {
		_ = a.protection.LogSecurityEvent("local", "warning", "auth", "Failed parent login attempt", err.Error())
		return sess, err
	}
	_ = a.protection.LogSecurityEvent("local", "info", "auth", "Parent session authenticated", "")
	return sess, nil
}

func (a *App) LogoutParent(sessionToken string) {
	a.auth.RevokeSession(sessionToken)
}

func (a *App) RecoverParentPassword(recoveryCode, newPassword string) (string, error) {
	newCode, err := a.family.RecoverParentPassword(recoveryCode, newPassword)
	if err != nil {
		_ = a.protection.LogSecurityEvent("local", "warning", "recovery", "Failed emergency password recovery attempt", err.Error())
		return "", err
	}
	_ = a.protection.LogSecurityEvent("local", "critical", "recovery", "Parent password reset via emergency recovery key", "")
	return newCode, nil
}

func (a *App) ChangeParentPassword(currentPassword, newPassword string) error {
	if err := a.family.ChangeParentPassword(currentPassword, newPassword); err != nil {
		return err
	}
	_ = a.protection.LogSecurityEvent("local", "info", "security", "Parent password updated", "")
	return nil
}

func (a *App) SwitchOperatingMode(password, targetMode string) (family.Config, error) {
	if !a.family.VerifyParentPassword(password) {
		return family.Config{}, errors.New("parent password required to switch operating mode")
	}
	cfg, err := a.family.SwitchOperatingMode(family.OperatingMode(targetMode))
	if err != nil {
		return cfg, err
	}
	if cfg.Mode == family.ModeParent {
		_, _ = a.coordinator.StartParentServer(fmt.Sprintf(":%d", cfg.ListenPort))
	} else {
		_ = a.coordinator.StopParentServer()
	}
	_ = a.protection.LogSecurityEvent(cfg.LocalDeviceID, "info", "mode", fmt.Sprintf("Switched operating mode to %s", targetMode), "")
	return cfg, nil
}

func (a *App) AddChild(name, avatarColor, ageGroup string) (family.ChildProfile, error) {
	return a.family.AddChild(name, avatarColor, ageGroup)
}

func (a *App) RemoveChild(childID string) error {
	return a.family.RemoveChild(childID)
}

func (a *App) CreatePairingCode(childID, deviceNameHint string) (devices.PairingTicket, error) {
	cfg, err := a.family.GetConfig()
	if err != nil {
		return devices.PairingTicket{}, err
	}
	ips := protection.GetLocalLANIPs()
	endpoint := fmt.Sprintf("%s:%d", ips[0], cfg.ListenPort)
	return a.devices.CreatePairingCode(childID, deviceNameHint, endpoint, cfg.TLSFingerprint)
}

func (a *App) PairWithParent(parentEndpoint, pairingCode, expectedCertFP string) (devices.PairingResponse, error) {
	resp, err := a.coordinator.PairWithParent(parentEndpoint, pairingCode, expectedCertFP)
	if err != nil {
		return resp, err
	}
	_, _ = a.TriggerChildSync()
	return resp, nil
}

func (a *App) TriggerChildSync() (networking.PolicyBundle, error) {
	pb, err := a.coordinator.SyncWithParent()
	if err == nil {
		a.lastSyncAt = time.Now().UTC().Format(time.RFC3339)
	}
	return pb, err
}

func (a *App) TriggerDiscoveryBroadcast() ([]networking.DiscoveryBeacon, error) {
	if a.discovery != nil {
		_ = a.discovery.BroadcastOnce()
		return a.discovery.ListDiscoveredPeers(""), nil
	}
	return []networking.DiscoveryBeacon{}, nil
}

func (a *App) RenameDevice(deviceID, newName, childID string) error {
	return a.devices.RenameDevice(deviceID, newName, childID)
}

func (a *App) RevokeDevice(deviceID string) error {
	if err := a.devices.RevokeDevice(deviceID); err != nil {
		return err
	}
	return a.protection.LogSecurityEvent(deviceID, "warning", "revocation", "Revoked paired device credentials", deviceID)
}

func (a *App) DeleteDevice(deviceID string) error {
	return a.devices.DeleteDevice(deviceID)
}

func (a *App) GetActivityBundle(deviceID, dateStr string) (ActivityBundle, error) {
	if deviceID == "" {
		cfg, _ := a.family.GetConfig()
		deviceID = cfg.LocalDeviceID
	}
	daily, err := a.monitoring.GetDailySummary(deviceID, dateStr)
	if err != nil {
		return ActivityBundle{}, err
	}
	hist, _ := a.monitoring.ListHistoricalDailyUsage(deviceID, 7)
	events, _ := a.monitoring.ListRecentEvents(deviceID, 60)
	return ActivityBundle{
		DailySummary: daily,
		History7Days: hist,
		RecentEvents: events,
	}, nil
}

func (a *App) GetScreenTimeBundle(deviceID string) (ScreenTimeBundle, error) {
	if deviceID == "" {
		cfg, _ := a.family.GetConfig()
		deviceID = cfg.LocalDeviceID
	}
	pol, err := a.screenTime.GetPolicy(deviceID)
	if err != nil {
		return ScreenTimeBundle{}, err
	}
	eval, _ := a.screenTime.Evaluate(deviceID, time.Now())
	return ScreenTimeBundle{
		Policy:     pol,
		Evaluation: eval,
	}, nil
}

func (a *App) SaveScreenTimePolicy(policy screen_time.Policy) (screen_time.Policy, error) {
	saved, err := a.screenTime.SavePolicy(policy)
	if err != nil {
		return saved, err
	}
	_ = a.protection.LogSecurityEvent(policy.DeviceID, "info", "policy", "Updated screen-time policy",
		fmt.Sprintf("Daily: %dm, Weekend: %dm, Bedtime: %s-%s", saved.DailyLimitMinutes, saved.WeekendLimitMinutes, saved.BedtimeStart, saved.BedtimeEnd))
	return saved, nil
}

func (a *App) SetDeviceLockAndPause(deviceID string, screenLocked, manualPause bool) (screen_time.Policy, error) {
	pol, err := a.screenTime.SetLockAndPause(deviceID, screenLocked, manualPause)
	if err != nil {
		return pol, err
	}
	_ = a.protection.LogSecurityEvent(deviceID, "info", "screen_control",
		fmt.Sprintf("Screen lock=%v, manual pause=%v", screenLocked, manualPause), "")
	return pol, nil
}

func (a *App) GrantBonusTime(deviceID string, bonusMinutes int) (screen_time.Policy, error) {
	return a.screenTime.GrantBonusMinutes(deviceID, bonusMinutes)
}

func (a *App) GetApplicationsBundle(deviceID string) (ApplicationsBundle, error) {
	if deviceID == "" {
		cfg, _ := a.family.GetConfig()
		deviceID = cfg.LocalDeviceID
	}
	installed, _ := a.apps.ListInstalledApps(deviceID)
	if len(installed) == 0 {
		installed, _ = a.apps.RefreshInstalledApps(deviceID)
	}
	rules, _ := a.apps.ListRules(deviceID)
	usage, _ := a.monitoring.ListAppUsage(deviceID, "")
	return ApplicationsBundle{
		InstalledApps: installed,
		Rules:         rules,
		RecentUsage:   usage,
	}, nil
}

func (a *App) RefreshInstalledApplications(deviceID string) ([]applications.InstalledApp, error) {
	if deviceID == "" {
		cfg, _ := a.family.GetConfig()
		deviceID = cfg.LocalDeviceID
	}
	return a.apps.RefreshInstalledApps(deviceID)
}

func (a *App) SaveApplicationRule(rule applications.AppRule) (applications.AppRule, error) {
	saved, err := a.apps.SaveRule(rule)
	if err != nil {
		return saved, err
	}
	_ = a.protection.LogSecurityEvent(saved.DeviceID, "info", "applications",
		fmt.Sprintf("Set application rule for %s: %s", saved.DisplayName, saved.Status), saved.ExecutableName)
	return saved, nil
}

func (a *App) DeleteApplicationRule(deviceID, executableName string) error {
	return a.apps.DeleteRule(deviceID, executableName)
}

func (a *App) AcknowledgeNewlyInstalledApp(deviceID, executableName string) error {
	return a.apps.AcknowledgeNewApp(deviceID, executableName)
}

func (a *App) GetWebsitesBundle(deviceID string) (WebsitesBundle, error) {
	if deviceID == "" {
		cfg, _ := a.family.GetConfig()
		deviceID = cfg.LocalDeviceID
	}
	pol, err := a.websites.GetPolicy(deviceID)
	if err != nil {
		return WebsitesBundle{}, err
	}
	ver := a.websites.VerifyDNSConfiguration(deviceID)
	events, _ := a.websites.ListBlockedDomainEvents(deviceID, 50)
	return WebsitesBundle{
		Policy:        pol,
		Verification:  ver,
		BlockedEvents: events,
	}, nil
}

func (a *App) SaveWebsitePolicy(policy websites.WebsitePolicy) (websites.WebsitePolicy, error) {
	saved, err := a.websites.SavePolicy(policy)
	if err != nil {
		return saved, err
	}
	_ = a.protection.LogSecurityEvent(saved.DeviceID, "info", "websites",
		fmt.Sprintf("Updated website/DNS policy (%s)", saved.DNSMode),
		fmt.Sprintf("Allowlist: %d, Blocklist: %d", len(saved.Allowlist), len(saved.Blocklist)))
	return saved, nil
}

func (a *App) EnforceDNSAndHosts(deviceID string) (websites.DNSVerificationStatus, error) {
	if deviceID == "" {
		cfg, _ := a.family.GetConfig()
		deviceID = cfg.LocalDeviceID
	}
	err := a.websites.EnforceLocalPolicy(deviceID)
	status := a.websites.VerifyDNSConfiguration(deviceID)
	return status, err
}

func (a *App) GetProtectionStatus(deviceID string) (protection.ProtectionStatus, error) {
	running := a.agent != nil && a.agent.IsRunning()
	return a.protection.RunIntegrityCheck(deviceID, running)
}

func (a *App) UpdateGeneralSettings(familyName, parentContact string, retentionDays, listenPort int) (family.Config, error) {
	return a.family.UpdateSettings(familyName, parentContact, retentionDays, listenPort)
}

func (a *App) PruneHistoricalData(retentionDays int) (int64, error) {
	n, err := a.store.PruneOldData(retentionDays)
	if err == nil {
		_ = a.protection.LogSecurityEvent("local", "info", "retention",
			fmt.Sprintf("Pruned %d expired records older than %d days", n, retentionDays), "")
	}
	return n, err
}

func (a *App) ExportEncryptedBackup() (string, error) {
	raw, err := a.store.ExportBackup()
	if err != nil {
		return "", err
	}
	enc, err := a.seal.Encrypt(raw)
	if err != nil {
		return "", err
	}
	_ = a.protection.LogSecurityEvent("local", "info", "backup", "Exported encrypted family configuration backup", "")
	return enc, nil
}

func (a *App) ImportEncryptedBackup(encryptedPayload string) error {
	encryptedPayload = strings.TrimSpace(encryptedPayload)
	raw, err := a.seal.Decrypt(encryptedPayload)
	if err != nil {
		// Allow importing raw JSON backup if valid JSON
		if strings.HasPrefix(encryptedPayload, "{") {
			raw = []byte(encryptedPayload)
		} else {
			return fmt.Errorf("failed to decrypt backup payload: %w", err)
		}
	}
	if err := a.store.ImportBackup(raw); err != nil {
		return err
	}
	_ = a.protection.LogSecurityEvent("local", "info", "backup", "Restored family configuration from backup", "")
	return nil
}

func (a *App) AuthorizeUninstall(parentPassword string) (bool, error) {
	if !a.family.VerifyParentPassword(parentPassword) {
		_ = a.protection.LogSecurityEvent("local", "warning", "uninstall", "Denied unauthorized uninstall request", "")
		return false, errors.New("invalid parent password; uninstall authorization denied")
	}
	_ = a.protection.LogSecurityEvent("local", "critical", "uninstall", "Parent authorized application uninstallation", "")
	return true, nil
}

// RegisterRPCBridge mounts the local JSON-RPC bridge handlers so the Svelte frontend works identically
// in Wails 3 WebView2 mode and standalone desktop window mode.
func (a *App) RegisterRPCBridge(mux *http.ServeMux) {
	mux.HandleFunc("/rpc/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		method := strings.TrimPrefix(r.URL.Path, "/rpc/")
		var payload map[string]json.RawMessage
		_ = json.NewDecoder(r.Body).Decode(&payload)

		res, err := a.dispatchRPC(method, payload)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": res})
	})
}

func (a *App) dispatchRPC(method string, args map[string]json.RawMessage) (any, error) {
	strArg := func(k string) string {
		var s string
		if raw, ok := args[k]; ok {
			_ = json.Unmarshal(raw, &s)
		}
		return s
	}
	intArg := func(k string) int {
		var v int
		if raw, ok := args[k]; ok {
			_ = json.Unmarshal(raw, &v)
		}
		return v
	}
	boolArg := func(k string) bool {
		var v bool
		if raw, ok := args[k]; ok {
			_ = json.Unmarshal(raw, &v)
		}
		return v
	}

	switch method {
	case "GetBootstrapState":
		return a.GetBootstrapState()
	case "SetupParentMode":
		return a.SetupParentMode(strArg("familyName"), strArg("password"), strArg("parentContact"), strArg("firstChildName"))
	case "SetupChildMode":
		return a.SetupChildMode(strArg("deviceName"), strArg("parentPassword"), strArg("parentContact"))
	case "LoginParent":
		return a.LoginParent(strArg("password"))
	case "LogoutParent":
		a.LogoutParent(strArg("sessionToken"))
		return map[string]bool{"ok": true}, nil
	case "RecoverParentPassword":
		return a.RecoverParentPassword(strArg("recoveryCode"), strArg("newPassword"))
	case "ChangeParentPassword":
		return map[string]bool{"ok": true}, a.ChangeParentPassword(strArg("currentPassword"), strArg("newPassword"))
	case "SwitchOperatingMode":
		return a.SwitchOperatingMode(strArg("password"), strArg("targetMode"))
	case "AddChild":
		return a.AddChild(strArg("name"), strArg("avatarColor"), strArg("ageGroup"))
	case "RemoveChild":
		return map[string]bool{"ok": true}, a.RemoveChild(strArg("childId"))
	case "CreatePairingCode":
		return a.CreatePairingCode(strArg("childId"), strArg("deviceNameHint"))
	case "PairWithParent":
		return a.PairWithParent(strArg("parentEndpoint"), strArg("pairingCode"), strArg("expectedCertFp"))
	case "TriggerChildSync":
		return a.TriggerChildSync()
	case "TriggerDiscoveryBroadcast":
		return a.TriggerDiscoveryBroadcast()
	case "RenameDevice":
		return map[string]bool{"ok": true}, a.RenameDevice(strArg("deviceId"), strArg("newName"), strArg("childId"))
	case "RevokeDevice":
		return map[string]bool{"ok": true}, a.RevokeDevice(strArg("deviceId"))
	case "DeleteDevice":
		return map[string]bool{"ok": true}, a.DeleteDevice(strArg("deviceId"))
	case "GetActivityBundle":
		return a.GetActivityBundle(strArg("deviceId"), strArg("dateStr"))
	case "GetScreenTimeBundle":
		return a.GetScreenTimeBundle(strArg("deviceId"))
	case "SaveScreenTimePolicy":
		var pol screen_time.Policy
		if raw, ok := args["policy"]; ok {
			if err := json.Unmarshal(raw, &pol); err != nil {
				return nil, err
			}
		}
		return a.SaveScreenTimePolicy(pol)
	case "SetDeviceLockAndPause":
		return a.SetDeviceLockAndPause(strArg("deviceId"), boolArg("screenLocked"), boolArg("manualPause"))
	case "GrantBonusTime":
		return a.GrantBonusTime(strArg("deviceId"), intArg("bonusMinutes"))
	case "GetApplicationsBundle":
		return a.GetApplicationsBundle(strArg("deviceId"))
	case "RefreshInstalledApplications":
		return a.RefreshInstalledApplications(strArg("deviceId"))
	case "SaveApplicationRule":
		var rule applications.AppRule
		if raw, ok := args["rule"]; ok {
			if err := json.Unmarshal(raw, &rule); err != nil {
				return nil, err
			}
		}
		return a.SaveApplicationRule(rule)
	case "DeleteApplicationRule":
		return map[string]bool{"ok": true}, a.DeleteApplicationRule(strArg("deviceId"), strArg("executableName"))
	case "AcknowledgeNewlyInstalledApp":
		return map[string]bool{"ok": true}, a.AcknowledgeNewlyInstalledApp(strArg("deviceId"), strArg("executableName"))
	case "GetWebsitesBundle":
		return a.GetWebsitesBundle(strArg("deviceId"))
	case "SaveWebsitePolicy":
		var pol websites.WebsitePolicy
		if raw, ok := args["policy"]; ok {
			if err := json.Unmarshal(raw, &pol); err != nil {
				return nil, err
			}
		}
		return a.SaveWebsitePolicy(pol)
	case "EnforceDNSAndHosts":
		return a.EnforceDNSAndHosts(strArg("deviceId"))
	case "GetProtectionStatus":
		return a.GetProtectionStatus(strArg("deviceId"))
	case "UpdateGeneralSettings":
		return a.UpdateGeneralSettings(strArg("familyName"), strArg("parentContact"), intArg("retentionDays"), intArg("listenPort"))
	case "PruneHistoricalData":
		return a.PruneHistoricalData(intArg("retentionDays"))
	case "ExportEncryptedBackup":
		return a.ExportEncryptedBackup()
	case "ImportEncryptedBackup":
		return map[string]bool{"ok": true}, a.ImportEncryptedBackup(strArg("encryptedPayload"))
	case "AuthorizeUninstall":
		return a.AuthorizeUninstall(strArg("parentPassword"))
	default:
		return nil, fmt.Errorf("unknown RPC method: %s", method)
	}
}
