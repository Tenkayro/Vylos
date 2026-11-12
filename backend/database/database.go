package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database connection and provides migration, transaction, and retention helpers.
type Store struct {
	db     *sql.DB
	dbPath string
	mu     sync.RWMutex
}

type Migration struct {
	Version int
	Name    string
	SQL     string
}

var schemaMigrations = []Migration{
	{
		Version: 1,
		Name:    "initial_core_schema",
		SQL: `
CREATE TABLE IF NOT EXISTS family_config (
	id INTEGER PRIMARY KEY CHECK (id = 1),
	mode TEXT NOT NULL DEFAULT 'unconfigured',
	family_name TEXT NOT NULL DEFAULT 'My Family',
	parent_password_hash TEXT NOT NULL DEFAULT '',
	recovery_code_hash TEXT NOT NULL DEFAULT '',
	parent_contact TEXT NOT NULL DEFAULT '',
	local_device_id TEXT NOT NULL DEFAULT '',
	local_device_name TEXT NOT NULL DEFAULT '',
	local_child_id TEXT NOT NULL DEFAULT '',
	local_ed25519_pub TEXT NOT NULL DEFAULT '',
	local_ed25519_priv_enc TEXT NOT NULL DEFAULT '',
	tls_cert_pem TEXT NOT NULL DEFAULT '',
	tls_key_enc TEXT NOT NULL DEFAULT '',
	tls_fingerprint TEXT NOT NULL DEFAULT '',
	listen_port INTEGER NOT NULL DEFAULT 41843,
	discovery_port INTEGER NOT NULL DEFAULT 41880,
	parent_endpoint TEXT NOT NULL DEFAULT '',
	parent_cert_fingerprint TEXT NOT NULL DEFAULT '',
	parent_ed25519_pub TEXT NOT NULL DEFAULT '',
	device_token_enc TEXT NOT NULL DEFAULT '',
	retention_days INTEGER NOT NULL DEFAULT 30,
	setup_completed INTEGER NOT NULL DEFAULT 0,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT OR IGNORE INTO family_config (id, mode, family_name, listen_port, discovery_port, retention_days, setup_completed)
VALUES (1, 'unconfigured', 'My Family', 41843, 41880, 30, 0);

CREATE TABLE IF NOT EXISTS children (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	avatar_color TEXT NOT NULL DEFAULT '#DCE9FF',
	age_group TEXT NOT NULL DEFAULT '10-13',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS devices (
	id TEXT PRIMARY KEY,
	child_id TEXT NOT NULL DEFAULT '',
	child_name TEXT NOT NULL DEFAULT '',
	name TEXT NOT NULL,
	hostname TEXT NOT NULL DEFAULT '',
	os_version TEXT NOT NULL DEFAULT 'Windows 11',
	ip_address TEXT NOT NULL DEFAULT '',
	port INTEGER NOT NULL DEFAULT 41844,
	cert_fingerprint TEXT NOT NULL DEFAULT '',
	ed25519_pub_key TEXT NOT NULL DEFAULT '',
	auth_token_hash TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'offline',
	screen_locked INTEGER NOT NULL DEFAULT 0,
	manual_pause INTEGER NOT NULL DEFAULT 0,
	remaining_minutes INTEGER NOT NULL DEFAULT 120,
	used_today_minutes INTEGER NOT NULL DEFAULT 0,
	active_app TEXT NOT NULL DEFAULT '',
	dns_protected INTEGER NOT NULL DEFAULT 1,
	agent_version TEXT NOT NULL DEFAULT '1.0.0',
	last_seen DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	paired_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pairing_codes (
	code_hash TEXT PRIMARY KEY,
	code_display TEXT NOT NULL,
	child_id TEXT NOT NULL DEFAULT '',
	child_name TEXT NOT NULL DEFAULT '',
	device_name_hint TEXT NOT NULL DEFAULT '',
	expires_at DATETIME NOT NULL,
	used INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS screen_time_policies (
	device_id TEXT PRIMARY KEY,
	daily_limit_minutes INTEGER NOT NULL DEFAULT 120,
	weekend_limit_minutes INTEGER NOT NULL DEFAULT 180,
	bedtime_start TEXT NOT NULL DEFAULT '21:00',
	bedtime_end TEXT NOT NULL DEFAULT '07:00',
	weekly_schedule_json TEXT NOT NULL DEFAULT '{}',
	break_interval_minutes INTEGER NOT NULL DEFAULT 45,
	break_duration_minutes INTEGER NOT NULL DEFAULT 10,
	manual_pause INTEGER NOT NULL DEFAULT 0,
	screen_locked INTEGER NOT NULL DEFAULT 0,
	bonus_minutes_today INTEGER NOT NULL DEFAULT 0,
	bonus_date TEXT NOT NULL DEFAULT '',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS app_rules (
	id TEXT PRIMARY KEY,
	device_id TEXT NOT NULL,
	executable_name TEXT NOT NULL,
	display_name TEXT NOT NULL,
	publisher TEXT NOT NULL DEFAULT '',
	install_path TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL DEFAULT 'allowed',
	daily_limit_minutes INTEGER NOT NULL DEFAULT 0,
	allowed_hours_json TEXT NOT NULL DEFAULT '',
	is_newly_installed INTEGER NOT NULL DEFAULT 0,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(device_id, executable_name)
);

CREATE TABLE IF NOT EXISTS installed_apps (
	id TEXT PRIMARY KEY,
	device_id TEXT NOT NULL,
	executable_name TEXT NOT NULL,
	display_name TEXT NOT NULL,
	version TEXT NOT NULL DEFAULT '',
	publisher TEXT NOT NULL DEFAULT '',
	install_path TEXT NOT NULL DEFAULT '',
	first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_used_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	newly_installed INTEGER NOT NULL DEFAULT 0,
	UNIQUE(device_id, executable_name)
);

CREATE TABLE IF NOT EXISTS website_policies (
	device_id TEXT PRIMARY KEY,
	dns_mode TEXT NOT NULL DEFAULT 'cloudflare_family',
	block_adult INTEGER NOT NULL DEFAULT 1,
	block_malware INTEGER NOT NULL DEFAULT 1,
	primary_ipv4 TEXT NOT NULL DEFAULT '1.1.1.3',
	secondary_ipv4 TEXT NOT NULL DEFAULT '1.0.0.3',
	primary_ipv6 TEXT NOT NULL DEFAULT '2606:4700:4700::1113',
	secondary_ipv6 TEXT NOT NULL DEFAULT '2606:4700:4700::1003',
	allowlist_json TEXT NOT NULL DEFAULT '[]',
	blocklist_json TEXT NOT NULL DEFAULT '[]',
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS blocked_domain_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	device_id TEXT NOT NULL,
	domain TEXT NOT NULL,
	category TEXT NOT NULL DEFAULT 'custom_blocklist',
	reason TEXT NOT NULL DEFAULT 'Blocked by family rule',
	occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS activity_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	device_id TEXT NOT NULL,
	event_type TEXT NOT NULL,
	app_name TEXT NOT NULL DEFAULT '',
	executable_name TEXT NOT NULL DEFAULT '',
	window_title TEXT NOT NULL DEFAULT '',
	duration_seconds INTEGER NOT NULL DEFAULT 0,
	occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	synced INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_activity_device_time ON activity_events(device_id, occurred_at DESC);

CREATE TABLE IF NOT EXISTS daily_usage (
	device_id TEXT NOT NULL,
	date_str TEXT NOT NULL,
	total_seconds INTEGER NOT NULL DEFAULT 0,
	blocked_attempts INTEGER NOT NULL DEFAULT 0,
	login_count INTEGER NOT NULL DEFAULT 0,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (device_id, date_str)
);

CREATE TABLE IF NOT EXISTS app_usage_daily (
	device_id TEXT NOT NULL,
	date_str TEXT NOT NULL,
	executable_name TEXT NOT NULL,
	display_name TEXT NOT NULL,
	category TEXT NOT NULL DEFAULT 'General',
	total_seconds INTEGER NOT NULL DEFAULT 0,
	launch_count INTEGER NOT NULL DEFAULT 1,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (device_id, date_str, executable_name)
);

CREATE TABLE IF NOT EXISTS security_events (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	device_id TEXT NOT NULL DEFAULT 'local',
	severity TEXT NOT NULL DEFAULT 'info',
	category TEXT NOT NULL,
	message TEXT NOT NULL,
	details TEXT NOT NULL DEFAULT '',
	occurred_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	synced INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_security_events_time ON security_events(occurred_at DESC);

CREATE TABLE IF NOT EXISTS sync_queue (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	payload_type TEXT NOT NULL,
	payload_json TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	retry_count INTEGER NOT NULL DEFAULT 0
);
`,
	},
}

// Open initializes the SQLite database at dbPath and executes all pending migrations.
func Open(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		return nil, fmt.Errorf("create database dir: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", filepath.ToSlash(dbPath))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{
		db:     db,
		dbPath: dbPath,
	}

	if err := s.runMigrations(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return s, nil
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// WithTx executes fn inside a serialized SQL transaction.
func (s *Store) WithTx(fn func(tx *sql.Tx) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) runMigrations() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`)
	if err != nil {
		return err
	}

	for _, m := range schemaMigrations {
		var count int
		if err := s.db.QueryRow(`SELECT COUNT(1) FROM schema_migrations WHERE version = ?`, m.Version).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		err := s.WithTx(func(tx *sql.Tx) error {
			if _, err := tx.Exec(m.SQL); err != nil {
				return fmt.Errorf("migration %d (%s): %w", m.Version, m.Name, err)
			}
			_, err := tx.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`,
				m.Version, m.Name, time.Now().UTC())
			return err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// PruneOldData deletes historical activity, blocked domain events, and security events older than retentionDays.
func (s *Store) PruneOldData(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	cutoffDateStr := cutoff.Format("2006-01-02")

	var totalDeleted int64
	err := s.WithTx(func(tx *sql.Tx) error {
		queries := []struct {
			q   string
			arg any
		}{
			{`DELETE FROM activity_events WHERE occurred_at < ?`, cutoff},
			{`DELETE FROM blocked_domain_events WHERE occurred_at < ?`, cutoff},
			{`DELETE FROM security_events WHERE occurred_at < ?`, cutoff},
			{`DELETE FROM daily_usage WHERE date_str < ?`, cutoffDateStr},
			{`DELETE FROM app_usage_daily WHERE date_str < ?`, cutoffDateStr},
			{`DELETE FROM pairing_codes WHERE expires_at < ?`, time.Now().UTC()},
		}
		for _, item := range queries {
			res, err := tx.Exec(item.q, item.arg)
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			totalDeleted += n
		}
		return nil
	})
	return totalDeleted, err
}

// BackupSnapshot represents a portable configuration backup of children, devices, screen-time policies, app rules, and website policies.
type BackupSnapshot struct {
	Version         int              `json:"version"`
	ExportedAt      time.Time        `json:"exportedAt"`
	FamilyName      string           `json:"familyName"`
	ParentContact   string           `json:"parentContact"`
	RetentionDays   int              `json:"retentionDays"`
	Children        []map[string]any `json:"children"`
	ScreenPolicies  []map[string]any `json:"screenPolicies"`
	AppRules        []map[string]any `json:"appRules"`
	WebsitePolicies []map[string]any `json:"websitePolicies"`
}

// ExportBackup serializes non-secret family policies into JSON bytes for encrypted export.
func (s *Store) ExportBackup() ([]byte, error) {
	snap := BackupSnapshot{
		Version:    1,
		ExportedAt: time.Now().UTC(),
	}

	_ = s.db.QueryRow(`SELECT family_name, parent_contact, retention_days FROM family_config WHERE id = 1`).
		Scan(&snap.FamilyName, &snap.ParentContact, &snap.RetentionDays)

	rows, err := s.db.Query(`SELECT id, name, avatar_color, age_group FROM children`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, name, color, age string
			if err := rows.Scan(&id, &name, &color, &age); err == nil {
				snap.Children = append(snap.Children, map[string]any{
					"id": id, "name": name, "avatarColor": color, "ageGroup": age,
				})
			}
		}
	}

	stRows, err := s.db.Query(`SELECT device_id, daily_limit_minutes, weekend_limit_minutes, bedtime_start, bedtime_end, weekly_schedule_json, break_interval_minutes, break_duration_minutes FROM screen_time_policies`)
	if err == nil {
		defer stRows.Close()
		for stRows.Next() {
			var devID, bStart, bEnd, sched string
			var daily, weekend, bInt, bDur int
			if err := stRows.Scan(&devID, &daily, &weekend, &bStart, &bEnd, &sched, &bInt, &bDur); err == nil {
				snap.ScreenPolicies = append(snap.ScreenPolicies, map[string]any{
					"deviceId": devID, "dailyLimitMinutes": daily, "weekendLimitMinutes": weekend,
					"bedtimeStart": bStart, "bedtimeEnd": bEnd, "weeklyScheduleJson": sched,
					"breakIntervalMinutes": bInt, "breakDurationMinutes": bDur,
				})
			}
		}
	}

	wRows, err := s.db.Query(`SELECT device_id, dns_mode, block_adult, block_malware, primary_ipv4, secondary_ipv4, primary_ipv6, secondary_ipv6, allowlist_json, blocklist_json FROM website_policies`)
	if err == nil {
		defer wRows.Close()
		for wRows.Next() {
			var devID, mode, p4, s4, p6, s6, allow, block string
			var adult, malware int
			if err := wRows.Scan(&devID, &mode, &adult, &malware, &p4, &s4, &p6, &s6, &allow, &block); err == nil {
				snap.WebsitePolicies = append(snap.WebsitePolicies, map[string]any{
					"deviceId": devID, "dnsMode": mode, "blockAdult": adult, "blockMalware": malware,
					"primaryIpv4": p4, "secondaryIpv4": s4, "primaryIpv6": p6, "secondaryIpv6": s6,
					"allowlistJson": allow, "blocklistJson": block,
				})
			}
		}
	}

	return json.MarshalIndent(snap, "", "  ")
}

// ImportBackup restores family configuration and policies from a backup JSON payload.
func (s *Store) ImportBackup(raw []byte) error {
	var snap BackupSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return fmt.Errorf("invalid backup payload: %w", err)
	}
	if snap.Version != 1 {
		return fmt.Errorf("unsupported backup version: %d", snap.Version)
	}

	return s.WithTx(func(tx *sql.Tx) error {
		if snap.FamilyName != "" {
			_, err := tx.Exec(`UPDATE family_config SET family_name = ?, parent_contact = ?, retention_days = ?, updated_at = CURRENT_TIMESTAMP WHERE id = 1`,
				snap.FamilyName, snap.ParentContact, snap.RetentionDays)
			if err != nil {
				return err
			}
		}
		for _, c := range snap.Children {
			id, _ := c["id"].(string)
			name, _ := c["name"].(string)
			color, _ := c["avatarColor"].(string)
			age, _ := c["ageGroup"].(string)
			if id != "" && name != "" {
				_, _ = tx.Exec(`INSERT INTO children (id, name, avatar_color, age_group) VALUES (?, ?, ?, ?)
					ON CONFLICT(id) DO UPDATE SET name = excluded.name, avatar_color = excluded.avatar_color, age_group = excluded.age_group`,
					id, name, color, age)
			}
		}
		return nil
	})
}
