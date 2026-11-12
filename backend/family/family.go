package family

import (
	"crypto/ed25519"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"familycontrol/backend/database"
	"familycontrol/backend/security"

	"github.com/google/uuid"
)

type OperatingMode string

const (
	ModeUnconfigured OperatingMode = "unconfigured"
	ModeParent       OperatingMode = "parent"
	ModeChild        OperatingMode = "child"
)

// BuildEdition defines the compiled installer edition ("unified", "parent", or "child").
// Overridden via -ldflags "-X familycontrol/backend/family.BuildEdition=parent|child".
var BuildEdition = "unified"

type Config struct {
	Mode                  OperatingMode `json:"mode"`
	BuildEdition          string        `json:"buildEdition"`
	FamilyName            string        `json:"familyName"`
	ParentContact         string        `json:"parentContact"`
	LocalDeviceID         string        `json:"localDeviceId"`
	LocalDeviceName       string        `json:"localDeviceName"`
	LocalChildID          string        `json:"localChildId"`
	LocalEd25519Pub       string        `json:"localEd25519Pub"`
	TLSFingerprint        string        `json:"tlsFingerprint"`
	ListenPort            int           `json:"listenPort"`
	DiscoveryPort         int           `json:"discoveryPort"`
	ParentEndpoint        string        `json:"parentEndpoint"`
	ParentCertFingerprint string        `json:"parentCertFingerprint"`
	ParentEd25519Pub      string        `json:"parentEd25519Pub"`
	RetentionDays         int           `json:"retentionDays"`
	SetupCompleted        bool          `json:"setupCompleted"`
	UpdatedAt             string        `json:"updatedAt"`
}

type ChildProfile struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AvatarColor string `json:"avatarColor"`
	AgeGroup    string `json:"ageGroup"`
	DeviceCount int    `json:"deviceCount"`
	CreatedAt   string `json:"createdAt"`
}

type SetupParentResult struct {
	Config       Config `json:"config"`
	RecoveryCode string `json:"recoveryCode"`
	SessionToken string `json:"sessionToken"`
}

type Manager struct {
	store *database.Store
	seal  *security.LocalSealManager
	auth  *security.AuthManager
}

func NewManager(store *database.Store, seal *security.LocalSealManager, auth *security.AuthManager) *Manager {
	return &Manager{
		store: store,
		seal:  seal,
		auth:  auth,
	}
}

// GetConfig loads the public family configuration from SQLite.
func (m *Manager) GetConfig() (Config, error) {
	var cfg Config
	var modeStr string
	var setupInt int
	var updated time.Time

	err := m.store.DB().QueryRow(`
		SELECT mode, family_name, parent_contact, local_device_id, local_device_name,
		       local_child_id, local_ed25519_pub, tls_fingerprint, listen_port, discovery_port,
		       parent_endpoint, parent_cert_fingerprint, parent_ed25519_pub, retention_days,
		       setup_completed, updated_at
		FROM family_config WHERE id = 1
	`).Scan(
		&modeStr, &cfg.FamilyName, &cfg.ParentContact, &cfg.LocalDeviceID, &cfg.LocalDeviceName,
		&cfg.LocalChildID, &cfg.LocalEd25519Pub, &cfg.TLSFingerprint, &cfg.ListenPort, &cfg.DiscoveryPort,
		&cfg.ParentEndpoint, &cfg.ParentCertFingerprint, &cfg.ParentEd25519Pub, &cfg.RetentionDays,
		&setupInt, &updated,
	)
	if err != nil {
		return cfg, err
	}
	cfg.Mode = OperatingMode(modeStr)
	cfg.BuildEdition = BuildEdition
	cfg.SetupCompleted = setupInt == 1
	cfg.UpdatedAt = updated.Format(time.RFC3339)

	if cfg.LocalDeviceName == "" {
		if host, err := os.Hostname(); err == nil && host != "" {
			cfg.LocalDeviceName = host
		} else {
			cfg.LocalDeviceName = "Windows-PC"
		}
	}
	return cfg, nil
}

// EnsureCryptographicIdentity ensures the local node has an Ed25519 keypair and TLS 1.3 certificate.
func (m *Manager) EnsureCryptographicIdentity(deviceName string) error {
	var devID, pubKey, certPEM, tlsFP string
	err := m.store.DB().QueryRow(`
		SELECT local_device_id, local_ed25519_pub, tls_cert_pem, tls_fingerprint
		FROM family_config WHERE id = 1
	`).Scan(&devID, &pubKey, &certPEM, &tlsFP)
	if err != nil {
		return err
	}

	if devID != "" && pubKey != "" && certPEM != "" && tlsFP != "" {
		return nil
	}

	if devID == "" {
		devID = "dev-" + uuid.NewString()[:8]
	}
	if deviceName == "" {
		if h, err := os.Hostname(); err == nil && h != "" {
			deviceName = h
		} else {
			deviceName = "FamilyControl-PC"
		}
	}

	pubB64, privBytes, err := security.GenerateEd25519KeyPair()
	if err != nil {
		return err
	}
	privEnc, err := m.seal.Encrypt(privBytes)
	if err != nil {
		return err
	}

	cPEM, kPEM, fp, err := security.GenerateSelfSignedTLS(deviceName, 10*365*24*time.Hour)
	if err != nil {
		return err
	}
	keyEnc, err := m.seal.Encrypt(kPEM)
	if err != nil {
		return err
	}

	_, err = m.store.DB().Exec(`
		UPDATE family_config
		SET local_device_id = ?, local_device_name = ?, local_ed25519_pub = ?,
		    local_ed25519_priv_enc = ?, tls_cert_pem = ?, tls_key_enc = ?,
		    tls_fingerprint = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, devID, deviceName, pubB64, privEnc, string(cPEM), keyEnc, fp)
	return err
}

// GetLocalTLSCredentials decrypts and returns the local TLS certificate and private key PEM bytes.
func (m *Manager) GetLocalTLSCredentials() (certPEM []byte, keyPEM []byte, fingerprint string, err error) {
	if err := m.EnsureCryptographicIdentity(""); err != nil {
		return nil, nil, "", err
	}
	var cStr, kEnc string
	err = m.store.DB().QueryRow(`SELECT tls_cert_pem, tls_key_enc, tls_fingerprint FROM family_config WHERE id = 1`).
		Scan(&cStr, &kEnc, &fingerprint)
	if err != nil {
		return nil, nil, "", err
	}
	kBytes, err := m.seal.Decrypt(kEnc)
	if err != nil {
		return nil, nil, "", err
	}
	return []byte(cStr), kBytes, fingerprint, nil
}

// GetLocalEd25519PrivateKey decrypts and returns the local Ed25519 private key.
func (m *Manager) GetLocalEd25519PrivateKey() (ed25519.PrivateKey, string, error) {
	if err := m.EnsureCryptographicIdentity(""); err != nil {
		return nil, "", err
	}
	var pubB64, privEnc string
	err := m.store.DB().QueryRow(`SELECT local_ed25519_pub, local_ed25519_priv_enc FROM family_config WHERE id = 1`).
		Scan(&pubB64, &privEnc)
	if err != nil {
		return nil, "", err
	}
	privBytes, err := m.seal.Decrypt(privEnc)
	if err != nil {
		return nil, "", err
	}
	return ed25519.PrivateKey(privBytes), pubB64, nil
}

// InitializeParentMode configures this computer as the Parent coordination endpoint.
func (m *Manager) InitializeParentMode(familyName, password, parentContact, firstChildName string) (SetupParentResult, error) {
	if BuildEdition == "child" {
		return SetupParentResult{}, errors.New("security block: this installer is Vylos Child Edition — Parent Mode cannot be activated on a Child Edition installation")
	}
	if existingCfg, err := m.GetConfig(); err == nil && existingCfg.SetupCompleted && existingCfg.Mode == ModeChild {
		return SetupParentResult{}, errors.New("security block: Vylos Child Mode is already active on this computer — Parent Edition cannot overwrite a protected Child PC")
	}
	familyName = strings.TrimSpace(familyName)
	if familyName == "" {
		familyName = "Family Household"
	}
	passHash, err := security.HashPassword(password)
	if err != nil {
		return SetupParentResult{}, err
	}
	recoveryPlain, err := security.GenerateRecoveryCode()
	if err != nil {
		return SetupParentResult{}, err
	}
	recoveryHash, err := security.HashPassword(security.NormalizeCode(recoveryPlain))
	if err != nil {
		return SetupParentResult{}, err
	}

	if err := m.EnsureCryptographicIdentity(""); err != nil {
		return SetupParentResult{}, err
	}

	err = m.store.WithTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			UPDATE family_config
			SET mode = 'parent',
			    family_name = ?,
			    parent_password_hash = ?,
			    recovery_code_hash = ?,
			    parent_contact = ?,
			    setup_completed = 1,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = 1
		`, familyName, passHash, recoveryHash, strings.TrimSpace(parentContact))
		if err != nil {
			return err
		}

		if strings.TrimSpace(firstChildName) != "" {
			childID := "child-" + uuid.NewString()[:8]
			_, err = tx.Exec(`
				INSERT INTO children (id, name, avatar_color, age_group)
				VALUES (?, ?, '#DCE9FF', '10-13')
			`, childID, strings.TrimSpace(firstChildName))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return SetupParentResult{}, err
	}

	sess, err := m.auth.CreateSession(security.RoleParent)
	if err != nil {
		return SetupParentResult{}, err
	}

	cfg, err := m.GetConfig()
	if err != nil {
		return SetupParentResult{}, err
	}

	return SetupParentResult{
		Config:       cfg,
		RecoveryCode: recoveryPlain,
		SessionToken: sess.Token,
	}, nil
}

// InitializeChildMode configures this computer to operate in Child Mode.
func (m *Manager) InitializeChildMode(deviceName, parentPassword, parentContact string) (Config, error) {
	if BuildEdition == "parent" {
		return Config{}, errors.New("this installation is Vylos Parent Edition — use Vylos-Child.msi on the child computer")
	}
	deviceName = strings.TrimSpace(deviceName)
	if deviceName == "" {
		if h, err := os.Hostname(); err == nil && h != "" {
			deviceName = h
		} else {
			deviceName = "Child-PC"
		}
	}
	passHash, err := security.HashPassword(parentPassword)
	if err != nil {
		return Config{}, err
	}
	if err := m.EnsureCryptographicIdentity(deviceName); err != nil {
		return Config{}, err
	}

	var devID string
	_ = m.store.DB().QueryRow(`SELECT local_device_id FROM family_config WHERE id = 1`).Scan(&devID)

	err = m.store.WithTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			UPDATE family_config
			SET mode = 'child',
			    local_device_name = ?,
			    parent_password_hash = ?,
			    parent_contact = ?,
			    setup_completed = 1,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = 1
		`, deviceName, passHash, strings.TrimSpace(parentContact))
		if err != nil {
			return err
		}

		// Ensure local screen-time and website policies exist for offline operation
		_, _ = tx.Exec(`INSERT OR IGNORE INTO screen_time_policies (device_id) VALUES (?)`, devID)
		_, _ = tx.Exec(`INSERT OR IGNORE INTO website_policies (device_id) VALUES (?)`, devID)
		// Block Vylos-Parent.exe from running on a Child Mode computer
		_, _ = tx.Exec(`
			INSERT OR IGNORE INTO app_rules (id, device_id, executable_name, display_name, status, daily_limit_minutes, allowed_hours)
			VALUES (?, ?, 'vylos-parent.exe', 'Vylos Parent Installer / Dashboard (Blocked on Child PC)', 'blocked', 0, '')
		`, "rule-block-parent-"+devID, devID)
		return nil
	})
	if err != nil {
		return Config{}, err
	}
	return m.GetConfig()
}

// AuthenticateParent verifies the parent password with rate-limiting and issues a session token.
func (m *Manager) AuthenticateParent(password string) (security.Session, error) {
	if !m.auth.AllowAttempt("local-parent-login") {
		return security.Session{}, errors.New("too many authentication attempts; please wait a few seconds")
	}
	var hash string
	err := m.store.DB().QueryRow(`SELECT parent_password_hash FROM family_config WHERE id = 1`).Scan(&hash)
	if err != nil || hash == "" {
		return security.Session{}, errors.New("parent authentication is not configured")
	}
	ok, err := security.VerifyPassword(password, hash)
	if err != nil || !ok {
		return security.Session{}, errors.New("invalid parent password")
	}
	return m.auth.CreateSession(security.RoleParent)
}

// VerifyParentPassword checks the parent password without creating a new session (e.g. for uninstall or rule unlock).
func (m *Manager) VerifyParentPassword(password string) bool {
	if !m.auth.AllowAttempt("local-verify") {
		return false
	}
	var hash string
	if err := m.store.DB().QueryRow(`SELECT parent_password_hash FROM family_config WHERE id = 1`).Scan(&hash); err != nil {
		return false
	}
	ok, _ := security.VerifyPassword(password, hash)
	return ok
}

// RecoverParentPassword resets the parent password using the emergency recovery key generated during setup.
func (m *Manager) RecoverParentPassword(recoveryCode, newPassword string) (string, error) {
	if !m.auth.AllowAttempt("local-recovery") {
		return "", errors.New("too many recovery attempts; please wait")
	}
	var recHash string
	if err := m.store.DB().QueryRow(`SELECT recovery_code_hash FROM family_config WHERE id = 1`).Scan(&recHash); err != nil || recHash == "" {
		return "", errors.New("no recovery code configured on this device")
	}

	normalized := security.NormalizeCode(recoveryCode)
	ok, err := security.VerifyPassword(normalized, recHash)
	if err != nil || !ok {
		return "", errors.New("invalid emergency recovery code")
	}

	newPassHash, err := security.HashPassword(newPassword)
	if err != nil {
		return "", err
	}
	newRecPlain, err := security.GenerateRecoveryCode()
	if err != nil {
		return "", err
	}
	newRecHash, err := security.HashPassword(security.NormalizeCode(newRecPlain))
	if err != nil {
		return "", err
	}

	_, err = m.store.DB().Exec(`
		UPDATE family_config
		SET parent_password_hash = ?, recovery_code_hash = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, newPassHash, newRecHash)
	if err != nil {
		return "", err
	}
	return newRecPlain, nil
}

// ChangeParentPassword updates the parent password after verifying the current password.
func (m *Manager) ChangeParentPassword(currentPassword, newPassword string) error {
	if !m.VerifyParentPassword(currentPassword) {
		return errors.New("current password is incorrect")
	}
	newHash, err := security.HashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = m.store.DB().Exec(`UPDATE family_config SET parent_password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1`, newHash)
	return err
}

// UpdateSettings updates general family settings.
func (m *Manager) UpdateSettings(familyName, parentContact string, retentionDays, listenPort int) (Config, error) {
	if retentionDays < 1 {
		retentionDays = 30
	}
	if listenPort < 1024 || listenPort > 65535 {
		listenPort = 41843
	}
	_, err := m.store.DB().Exec(`
		UPDATE family_config
		SET family_name = ?, parent_contact = ?, retention_days = ?, listen_port = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = 1
	`, strings.TrimSpace(familyName), strings.TrimSpace(parentContact), retentionDays, listenPort)
	if err != nil {
		return Config{}, err
	}
	return m.GetConfig()
}

// ListChildren returns all configured children with their paired device counts.
func (m *Manager) ListChildren() ([]ChildProfile, error) {
	rows, err := m.store.DB().Query(`
		SELECT c.id, c.name, c.avatar_color, c.age_group, c.created_at,
		       (SELECT COUNT(1) FROM devices d WHERE d.child_id = c.id AND d.status != 'revoked')
		FROM children c
		ORDER BY c.name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ChildProfile
	for rows.Next() {
		var cp ChildProfile
		var created time.Time
		if err := rows.Scan(&cp.ID, &cp.Name, &cp.AvatarColor, &cp.AgeGroup, &created, &cp.DeviceCount); err != nil {
			return nil, err
		}
		cp.CreatedAt = created.Format(time.RFC3339)
		result = append(result, cp)
	}
	if result == nil {
		result = []ChildProfile{}
	}
	return result, nil
}

// AddChild creates a new child profile.
func (m *Manager) AddChild(name, avatarColor, ageGroup string) (ChildProfile, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ChildProfile{}, errors.New("child name is required")
	}
	if avatarColor == "" {
		avatarColor = "#DCE9FF"
	}
	if ageGroup == "" {
		ageGroup = "10-13"
	}
	id := "child-" + uuid.NewString()[:8]
	now := time.Now().UTC()
	_, err := m.store.DB().Exec(`
		INSERT INTO children (id, name, avatar_color, age_group, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, name, avatarColor, ageGroup, now, now)
	if err != nil {
		return ChildProfile{}, fmt.Errorf("insert child: %w", err)
	}
	return ChildProfile{
		ID:          id,
		Name:        name,
		AvatarColor: avatarColor,
		AgeGroup:    ageGroup,
		DeviceCount: 0,
		CreatedAt:   now.Format(time.RFC3339),
	}, nil
}

// RemoveChild deletes a child profile and unassigns associated devices.
func (m *Manager) RemoveChild(childID string) error {
	return m.store.WithTx(func(tx *sql.Tx) error {
		_, _ = tx.Exec(`UPDATE devices SET child_id = '', child_name = '' WHERE child_id = ?`, childID)
		_, err := tx.Exec(`DELETE FROM children WHERE id = ?`, childID)
		return err
	})
}

// SwitchOperatingMode allows a legitimate parent (after password check) to switch between Parent and Child modes.
func (m *Manager) SwitchOperatingMode(targetMode OperatingMode) (Config, error) {
	if targetMode != ModeParent && targetMode != ModeChild {
		return Config{}, errors.New("invalid operating mode")
	}
	if BuildEdition == "child" && targetMode == ModeParent {
		return Config{}, errors.New("security block: Vylos Child Edition is locked to Child Mode")
	}
	if BuildEdition == "parent" && targetMode == ModeChild {
		return Config{}, errors.New("Vylos Parent Edition is locked to Parent Mode")
	}
	if err := m.EnsureCryptographicIdentity(""); err != nil {
		return Config{}, err
	}
	var devID string
	_ = m.store.DB().QueryRow(`SELECT local_device_id FROM family_config WHERE id = 1`).Scan(&devID)

	_, err := m.store.DB().Exec(`
		UPDATE family_config SET mode = ?, setup_completed = 1, updated_at = CURRENT_TIMESTAMP WHERE id = 1
	`, string(targetMode))
	if err != nil {
		return Config{}, err
	}
	if targetMode == ModeChild && devID != "" {
		_, _ = m.store.DB().Exec(`INSERT OR IGNORE INTO screen_time_policies (device_id) VALUES (?)`, devID)
		_, _ = m.store.DB().Exec(`INSERT OR IGNORE INTO website_policies (device_id) VALUES (?)`, devID)
	}
	return m.GetConfig()
}
