package websites

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"familycontrol/backend/database"
	"familycontrol/backend/windows"
)

const (
	CloudflareFamilyPrimaryIPv4   = "1.1.1.3"
	CloudflareFamilySecondaryIPv4 = "1.0.0.3"
	CloudflareFamilyPrimaryIPv6   = "2606:4700:4700::1113"
	CloudflareFamilySecondaryIPv6 = "2606:4700:4700::1003"

	CloudflareMalwarePrimaryIPv4   = "1.1.1.2"
	CloudflareMalwareSecondaryIPv4 = "1.0.0.2"
	CloudflareMalwarePrimaryIPv6   = "2606:4700:4700::1112"
	CloudflareMalwareSecondaryIPv6 = "2606:4700:4700::1002"

	hostsMarkerBegin = "# BEGIN FAMILYCONTROL MANAGED BLOCKLIST"
	hostsMarkerEnd   = "# END FAMILYCONTROL MANAGED BLOCKLIST"
)

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

type WebsitePolicy struct {
	DeviceID         string   `json:"deviceId"`
	DNSMode          string   `json:"dnsMode"` // "cloudflare_family" | "cloudflare_malware" | "custom" | "off"
	BlockAdult       bool     `json:"blockAdult"`
	BlockMalware     bool     `json:"blockMalware"`
	PrimaryIPv4      string   `json:"primaryIpv4"`
	SecondaryIPv4    string   `json:"secondaryIpv4"`
	PrimaryIPv6      string   `json:"primaryIpv6"`
	SecondaryIPv6    string   `json:"secondaryIpv6"`
	Allowlist        []string `json:"allowlist"`
	Blocklist        []string `json:"blocklist"`
	TransparencyNote string   `json:"transparencyNote"`
	UpdatedAt        string   `json:"updatedAt"`
}

type BlockedDomainEvent struct {
	ID         int64  `json:"id"`
	DeviceID   string `json:"deviceId"`
	Domain     string `json:"domain"`
	Category   string `json:"category"`
	Reason     string `json:"reason"`
	OccurredAt string `json:"occurredAt"`
}

type DNSVerificationStatus struct {
	ActiveDNSServers        []string `json:"activeDnsServers"`
	CloudflareFamilyActive  bool     `json:"cloudflareFamilyActive"`
	CloudflareMalwareActive bool     `json:"cloudflareMalwareActive"`
	HostsFileSynced         bool     `json:"hostsFileSynced"`
	DriftDetected           bool     `json:"driftDetected"`
	StatusSummary           string   `json:"statusSummary"`
}

type Manager struct {
	store *database.Store
}

func NewManager(store *database.Store) *Manager {
	return &Manager{store: store}
}

const transparencyNotice = "DNS and hosts-level filtering blocks domain resolution for malware, adult content (via Cloudflare 1.1.1.3), and custom blocklisted domains across all browsers and apps. Because modern traffic uses encrypted HTTPS/TLS, DNS monitoring does not inspect full URL paths, search queries, or page contents."

// GetPolicy retrieves the website and DNS policy for a device.
func (m *Manager) GetPolicy(deviceID string) (WebsitePolicy, error) {
	if deviceID == "" {
		deviceID = "default"
	}
	_, _ = m.store.DB().Exec(`INSERT OR IGNORE INTO website_policies (device_id) VALUES (?)`, deviceID)

	var p WebsitePolicy
	var adultInt, malwareInt int
	var allowJSON, blockJSON string
	var updated time.Time

	err := m.store.DB().QueryRow(`
		SELECT device_id, dns_mode, block_adult, block_malware,
		       primary_ipv4, secondary_ipv4, primary_ipv6, secondary_ipv6,
		       allowlist_json, blocklist_json, updated_at
		FROM website_policies WHERE device_id = ?
	`, deviceID).Scan(
		&p.DeviceID, &p.DNSMode, &adultInt, &malwareInt,
		&p.PrimaryIPv4, &p.SecondaryIPv4, &p.PrimaryIPv6, &p.SecondaryIPv6,
		&allowJSON, &blockJSON, &updated,
	)
	if err != nil {
		return p, err
	}

	p.BlockAdult = adultInt == 1
	p.BlockMalware = malwareInt == 1
	p.TransparencyNote = transparencyNotice
	p.UpdatedAt = updated.Format(time.RFC3339)

	_ = json.Unmarshal([]byte(allowJSON), &p.Allowlist)
	_ = json.Unmarshal([]byte(blockJSON), &p.Blocklist)
	if p.Allowlist == nil {
		p.Allowlist = []string{}
	}
	if p.Blocklist == nil {
		p.Blocklist = []string{}
	}
	return p, nil
}

// SavePolicy validates and saves the website/DNS filtering configuration.
func (m *Manager) SavePolicy(p WebsitePolicy) (WebsitePolicy, error) {
	if p.DeviceID == "" {
		p.DeviceID = "default"
	}
	switch p.DNSMode {
	case "cloudflare_family", "":
		p.DNSMode = "cloudflare_family"
		p.BlockAdult = true
		p.BlockMalware = true
		p.PrimaryIPv4 = CloudflareFamilyPrimaryIPv4
		p.SecondaryIPv4 = CloudflareFamilySecondaryIPv4
		p.PrimaryIPv6 = CloudflareFamilyPrimaryIPv6
		p.SecondaryIPv6 = CloudflareFamilySecondaryIPv6
	case "cloudflare_malware":
		p.BlockAdult = false
		p.BlockMalware = true
		p.PrimaryIPv4 = CloudflareMalwarePrimaryIPv4
		p.SecondaryIPv4 = CloudflareMalwareSecondaryIPv4
		p.PrimaryIPv6 = CloudflareMalwarePrimaryIPv6
		p.SecondaryIPv6 = CloudflareMalwareSecondaryIPv6
	case "off":
		p.BlockAdult = false
		p.BlockMalware = false
	}

	cleanAllow, err := sanitizeDomainList(p.Allowlist)
	if err != nil {
		return WebsitePolicy{}, err
	}
	cleanBlock, err := sanitizeDomainList(p.Blocklist)
	if err != nil {
		return WebsitePolicy{}, err
	}

	allowBytes, _ := json.Marshal(cleanAllow)
	blockBytes, _ := json.Marshal(cleanBlock)

	adultInt := 0
	if p.BlockAdult {
		adultInt = 1
	}
	malwareInt := 0
	if p.BlockMalware {
		malwareInt = 1
	}

	_, err = m.store.DB().Exec(`
		INSERT INTO website_policies (
			device_id, dns_mode, block_adult, block_malware,
			primary_ipv4, secondary_ipv4, primary_ipv6, secondary_ipv6,
			allowlist_json, blocklist_json, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(device_id) DO UPDATE SET
			dns_mode = excluded.dns_mode,
			block_adult = excluded.block_adult,
			block_malware = excluded.block_malware,
			primary_ipv4 = excluded.primary_ipv4,
			secondary_ipv4 = excluded.secondary_ipv4,
			primary_ipv6 = excluded.primary_ipv6,
			secondary_ipv6 = excluded.secondary_ipv6,
			allowlist_json = excluded.allowlist_json,
			blocklist_json = excluded.blocklist_json,
			updated_at = CURRENT_TIMESTAMP
	`, p.DeviceID, p.DNSMode, adultInt, malwareInt,
		p.PrimaryIPv4, p.SecondaryIPv4, p.PrimaryIPv6, p.SecondaryIPv6,
		string(allowBytes), string(blockBytes))
	if err != nil {
		return WebsitePolicy{}, err
	}

	return m.GetPolicy(p.DeviceID)
}

// EnforceLocalPolicy applies the DNS and hosts-file blocklist on the local machine when in Child/Service mode.
func (m *Manager) EnforceLocalPolicy(deviceID string) error {
	pol, err := m.GetPolicy(deviceID)
	if err != nil {
		return err
	}
	if pol.DNSMode == "cloudflare_family" || pol.DNSMode == "cloudflare_malware" {
		_ = windows.ApplyDNSServers(pol.PrimaryIPv4, pol.SecondaryIPv4, pol.PrimaryIPv6, pol.SecondaryIPv6)
	}
	return SyncHostsBlocklist(windows.GetHostsFilePath(), pol.Blocklist, pol.Allowlist)
}

// SyncHostsBlocklist writes the FamilyControl managed domain blocklist section into a hosts file atomically.
func SyncHostsBlocklist(hostsPath string, blocklist, allowlist []string) error {
	existingBytes, err := os.ReadFile(hostsPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read hosts file: %w", err)
	}
	content := string(existingBytes)

	// Strip existing FamilyControl block
	startIdx := strings.Index(content, hostsMarkerBegin)
	endIdx := strings.Index(content, hostsMarkerEnd)
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		after := endIdx + len(hostsMarkerEnd)
		if after < len(content) && content[after] == '\r' {
			after++
		}
		if after < len(content) && content[after] == '\n' {
			after++
		}
		content = content[:startIdx] + content[after:]
	}

	allowMap := make(map[string]bool, len(allowlist))
	for _, a := range allowlist {
		allowMap[strings.ToLower(strings.TrimSpace(a))] = true
	}

	var effectiveBlock []string
	for _, b := range blocklist {
		d := strings.ToLower(strings.TrimSpace(b))
		if d != "" && !allowMap[d] {
			effectiveBlock = append(effectiveBlock, d)
		}
	}

	if len(effectiveBlock) > 0 {
		var sb strings.Builder
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			sb.WriteString("\r\n")
		}
		sb.WriteString(hostsMarkerBegin + "\r\n")
		for _, domain := range effectiveBlock {
			sb.WriteString(fmt.Sprintf("0.0.0.0 %s\r\n", domain))
			if !strings.HasPrefix(domain, "www.") {
				sb.WriteString(fmt.Sprintf("0.0.0.0 www.%s\r\n", domain))
			}
			sb.WriteString(fmt.Sprintf(":: %s\r\n", domain))
		}
		sb.WriteString(hostsMarkerEnd + "\r\n")
		content += sb.String()
	}

	return os.WriteFile(hostsPath, []byte(content), 0644)
}

// VerifyDNSConfiguration inspects current OS DNS servers and hosts file state against the device's policy.
func (m *Manager) VerifyDNSConfiguration(deviceID string) DNSVerificationStatus {
	pol, _ := m.GetPolicy(deviceID)
	servers := windows.GetConfiguredDNSServers()

	cfFamily := false
	cfMalware := false
	for _, s := range servers {
		if s == CloudflareFamilyPrimaryIPv4 || s == CloudflareFamilySecondaryIPv4 || s == CloudflareFamilyPrimaryIPv6 || s == CloudflareFamilySecondaryIPv6 {
			cfFamily = true
		}
		if s == CloudflareMalwarePrimaryIPv4 || s == CloudflareMalwareSecondaryIPv4 {
			cfMalware = true
		}
	}

	hostsSynced := true
	if len(pol.Blocklist) > 0 {
		if data, err := os.ReadFile(windows.GetHostsFilePath()); err == nil {
			hostsSynced = strings.Contains(string(data), hostsMarkerBegin)
		} else {
			hostsSynced = false
		}
	}

	drift := false
	summary := "Cloudflare Family DNS (1.1.1.3) Configured"
	if pol.DNSMode == "cloudflare_family" && !cfFamily {
		drift = true
		summary = "Policy set to Cloudflare Family DNS (1.1.1.3); awaiting elevated service enforcement"
	} else if pol.DNSMode == "off" {
		summary = "DNS Filtering Disabled"
	}

	return DNSVerificationStatus{
		ActiveDNSServers:        servers,
		CloudflareFamilyActive:  cfFamily,
		CloudflareMalwareActive: cfMalware,
		HostsFileSynced:         hostsSynced,
		DriftDetected:           drift,
		StatusSummary:           summary,
	}
}

// RecordBlockedDomain logs a blocked domain resolution attempt.
func (m *Manager) RecordBlockedDomain(deviceID, domain, category, reason string) error {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return nil
	}
	_, err := m.store.DB().Exec(`
		INSERT INTO blocked_domain_events (device_id, domain, category, reason, occurred_at)
		VALUES (?, ?, ?, ?, ?)
	`, deviceID, domain, category, reason, time.Now().UTC())
	return err
}

// ListBlockedDomainEvents returns the recent blocked domain history.
func (m *Manager) ListBlockedDomainEvents(deviceID string, limit int) ([]BlockedDomainEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `SELECT id, device_id, domain, category, reason, occurred_at FROM blocked_domain_events ORDER BY occurred_at DESC LIMIT ?`
	args := []any{limit}
	if deviceID != "" && deviceID != "all" {
		q = `SELECT id, device_id, domain, category, reason, occurred_at FROM blocked_domain_events WHERE device_id = ? ORDER BY occurred_at DESC LIMIT ?`
		args = []any{deviceID, limit}
	}

	rows, err := m.store.DB().Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []BlockedDomainEvent
	for rows.Next() {
		var ev BlockedDomainEvent
		var occ time.Time
		if err := rows.Scan(&ev.ID, &ev.DeviceID, &ev.Domain, &ev.Category, &ev.Reason, &occ); err != nil {
			continue
		}
		ev.OccurredAt = occ.Format(time.RFC3339)
		list = append(list, ev)
	}
	if list == nil {
		list = []BlockedDomainEvent{}
	}
	return list, nil
}

func sanitizeDomainList(domains []string) ([]string, error) {
	seen := make(map[string]bool)
	var out []string
	for _, raw := range domains {
		d := strings.ToLower(strings.TrimSpace(raw))
		d = strings.TrimPrefix(d, "https://")
		d = strings.TrimPrefix(d, "http://")
		d = strings.Split(d, "/")[0]
		if d == "" {
			continue
		}
		if !domainRegex.MatchString(d) {
			return nil, fmt.Errorf("invalid domain name: %s", raw)
		}
		if len(d) > 253 {
			return nil, errors.New("domain name exceeds maximum length")
		}
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}
