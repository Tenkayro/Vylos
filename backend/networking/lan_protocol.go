package networking

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"familycontrol/backend/applications"
	"familycontrol/backend/database"
	"familycontrol/backend/devices"
	"familycontrol/backend/family"
	"familycontrol/backend/monitoring"
	"familycontrol/backend/protection"
	"familycontrol/backend/screen_time"
	"familycontrol/backend/security"
	"familycontrol/backend/websites"
	"familycontrol/backend/windows"

	"golang.org/x/time/rate"
)

// SyncBundle represents the bidirectional payload exchanged between a Child agent and the Parent LAN endpoint.
type SyncBundle struct {
	DeviceID         string                        `json:"deviceId"`
	ActiveApp        string                        `json:"activeApp"`
	UsedTodayMinutes int                           `json:"usedTodayMinutes"`
	RemainingMinutes int                           `json:"remainingMinutes"`
	ScreenLocked     bool                          `json:"screenLocked"`
	DNSProtected     bool                          `json:"dnsProtected"`
	ActivityEvents   []monitoring.ActivityEvent    `json:"activityEvents"`
	AppUsageEntries  []monitoring.AppUsageEntry    `json:"appUsageEntries"`
	InstalledApps    []applications.InstalledApp   `json:"installedApps"`
	BlockedDomains   []websites.BlockedDomainEvent `json:"blockedDomains"`
	SecurityEvents   []protection.SecurityEvent    `json:"securityEvents"`
	TimestampUnix    int64                         `json:"timestampUnix"`
}

// PolicyBundle contains the authoritative rules returned by the Parent PC to a Child PC.
type PolicyBundle struct {
	DeviceID         string                 `json:"deviceId"`
	FamilyName       string                 `json:"familyName"`
	ParentContact    string                 `json:"parentContact"`
	ScreenTimePolicy screen_time.Policy     `json:"screenTimePolicy"`
	AppRules         []applications.AppRule `json:"appRules"`
	WebsitePolicy    websites.WebsitePolicy `json:"websitePolicy"`
	ServerTimeUnix   int64                  `json:"serverTimeUnix"`
}

// Coordinator manages the TLS 1.3 Parent LAN Server, the Child LAN Sync Client, and the offline event queue.
type Coordinator struct {
	mu          sync.Mutex
	store       *database.Store
	seal        *security.LocalSealManager
	family      *family.Manager
	devices     *devices.Manager
	monitoring  *monitoring.Manager
	screenTime  *screen_time.Manager
	apps        *applications.Manager
	websites    *websites.Manager
	protection  *protection.Manager
	replayGuard *security.ReplayGuard
	ipLimiters  map[string]*rate.Limiter
	server      *http.Server
	listener    net.Listener
	boundAddr   string
	dialContext func(ctx context.Context, network, addr string) (net.Conn, error)
}

func NewCoordinator(
	store *database.Store,
	seal *security.LocalSealManager,
	fam *family.Manager,
	dev *devices.Manager,
	mon *monitoring.Manager,
	st *screen_time.Manager,
	apps *applications.Manager,
	web *websites.Manager,
	prot *protection.Manager,
) *Coordinator {
	return &Coordinator{
		store:       store,
		seal:        seal,
		family:      fam,
		devices:     dev,
		monitoring:  mon,
		screenTime:  st,
		apps:        apps,
		websites:    web,
		protection:  prot,
		replayGuard: security.NewReplayGuard(2 * time.Minute),
		ipLimiters:  make(map[string]*rate.Limiter),
	}
}

// StartParentServer starts the TLS 1.3 HTTPS coordination server on listenAddr (e.g., ":41843" or "127.0.0.1:0").
func (c *Coordinator) StartParentServer(listenAddr string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.server != nil {
		return c.boundAddr, nil
	}

	certPEM, keyPEM, _, err := c.family.GetLocalTLSCredentials()
	if err != nil {
		return "", fmt.Errorf("load TLS credentials: %w", err)
	}
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return "", fmt.Errorf("parse X509 keypair: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ping", c.withRateLimit(c.handlePing))
	mux.HandleFunc("/api/v1/pair", c.withRateLimit(c.handlePair))
	mux.HandleFunc("/api/v1/sync", c.withRateLimit(c.withDeviceAuth(c.handleSync)))

	ln, err := tls.Listen("tcp", listenAddr, tlsCfg)
	if err != nil {
		return "", fmt.Errorf("listen TLS on %s: %w", listenAddr, err)
	}

	c.listener = ln
	c.boundAddr = ln.Addr().String()
	c.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		_ = c.server.Serve(ln)
	}()

	return c.boundAddr, nil
}

// StopParentServer gracefully stops the TLS 1.3 LAN server.
func (c *Coordinator) StopParentServer() error {
	c.mu.Lock()
	srv := c.server
	c.server = nil
	c.boundAddr = ""
	c.mu.Unlock()

	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
	return nil
}

func (c *Coordinator) BoundAddr() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.boundAddr
}

// SetCustomDialContext sets an optional custom dialer (e.g. for in-memory pipe testing in network-restricted sandboxes).
func (c *Coordinator) SetCustomDialContext(fn func(ctx context.Context, network, addr string) (net.Conn, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dialContext = fn
}

// StartParentServerWithListener starts the TLS 1.3 HTTPS coordination server over a provided net.Listener.
func (c *Coordinator) StartParentServerWithListener(rawLn net.Listener) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.server != nil {
		return c.boundAddr, nil
	}

	certPEM, keyPEM, _, err := c.family.GetLocalTLSCredentials()
	if err != nil {
		return "", fmt.Errorf("load TLS credentials: %w", err)
	}
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return "", fmt.Errorf("parse X509 keypair: %w", err)
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS13,
		MaxVersion:   tls.VersionTLS13,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ping", c.withRateLimit(c.handlePing))
	mux.HandleFunc("/api/v1/pair", c.withRateLimit(c.handlePair))
	mux.HandleFunc("/api/v1/sync", c.withRateLimit(c.withDeviceAuth(c.handleSync)))

	ln := tls.NewListener(rawLn, tlsCfg)
	c.listener = ln
	c.boundAddr = ln.Addr().String()
	c.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		_ = c.server.Serve(ln)
	}()

	return c.boundAddr, nil
}

func (c *Coordinator) allowIP(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	lim, ok := c.ipLimiters[host]
	if !ok {
		lim = rate.NewLimiter(rate.Every(200*time.Millisecond), 25)
		c.ipLimiters[host] = lim
	}
	return lim.Allow()
}

func (c *Coordinator) withRateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !c.allowIP(r.RemoteAddr) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// withDeviceAuth authenticates child requests using DeviceID, Bearer Token, Replay Guard (Timestamp + Nonce),
// and Ed25519 cryptographic request signature. Never relies on IP address alone.
func (c *Coordinator) withDeviceAuth(next func(http.ResponseWriter, *http.Request, string, []byte)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		deviceID := strings.TrimSpace(r.Header.Get("X-FC-Device-Id"))
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		tsStr := strings.TrimSpace(r.Header.Get("X-FC-Timestamp"))
		nonce := strings.TrimSpace(r.Header.Get("X-FC-Nonce"))
		sigB64 := strings.TrimSpace(r.Header.Get("X-FC-Signature"))

		rawToken := strings.TrimPrefix(authHeader, "Bearer ")
		pubKeyB64, _, err := c.devices.VerifyDeviceCredentials(deviceID, rawToken)
		if err != nil {
			_ = c.protection.LogSecurityEvent(deviceID, "warning", "auth", "Rejected unauthorized LAN request", err.Error())
			http.Error(w, "unauthorized device credentials", http.StatusUnauthorized)
			return
		}

		tsUnix, err := strconv.ParseInt(tsStr, 10, 64)
		if err != nil {
			http.Error(w, "invalid timestamp header", http.StatusBadRequest)
			return
		}
		if err := c.replayGuard.ValidateAndRecord(deviceID, nonce, tsUnix); err != nil {
			_ = c.protection.LogSecurityEvent(deviceID, "warning", "replay", "Rejected replayed or stale LAN request", err.Error())
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 2*1024*1024))
		if err != nil {
			http.Error(w, "read body error", http.StatusBadRequest)
			return
		}

		bodySum := sha256.Sum256(body)
		canonical := fmt.Sprintf("%s:%s:%s:%d:%s:%s",
			r.Method, r.URL.Path, deviceID, tsUnix, nonce, hex.EncodeToString(bodySum[:]))
		if !security.VerifyMessageSignature(pubKeyB64, []byte(canonical), sigB64) {
			_ = c.protection.LogSecurityEvent(deviceID, "critical", "signature", "Invalid Ed25519 signature on LAN request", deviceID)
			http.Error(w, "invalid cryptographic signature", http.StatusUnauthorized)
			return
		}

		next(w, r, deviceID, body)
	}
}

func (c *Coordinator) handlePing(w http.ResponseWriter, r *http.Request) {
	cfg, err := c.family.GetConfig()
	if err != nil {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          "ok",
		"mode":            cfg.Mode,
		"familyName":      cfg.FamilyName,
		"deviceId":        cfg.LocalDeviceID,
		"certFingerprint": cfg.TLSFingerprint,
		"timestamp":       time.Now().UTC().Unix(),
	})
}

func (c *Coordinator) handlePair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	var req devices.PairingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if err := c.replayGuard.ValidateAndRecord(req.DeviceID, "pair-"+req.PairingCode, req.TimestampUnix); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && req.IPAddress == "" {
		req.IPAddress = host
	}

	resp, err := c.devices.CompletePairing(req)
	if err != nil {
		_ = c.protection.LogSecurityEvent(req.DeviceID, "warning", "pairing", "Failed device pairing attempt", err.Error())
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	_ = c.protection.LogSecurityEvent(req.DeviceID, "info", "pairing",
		fmt.Sprintf("Paired child device '%s' (%s)", req.DeviceName, req.IPAddress),
		fmt.Sprintf("Cert SHA256: %s", req.CertFingerprint))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (c *Coordinator) handleSync(w http.ResponseWriter, r *http.Request, deviceID string, body []byte) {
	var bundle SyncBundle
	if err := json.Unmarshal(body, &bundle); err != nil {
		http.Error(w, "invalid sync payload", http.StatusBadRequest)
		return
	}

	remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	_ = c.devices.UpdateHeartbeat(
		deviceID, remoteIP, bundle.ActiveApp,
		bundle.UsedTodayMinutes, bundle.RemainingMinutes,
		bundle.ScreenLocked, bundle.DNSProtected,
	)

	// Ingest synced telemetry from the child computer
	c.ingestChildSyncBundle(deviceID, bundle)

	// Return authoritative policy bundle for this child device
	cfg, _ := c.family.GetConfig()
	stPol, _ := c.screenTime.GetPolicy(deviceID)
	appRules, _ := c.apps.ListRules(deviceID)
	webPol, _ := c.websites.GetPolicy(deviceID)

	resp := PolicyBundle{
		DeviceID:         deviceID,
		FamilyName:       cfg.FamilyName,
		ParentContact:    cfg.ParentContact,
		ScreenTimePolicy: stPol,
		AppRules:         appRules,
		WebsitePolicy:    webPol,
		ServerTimeUnix:   time.Now().UTC().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (c *Coordinator) ingestChildSyncBundle(deviceID string, b SyncBundle) {
	now := time.Now().UTC()
	dateStr := now.Format("2006-01-02")

	if b.UsedTodayMinutes > 0 {
		_, _ = c.store.DB().Exec(`
			INSERT INTO daily_usage (device_id, date_str, total_seconds, blocked_attempts, login_count, updated_at)
			VALUES (?, ?, ?, 0, 1, ?)
			ON CONFLICT(device_id, date_str) DO UPDATE SET
				total_seconds = MAX(daily_usage.total_seconds, excluded.total_seconds),
				updated_at = excluded.updated_at
		`, deviceID, dateStr, b.UsedTodayMinutes*60, now)
	}

	for _, ev := range b.ActivityEvents {
		occ, err := time.Parse(time.RFC3339, ev.OccurredAt)
		if err != nil {
			occ = now
		}
		_, _ = c.store.DB().Exec(`
			INSERT INTO activity_events (device_id, event_type, app_name, executable_name, window_title, duration_seconds, occurred_at, synced)
			VALUES (?, ?, ?, ?, ?, ?, ?, 1)
		`, deviceID, ev.EventType, ev.AppName, ev.ExecutableName, ev.WindowTitle, ev.DurationSeconds, occ)
	}

	for _, au := range b.AppUsageEntries {
		_, _ = c.store.DB().Exec(`
			INSERT INTO app_usage_daily (device_id, date_str, executable_name, display_name, category, total_seconds, launch_count, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(device_id, date_str, executable_name) DO UPDATE SET
				total_seconds = excluded.total_seconds,
				launch_count = excluded.launch_count,
				updated_at = excluded.updated_at
		`, deviceID, au.DateStr, au.ExecutableName, au.DisplayName, au.Category, au.TotalSeconds, au.LaunchCount, now)
	}

	for _, ia := range b.InstalledApps {
		newInt := 0
		if ia.NewlyInstalled {
			newInt = 1
		}
		_, _ = c.store.DB().Exec(`
			INSERT INTO installed_apps (id, device_id, executable_name, display_name, version, publisher, install_path, first_seen_at, last_used_at, newly_installed)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(device_id, executable_name) DO UPDATE SET
				display_name = excluded.display_name,
				version = excluded.version,
				publisher = excluded.publisher,
				newly_installed = MAX(installed_apps.newly_installed, excluded.newly_installed)
		`, ia.ID, deviceID, ia.ExecutableName, ia.DisplayName, ia.Version, ia.Publisher, ia.InstallPath, now, now, newInt)
	}

	for _, bd := range b.BlockedDomains {
		_ = c.websites.RecordBlockedDomain(deviceID, bd.Domain, bd.Category, bd.Reason)
	}

	for _, se := range b.SecurityEvents {
		_ = c.protection.LogSecurityEvent(deviceID, se.Severity, se.Category, se.Message, se.Details)
	}
}

// PairWithParent connects from a Child computer to the Parent PC over TLS 1.3, pins the certificate,
// signs the pairing request with Ed25519, and stores the resulting credentials encrypted in SQLite.
func (c *Coordinator) PairWithParent(parentEndpoint, pairingCode, expectedParentCertFP string) (devices.PairingResponse, error) {
	parentEndpoint = strings.TrimSpace(parentEndpoint)
	parentEndpoint = strings.TrimPrefix(parentEndpoint, "https://")
	parentEndpoint = strings.TrimPrefix(parentEndpoint, "http://")
	if !strings.Contains(parentEndpoint, ":") {
		parentEndpoint = fmt.Sprintf("%s:41843", parentEndpoint)
	}

	if err := c.family.EnsureCryptographicIdentity(""); err != nil {
		return devices.PairingResponse{}, err
	}
	cfg, err := c.family.GetConfig()
	if err != nil {
		return devices.PairingResponse{}, err
	}
	privKey, pubB64, err := c.family.GetLocalEd25519PrivateKey()
	if err != nil {
		return devices.PairingResponse{}, err
	}

	normalizedCode := security.NormalizeCode(pairingCode)
	nowUnix := time.Now().UTC().Unix()
	canonical := fmt.Sprintf("PAIR:%s:%s:%s:%d", normalizedCode, cfg.LocalDeviceID, cfg.TLSFingerprint, nowUnix)
	sigB64 := security.SignMessage(privKey, []byte(canonical))

	hostname, _ := os.Hostname()
	localIPs := protection.GetLocalLANIPs()
	reqPayload := devices.PairingRequest{
		PairingCode:     pairingCode,
		DeviceID:        cfg.LocalDeviceID,
		DeviceName:      cfg.LocalDeviceName,
		Hostname:        hostname,
		OSVersion:       windows.GetOSVersionString(),
		IPAddress:       localIPs[0],
		Port:            cfg.ListenPort,
		CertFingerprint: cfg.TLSFingerprint,
		Ed25519PubKey:   pubB64,
		AgentVersion:    "1.0.0",
		TimestampUnix:   nowUnix,
		Signature:       sigB64,
	}

	bodyBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return devices.PairingResponse{}, err
	}

	// Capture parent's certificate fingerprint on initial pairing (or verify against expectedParentCertFP if provided)
	var pinnedFP string
	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS13,
		MaxVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			if len(rawCerts) == 0 {
				return errors.New("parent endpoint presented no TLS certificate")
			}
			fp := security.CertFingerprintFromDER(rawCerts[0])
			if expectedParentCertFP != "" && !strings.EqualFold(fp, expectedParentCertFP) {
				return fmt.Errorf("parent TLS certificate fingerprint mismatch: expected %s, got %s", expectedParentCertFP, fp)
			}
			pinnedFP = fp
			return nil
		},
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
			DialContext:     c.dialContext,
		},
	}

	url := fmt.Sprintf("https://%s/api/v1/pair", parentEndpoint)
	resp, err := client.Post(url, "application/json", bytes.NewReader(bodyBytes))
	if err != nil {
		return devices.PairingResponse{}, fmt.Errorf("connect to parent PC (%s): %w", parentEndpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errMsg, _ := io.ReadAll(resp.Body)
		return devices.PairingResponse{}, fmt.Errorf("pairing rejected (%d): %s", resp.StatusCode, strings.TrimSpace(string(errMsg)))
	}

	var pairResp devices.PairingResponse
	if err := json.NewDecoder(resp.Body).Decode(&pairResp); err != nil {
		return devices.PairingResponse{}, err
	}
	if pinnedFP == "" {
		pinnedFP = pairResp.ParentCertFingerprint
	}

	tokenEnc, err := c.seal.Encrypt([]byte(pairResp.DeviceToken))
	if err != nil {
		return devices.PairingResponse{}, err
	}

	_, err = c.store.DB().Exec(`
		UPDATE family_config
		SET mode = 'child',
		    family_name = ?,
		    parent_contact = ?,
		    local_child_id = ?,
		    parent_endpoint = ?,
		    parent_cert_fingerprint = ?,
		    parent_ed25519_pub = ?,
		    device_token_enc = ?,
		    setup_completed = 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, pairResp.FamilyName, pairResp.ParentContact, pairResp.ChildName, parentEndpoint, pinnedFP, pairResp.ParentEd25519Pub, tokenEnc)
	if err != nil {
		return devices.PairingResponse{}, err
	}

	_ = c.protection.LogSecurityEvent(cfg.LocalDeviceID, "info", "pairing",
		fmt.Sprintf("Paired with Parent PC at %s", parentEndpoint),
		fmt.Sprintf("Pinned TLS SHA256: %s", pinnedFP))

	return pairResp, nil
}

// SyncWithParent performs an authenticated, replay-protected TLS 1.3 delta sync from a Child PC to the Parent PC.
// If the Parent PC is offline, unsynced events remain safely queued in local SQLite and local rules continue to be enforced.
func (c *Coordinator) SyncWithParent() (PolicyBundle, error) {
	cfg, err := c.family.GetConfig()
	if err != nil {
		return PolicyBundle{}, err
	}
	if cfg.ParentEndpoint == "" {
		return PolicyBundle{}, errors.New("no parent endpoint configured")
	}

	var tokenEnc string
	err = c.store.DB().QueryRow(`SELECT device_token_enc FROM family_config WHERE id = 1`).Scan(&tokenEnc)
	if err != nil || tokenEnc == "" {
		return PolicyBundle{}, errors.New("device is not paired with a parent endpoint")
	}
	tokenBytes, err := c.seal.Decrypt(tokenEnc)
	if err != nil {
		return PolicyBundle{}, err
	}
	rawToken := string(tokenBytes)

	privKey, _, err := c.family.GetLocalEd25519PrivateKey()
	if err != nil {
		return PolicyBundle{}, err
	}

	// Build delta sync bundle from local unsynced events
	bundle := c.collectUnsyncedBundle(cfg.LocalDeviceID)
	bodyBytes, err := json.Marshal(bundle)
	if err != nil {
		return PolicyBundle{}, err
	}

	path := "/api/v1/sync"
	tsUnix := time.Now().UTC().Unix()
	nonce, _ := security.GenerateRandomToken(12)
	bodySum := sha256.Sum256(bodyBytes)
	canonical := fmt.Sprintf("POST:%s:%s:%d:%s:%s",
		path, cfg.LocalDeviceID, tsUnix, nonce, hex.EncodeToString(bodySum[:]))
	sigB64 := SignWithEd25519(privKey, []byte(canonical))

	tlsCfg := security.NewPinnedTLSClientConfig(cfg.ParentCertFingerprint, nil)
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
			DialContext:     c.dialContext,
		},
	}

	url := fmt.Sprintf("https://%s%s", cfg.ParentEndpoint, path)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return PolicyBundle{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-FC-Device-Id", cfg.LocalDeviceID)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	req.Header.Set("X-FC-Timestamp", strconv.FormatInt(tsUnix, 10))
	req.Header.Set("X-FC-Nonce", nonce)
	req.Header.Set("X-FC-Signature", sigB64)

	resp, err := client.Do(req)
	if err != nil {
		// Parent is offline: queue sync snapshot marker and continue operating locally
		c.enqueueOfflineEvent("sync_deferred", cfg.LocalDeviceID)
		return PolicyBundle{}, fmt.Errorf("parent computer unreachable (%w); enforcing local rules", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return PolicyBundle{}, fmt.Errorf("parent sync returned %d: %s", resp.StatusCode, strings.TrimSpace(string(msg)))
	}

	var policyBundle PolicyBundle
	if err := json.NewDecoder(resp.Body).Decode(&policyBundle); err != nil {
		return PolicyBundle{}, err
	}

	// Mark synced events and apply updated policies locally
	c.markEventsSynced(cfg.LocalDeviceID)
	c.applyPolicyBundleLocally(cfg.LocalDeviceID, policyBundle)

	return policyBundle, nil
}

func SignWithEd25519(priv ed25519.PrivateKey, msg []byte) string {
	return security.SignMessage(priv, msg)
}

func (c *Coordinator) collectUnsyncedBundle(deviceID string) SyncBundle {
	eval, _ := c.screenTime.Evaluate(deviceID, time.Now())
	dateStr := time.Now().UTC().Format("2006-01-02")
	appUsage, _ := c.monitoring.ListAppUsage(deviceID, dateStr)
	instApps, _ := c.apps.ListInstalledApps(deviceID)

	var unsyncedEvents []monitoring.ActivityEvent
	rows, err := c.store.DB().Query(`
		SELECT id, device_id, event_type, app_name, executable_name, window_title, duration_seconds, occurred_at
		FROM activity_events WHERE device_id = ? AND synced = 0 LIMIT 100
	`, deviceID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ev monitoring.ActivityEvent
			var occ time.Time
			if err := rows.Scan(&ev.ID, &ev.DeviceID, &ev.EventType, &ev.AppName, &ev.ExecutableName, &ev.WindowTitle, &ev.DurationSeconds, &occ); err == nil {
				ev.OccurredAt = occ.Format(time.RFC3339)
				unsyncedEvents = append(unsyncedEvents, ev)
			}
		}
	}

	activeApp := "Desktop"
	if fg, err := windows.GetForegroundActivity(); err == nil && fg.ExecutableName != "" {
		activeApp = monitoring.FriendlyAppName(fg.ExecutableName)
	}

	webPol, _ := c.websites.GetPolicy(deviceID)

	return SyncBundle{
		DeviceID:         deviceID,
		ActiveApp:        activeApp,
		UsedTodayMinutes: eval.UsedTodayMinutes,
		RemainingMinutes: eval.RemainingMinutes,
		ScreenLocked:     eval.ScreenLocked,
		DNSProtected:     webPol.DNSMode != "off",
		ActivityEvents:   unsyncedEvents,
		AppUsageEntries:  appUsage,
		InstalledApps:    instApps,
		TimestampUnix:    time.Now().UTC().Unix(),
	}
}

func (c *Coordinator) markEventsSynced(deviceID string) {
	_, _ = c.store.DB().Exec(`UPDATE activity_events SET synced = 1 WHERE device_id = ? AND synced = 0`, deviceID)
	_, _ = c.store.DB().Exec(`UPDATE security_events SET synced = 1 WHERE device_id = ? AND synced = 0`, deviceID)
	_, _ = c.store.DB().Exec(`DELETE FROM sync_queue`)
}

func (c *Coordinator) enqueueOfflineEvent(eventType, details string) {
	_, _ = c.store.DB().Exec(`
		INSERT INTO sync_queue (payload_type, payload_json, created_at, retry_count)
		VALUES (?, ?, CURRENT_TIMESTAMP, 1)
	`, eventType, details)
}

func (c *Coordinator) applyPolicyBundleLocally(localDeviceID string, pb PolicyBundle) {
	if pb.FamilyName != "" || pb.ParentContact != "" {
		_, _ = c.store.DB().Exec(`
			UPDATE family_config SET family_name = ?, parent_contact = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1
		`, pb.FamilyName, pb.ParentContact)
	}

	pb.ScreenTimePolicy.DeviceID = localDeviceID
	_, _ = c.screenTime.SavePolicy(pb.ScreenTimePolicy)

	pb.WebsitePolicy.DeviceID = localDeviceID
	_, _ = c.websites.SavePolicy(pb.WebsitePolicy)
	_ = c.websites.EnforceLocalPolicy(localDeviceID)

	// Synchronize app rules
	for _, r := range pb.AppRules {
		r.DeviceID = localDeviceID
		_, _ = c.apps.SaveRule(r)
	}
}
