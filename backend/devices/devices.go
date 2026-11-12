package devices

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"familycontrol/backend/database"
	"familycontrol/backend/security"
)

type Device struct {
	ID               string `json:"id"`
	ChildID          string `json:"childId"`
	ChildName        string `json:"childName"`
	Name             string `json:"name"`
	Hostname         string `json:"hostname"`
	OSVersion        string `json:"osVersion"`
	IPAddress        string `json:"ipAddress"`
	Port             int    `json:"port"`
	CertFingerprint  string `json:"certFingerprint"`
	Ed25519PubKey    string `json:"ed25519PubKey"`
	Status           string `json:"status"`
	ScreenLocked     bool   `json:"screenLocked"`
	ManualPause      bool   `json:"manualPause"`
	RemainingMinutes int    `json:"remainingMinutes"`
	UsedTodayMinutes int    `json:"usedTodayMinutes"`
	ActiveApp        string `json:"activeApp"`
	DNSProtected     bool   `json:"dnsProtected"`
	AgentVersion     string `json:"agentVersion"`
	LastSeen         string `json:"lastSeen"`
	PairedAt         string `json:"pairedAt"`
}

type PairingTicket struct {
	Code           string `json:"code"`
	ChildID        string `json:"childId"`
	ChildName      string `json:"childName"`
	DeviceNameHint string `json:"deviceNameHint"`
	ExpiresAt      string `json:"expiresAt"`
	ParentEndpoint string `json:"parentEndpoint"`
	ParentCertFP   string `json:"parentCertFingerprint"`
}

type PairingRequest struct {
	PairingCode     string `json:"pairingCode"`
	DeviceID        string `json:"deviceId"`
	DeviceName      string `json:"deviceName"`
	Hostname        string `json:"hostname"`
	OSVersion       string `json:"osVersion"`
	IPAddress       string `json:"ipAddress"`
	Port            int    `json:"port"`
	CertFingerprint string `json:"certFingerprint"`
	Ed25519PubKey   string `json:"ed25519PubKey"`
	AgentVersion    string `json:"agentVersion"`
	TimestampUnix   int64  `json:"timestampUnix"`
	Signature       string `json:"signature"`
}

type PairingResponse struct {
	DeviceID              string `json:"deviceId"`
	ChildID               string `json:"childId"`
	ChildName             string `json:"childName"`
	DeviceToken           string `json:"deviceToken"`
	ParentDeviceID        string `json:"parentDeviceId"`
	ParentEd25519Pub      string `json:"parentEd25519Pub"`
	ParentCertFingerprint string `json:"parentCertFingerprint"`
	FamilyName            string `json:"familyName"`
	ParentContact         string `json:"parentContact"`
}

type Manager struct {
	store *database.Store
}

func NewManager(store *database.Store) *Manager {
	return &Manager{store: store}
}

// CreatePairingCode generates a short-lived one-time pairing code (valid for 15 minutes).
func (m *Manager) CreatePairingCode(childID, deviceNameHint, parentEndpoint, parentCertFP string) (PairingTicket, error) {
	codeDisplay, err := security.GeneratePairingCode()
	if err != nil {
		return PairingTicket{}, err
	}
	normalized := security.NormalizeCode(codeDisplay)
	codeHash := security.SHA256Hex(normalized)

	var childName string
	if childID != "" {
		_ = m.store.DB().QueryRow(`SELECT name FROM children WHERE id = ?`, childID).Scan(&childName)
	}
	expiresAt := time.Now().UTC().Add(15 * time.Minute)

	_, err = m.store.DB().Exec(`
		INSERT INTO pairing_codes (code_hash, code_display, child_id, child_name, device_name_hint, expires_at, used)
		VALUES (?, ?, ?, ?, ?, ?, 0)
	`, codeHash, codeDisplay, childID, childName, strings.TrimSpace(deviceNameHint), expiresAt)
	if err != nil {
		return PairingTicket{}, fmt.Errorf("store pairing code: %w", err)
	}

	return PairingTicket{
		Code:           codeDisplay,
		ChildID:        childID,
		ChildName:      childName,
		DeviceNameHint: deviceNameHint,
		ExpiresAt:      expiresAt.Format(time.RFC3339),
		ParentEndpoint: parentEndpoint,
		ParentCertFP:   parentCertFP,
	}, nil
}

// CompletePairing validates a child's pairing request, verifies the Ed25519 signature, consumes the one-time code,
// registers the device, creates default policies, and returns the device's authentication credentials.
func (m *Manager) CompletePairing(req PairingRequest) (PairingResponse, error) {
	normalized := security.NormalizeCode(req.PairingCode)
	if normalized == "" {
		return PairingResponse{}, errors.New("pairing code is required")
	}
	if req.DeviceID == "" || req.Ed25519PubKey == "" || req.CertFingerprint == "" {
		return PairingResponse{}, errors.New("incomplete cryptographic identity in pairing request")
	}

	// Verify Ed25519 signature over canonical pairing message
	canonical := fmt.Sprintf("PAIR:%s:%s:%s:%d", normalized, req.DeviceID, req.CertFingerprint, req.TimestampUnix)
	if !security.VerifyMessageSignature(req.Ed25519PubKey, []byte(canonical), req.Signature) {
		return PairingResponse{}, errors.New("invalid Ed25519 signature on pairing request")
	}

	codeHash := security.SHA256Hex(normalized)
	var childID, childName, nameHint string
	var expiresAt time.Time
	var used int

	err := m.store.DB().QueryRow(`
		SELECT child_id, child_name, device_name_hint, expires_at, used
		FROM pairing_codes WHERE code_hash = ?
	`, codeHash).Scan(&childID, &childName, &nameHint, &expiresAt, &used)
	if err != nil {
		return PairingResponse{}, errors.New("invalid or unknown pairing code")
	}
	if used != 0 {
		return PairingResponse{}, errors.New("pairing code has already been used")
	}
	if time.Now().UTC().After(expiresAt) {
		return PairingResponse{}, errors.New("pairing code has expired")
	}

	deviceToken, err := security.GenerateRandomToken(32)
	if err != nil {
		return PairingResponse{}, err
	}
	tokenHash := security.SHA256Hex(deviceToken)

	devName := strings.TrimSpace(req.DeviceName)
	if nameHint != "" {
		devName = nameHint
	}
	if devName == "" {
		devName = req.Hostname
	}
	if devName == "" {
		devName = "Child PC"
	}

	now := time.Now().UTC()
	err = m.store.WithTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE pairing_codes SET used = 1 WHERE code_hash = ?`, codeHash)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			INSERT INTO devices (
				id, child_id, child_name, name, hostname, os_version, ip_address, port,
				cert_fingerprint, ed25519_pub_key, auth_token_hash, status,
				screen_locked, manual_pause, remaining_minutes, used_today_minutes,
				active_app, dns_protected, agent_version, last_seen, paired_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'online', 0, 0, 120, 0, 'Desktop', 1, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				child_id = excluded.child_id,
				child_name = excluded.child_name,
				name = excluded.name,
				hostname = excluded.hostname,
				os_version = excluded.os_version,
				ip_address = excluded.ip_address,
				port = excluded.port,
				cert_fingerprint = excluded.cert_fingerprint,
				ed25519_pub_key = excluded.ed25519_pub_key,
				auth_token_hash = excluded.auth_token_hash,
				status = 'online',
				last_seen = excluded.last_seen
		`, req.DeviceID, childID, childName, devName, req.Hostname, req.OSVersion, req.IPAddress, req.Port,
			req.CertFingerprint, req.Ed25519PubKey, tokenHash, req.AgentVersion, now, now)
		if err != nil {
			return err
		}

		// Initialize default policies for the newly paired device
		_, _ = tx.Exec(`INSERT OR IGNORE INTO screen_time_policies (device_id) VALUES (?)`, req.DeviceID)
		_, _ = tx.Exec(`INSERT OR IGNORE INTO website_policies (device_id) VALUES (?)`, req.DeviceID)
		return nil
	})
	if err != nil {
		return PairingResponse{}, err
	}

	var parentDevID, parentPub, parentFP, familyName, parentContact string
	_ = m.store.DB().QueryRow(`
		SELECT local_device_id, local_ed25519_pub, tls_fingerprint, family_name, parent_contact
		FROM family_config WHERE id = 1
	`).Scan(&parentDevID, &parentPub, &parentFP, &familyName, &parentContact)

	return PairingResponse{
		DeviceID:              req.DeviceID,
		ChildID:               childID,
		ChildName:             childName,
		DeviceToken:           deviceToken,
		ParentDeviceID:        parentDevID,
		ParentEd25519Pub:      parentPub,
		ParentCertFingerprint: parentFP,
		FamilyName:            familyName,
		ParentContact:         parentContact,
	}, nil
}

// VerifyDeviceCredentials checks a paired device's token and returns its stored Ed25519 public key and TLS fingerprint.
func (m *Manager) VerifyDeviceCredentials(deviceID, rawToken string) (ed25519Pub string, certFP string, err error) {
	if deviceID == "" || rawToken == "" {
		return "", "", errors.New("missing device credentials")
	}
	var tokenHash, status string
	err = m.store.DB().QueryRow(`
		SELECT auth_token_hash, ed25519_pub_key, cert_fingerprint, status
		FROM devices WHERE id = ?
	`, deviceID).Scan(&tokenHash, &ed25519Pub, &certFP, &status)
	if err != nil {
		return "", "", errors.New("unknown device")
	}
	if status == "revoked" {
		return "", "", errors.New("device credentials have been revoked")
	}
	if security.SHA256Hex(rawToken) != tokenHash {
		return "", "", errors.New("invalid device token")
	}
	return ed25519Pub, certFP, nil
}

// ListDevices returns all non-revoked devices and updates online/offline status based on heartbeat freshness.
func (m *Manager) ListDevices() ([]Device, error) {
	rows, err := m.store.DB().Query(`
		SELECT id, child_id, child_name, name, hostname, os_version, ip_address, port,
		       cert_fingerprint, ed25519_pub_key, status, screen_locked, manual_pause,
		       remaining_minutes, used_today_minutes, active_app, dns_protected,
		       agent_version, last_seen, paired_at
		FROM devices
		WHERE status != 'revoked'
		ORDER BY paired_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []Device
	now := time.Now().UTC()
	for rows.Next() {
		var d Device
		var lockedInt, pauseInt, dnsInt int
		var lastSeen, pairedAt time.Time
		if err := rows.Scan(
			&d.ID, &d.ChildID, &d.ChildName, &d.Name, &d.Hostname, &d.OSVersion, &d.IPAddress, &d.Port,
			&d.CertFingerprint, &d.Ed25519PubKey, &d.Status, &lockedInt, &pauseInt,
			&d.RemainingMinutes, &d.UsedTodayMinutes, &d.ActiveApp, &dnsInt,
			&d.AgentVersion, &lastSeen, &pairedAt,
		); err != nil {
			return nil, err
		}
		d.ScreenLocked = lockedInt == 1
		d.ManualPause = pauseInt == 1
		d.DNSProtected = dnsInt == 1
		d.LastSeen = lastSeen.Format(time.RFC3339)
		d.PairedAt = pairedAt.Format(time.RFC3339)

		if d.Status == "online" && now.Sub(lastSeen) > 2*time.Minute {
			d.Status = "offline"
		}
		list = append(list, d)
	}
	if list == nil {
		list = []Device{}
	}
	return list, nil
}

// GetDevice returns a single device by ID.
func (m *Manager) GetDevice(deviceID string) (Device, error) {
	list, err := m.ListDevices()
	if err != nil {
		return Device{}, err
	}
	for _, d := range list {
		if d.ID == deviceID {
			return d, nil
		}
	}
	return Device{}, errors.New("device not found")
}

// RenameDevice updates a device's friendly name and optionally reassigns its child profile.
func (m *Manager) RenameDevice(deviceID, newName, childID string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("device name cannot be empty")
	}
	var childName string
	if childID != "" {
		_ = m.store.DB().QueryRow(`SELECT name FROM children WHERE id = ?`, childID).Scan(&childName)
	}
	_, err := m.store.DB().Exec(`
		UPDATE devices SET name = ?, child_id = ?, child_name = ? WHERE id = ?
	`, newName, childID, childName, deviceID)
	return err
}

// RevokeDevice revokes a paired device's credentials and removes it from active coordination.
func (m *Manager) RevokeDevice(deviceID string) error {
	_, err := m.store.DB().Exec(`
		UPDATE devices SET status = 'revoked', auth_token_hash = '' WHERE id = ?
	`, deviceID)
	return err
}

// DeleteDevice permanently removes a device and its associated rules.
func (m *Manager) DeleteDevice(deviceID string) error {
	return m.store.WithTx(func(tx *sql.Tx) error {
		_, _ = tx.Exec(`DELETE FROM screen_time_policies WHERE device_id = ?`, deviceID)
		_, _ = tx.Exec(`DELETE FROM website_policies WHERE device_id = ?`, deviceID)
		_, _ = tx.Exec(`DELETE FROM app_rules WHERE device_id = ?`, deviceID)
		_, err := tx.Exec(`DELETE FROM devices WHERE id = ?`, deviceID)
		return err
	})
}

// UpdateHeartbeat updates the live telemetry of a paired child device.
func (m *Manager) UpdateHeartbeat(deviceID, ipAddress, activeApp string, usedMinutes, remainingMinutes int, screenLocked, dnsProtected bool) error {
	lockedInt := 0
	if screenLocked {
		lockedInt = 1
	}
	dnsInt := 0
	if dnsProtected {
		dnsInt = 1
	}
	_, err := m.store.DB().Exec(`
		UPDATE devices
		SET status = 'online',
		    ip_address = CASE WHEN ? != '' THEN ? ELSE ip_address END,
		    active_app = ?,
		    used_today_minutes = ?,
		    remaining_minutes = ?,
		    screen_locked = ?,
		    dns_protected = ?,
		    last_seen = ?
		WHERE id = ? AND status != 'revoked'
	`, ipAddress, ipAddress, activeApp, usedMinutes, remainingMinutes, lockedInt, dnsInt, time.Now().UTC(), deviceID)
	return err
}
