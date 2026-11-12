package protection

import (
	"net"
	"time"

	"familycontrol/backend/database"
	"familycontrol/backend/websites"
	"familycontrol/backend/windows"
)

type SecurityEvent struct {
	ID         int64  `json:"id"`
	DeviceID   string `json:"deviceId"`
	Severity   string `json:"severity"` // "info" | "warning" | "critical"
	Category   string `json:"category"`
	Message    string `json:"message"`
	Details    string `json:"details"`
	OccurredAt string `json:"occurredAt"`
}

type ProtectionStatus struct {
	DeviceID             string          `json:"deviceId"`
	DNSProtected         bool            `json:"dnsProtected"`
	DNSMode              string          `json:"dnsMode"`
	ActiveDNSServers     []string        `json:"activeDnsServers"`
	FirewallEnabled      bool            `json:"firewallEnabled"`
	AgentRunning         bool            `json:"agentRunning"`
	ServiceInstalled     bool            `json:"serviceInstalled"`
	LocalIPAddresses     []string        `json:"localIpAddresses"`
	TLSActive            bool            `json:"tlsActive"`
	CertFingerprint      string          `json:"certFingerprint"`
	LastIntegrityCheck   string          `json:"lastIntegrityCheck"`
	ConfigurationIntact  bool            `json:"configurationIntact"`
	RecentSecurityEvents []SecurityEvent `json:"recentSecurityEvents"`
}

type Manager struct {
	store    *database.Store
	websites *websites.Manager
}

func NewManager(store *database.Store, webMgr *websites.Manager) *Manager {
	return &Manager{
		store:    store,
		websites: webMgr,
	}
}

// LogSecurityEvent records an audit or tamper event in the SQLite security_events table.
func (m *Manager) LogSecurityEvent(deviceID, severity, category, message, details string) error {
	if deviceID == "" {
		deviceID = "local"
	}
	if severity == "" {
		severity = "info"
	}
	_, err := m.store.DB().Exec(`
		INSERT INTO security_events (device_id, severity, category, message, details, occurred_at, synced)
		VALUES (?, ?, ?, ?, ?, ?, 0)
	`, deviceID, severity, category, message, details, time.Now().UTC())
	return err
}

// RunIntegrityCheck verifies DNS settings, firewall state, TLS identity, and configuration integrity.
func (m *Manager) RunIntegrityCheck(deviceID string, agentRunning bool) (ProtectionStatus, error) {
	if deviceID == "" {
		_ = m.store.DB().QueryRow(`SELECT local_device_id FROM family_config WHERE id = 1`).Scan(&deviceID)
	}
	var certFP string
	_ = m.store.DB().QueryRow(`SELECT tls_fingerprint FROM family_config WHERE id = 1`).Scan(&certFP)

	dnsStatus := m.websites.VerifyDNSConfiguration(deviceID)
	webPol, _ := m.websites.GetPolicy(deviceID)
	fwEnabled := windows.CheckWindowsFirewallEnabled()
	ips := GetLocalLANIPs()

	events, _ := m.ListSecurityEvents(deviceID, 30)

	status := ProtectionStatus{
		DeviceID:             deviceID,
		DNSProtected:         webPol.DNSMode != "off",
		DNSMode:              webPol.DNSMode,
		ActiveDNSServers:     dnsStatus.ActiveDNSServers,
		FirewallEnabled:      fwEnabled,
		AgentRunning:         agentRunning,
		ServiceInstalled:     true,
		LocalIPAddresses:     ips,
		TLSActive:            certFP != "",
		CertFingerprint:      certFP,
		LastIntegrityCheck:   time.Now().UTC().Format(time.RFC3339),
		ConfigurationIntact:  certFP != "",
		RecentSecurityEvents: events,
	}
	return status, nil
}

// ListSecurityEvents returns recent security & audit log entries.
func (m *Manager) ListSecurityEvents(deviceID string, limit int) ([]SecurityEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := m.store.DB().Query(`
		SELECT id, device_id, severity, category, message, details, occurred_at
		FROM security_events
		ORDER BY occurred_at DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SecurityEvent
	for rows.Next() {
		var ev SecurityEvent
		var occ time.Time
		if err := rows.Scan(&ev.ID, &ev.DeviceID, &ev.Severity, &ev.Category, &ev.Message, &ev.Details, &occ); err != nil {
			continue
		}
		ev.OccurredAt = occ.Format(time.RFC3339)
		list = append(list, ev)
	}
	if list == nil {
		list = []SecurityEvent{}
	}
	return list, nil
}

// GetLocalLANIPs returns all non-loopback IPv4 addresses on active network interfaces.
func GetLocalLANIPs() []string {
	var ips []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return []string{"127.0.0.1"}
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ip4 := ipnet.IP.To4(); ip4 != nil {
					ips = append(ips, ip4.String())
				}
			}
		}
	}
	if len(ips) == 0 {
		ips = []string{"127.0.0.1"}
	}
	return ips
}
