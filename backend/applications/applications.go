package applications

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"familycontrol/backend/database"
	"familycontrol/backend/windows"

	"github.com/google/uuid"
)

type AppRule struct {
	ID                string `json:"id"`
	DeviceID          string `json:"deviceId"`
	ExecutableName    string `json:"executableName"`
	DisplayName       string `json:"displayName"`
	Publisher         string `json:"publisher"`
	InstallPath       string `json:"installPath"`
	Status            string `json:"status"` // "allowed" | "blocked" | "scheduled"
	DailyLimitMinutes int    `json:"dailyLimitMinutes"`
	AllowedHours      string `json:"allowedHours"` // e.g. "15:00-19:00"
	IsNewlyInstalled  bool   `json:"isNewlyInstalled"`
	UsedTodayMinutes  int    `json:"usedTodayMinutes"`
	UpdatedAt         string `json:"updatedAt"`
}

type InstalledApp struct {
	ID             string `json:"id"`
	DeviceID       string `json:"deviceId"`
	ExecutableName string `json:"executableName"`
	DisplayName    string `json:"displayName"`
	Version        string `json:"version"`
	Publisher      string `json:"publisher"`
	InstallPath    string `json:"installPath"`
	FirstSeenAt    string `json:"firstSeenAt"`
	LastUsedAt     string `json:"lastUsedAt"`
	NewlyInstalled bool   `json:"newlyInstalled"`
	RuleStatus     string `json:"ruleStatus"`
}

type Manager struct {
	store *database.Store
}

func NewManager(store *database.Store) *Manager {
	return &Manager{store: store}
}

// RefreshInstalledApps scans the local Windows Registry for installed applications and updates SQLite.
func (m *Manager) RefreshInstalledApps(deviceID string) ([]InstalledApp, error) {
	if deviceID == "" {
		deviceID = "local"
	}
	scanned, err := windows.ScanInstalledApplications()
	if err != nil {
		return nil, err
	}

	var existingCount int
	_ = m.store.DB().QueryRow(`SELECT COUNT(1) FROM installed_apps WHERE device_id = ?`, deviceID).Scan(&existingCount)
	isInitialScan := existingCount == 0

	now := time.Now().UTC()
	_ = m.store.WithTx(func(tx *sql.Tx) error {
		for _, app := range scanned {
			exeLower := strings.ToLower(strings.TrimSpace(app.ExecutableName))
			if exeLower == "" || windows.IsSystemCriticalProcess(exeLower) {
				continue
			}
			newFlag := 0
			if !isInitialScan {
				var c int
				_ = tx.QueryRow(`SELECT COUNT(1) FROM installed_apps WHERE device_id = ? AND executable_name = ?`, deviceID, exeLower).Scan(&c)
				if c == 0 {
					newFlag = 1
				}
			}
			id := "app-" + uuid.NewString()[:8]
			_, _ = tx.Exec(`
				INSERT INTO installed_apps (id, device_id, executable_name, display_name, version, publisher, install_path, first_seen_at, last_used_at, newly_installed)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT(device_id, executable_name) DO UPDATE SET
					display_name = excluded.display_name,
					version = excluded.version,
					publisher = excluded.publisher,
					install_path = excluded.install_path
			`, id, deviceID, exeLower, app.DisplayName, app.Version, app.Publisher, app.InstallPath, now, now, newFlag)
		}
		return nil
	})

	return m.ListInstalledApps(deviceID)
}

// ListInstalledApps returns all discovered applications for a device, merged with their current rule status.
func (m *Manager) ListInstalledApps(deviceID string) ([]InstalledApp, error) {
	if deviceID == "" {
		deviceID = "local"
	}
	rows, err := m.store.DB().Query(`
		SELECT ia.id, ia.device_id, ia.executable_name, ia.display_name, ia.version,
		       ia.publisher, ia.install_path, ia.first_seen_at, ia.last_used_at, ia.newly_installed,
		       COALESCE(ar.status, 'allowed')
		FROM installed_apps ia
		LEFT JOIN app_rules ar ON ar.device_id = ia.device_id AND ar.executable_name = ia.executable_name
		WHERE ia.device_id = ?
		ORDER BY ia.newly_installed DESC, ia.display_name ASC
	`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []InstalledApp
	for rows.Next() {
		var item InstalledApp
		var firstSeen, lastUsed time.Time
		var newInt int
		if err := rows.Scan(
			&item.ID, &item.DeviceID, &item.ExecutableName, &item.DisplayName, &item.Version,
			&item.Publisher, &item.InstallPath, &firstSeen, &lastUsed, &newInt, &item.RuleStatus,
		); err != nil {
			continue
		}
		item.FirstSeenAt = firstSeen.Format(time.RFC3339)
		item.LastUsedAt = lastUsed.Format(time.RFC3339)
		item.NewlyInstalled = newInt == 1
		list = append(list, item)
	}
	if list == nil {
		list = []InstalledApp{}
	}
	return list, nil
}

// ListRules returns all configured application rules for a device.
func (m *Manager) ListRules(deviceID string) ([]AppRule, error) {
	if deviceID == "" {
		deviceID = "local"
	}
	dateStr := time.Now().UTC().Format("2006-01-02")

	rows, err := m.store.DB().Query(`
		SELECT ar.id, ar.device_id, ar.executable_name, ar.display_name, ar.publisher,
		       ar.install_path, ar.status, ar.daily_limit_minutes, ar.allowed_hours_json,
		       ar.is_newly_installed, ar.updated_at,
		       COALESCE(au.total_seconds, 0)
		FROM app_rules ar
		LEFT JOIN app_usage_daily au
		  ON au.device_id = ar.device_id AND au.executable_name = ar.executable_name AND au.date_str = ?
		WHERE ar.device_id = ?
		ORDER BY ar.status DESC, ar.display_name ASC
	`, dateStr, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []AppRule
	for rows.Next() {
		var r AppRule
		var newInt, usedSec int
		var updated time.Time
		if err := rows.Scan(
			&r.ID, &r.DeviceID, &r.ExecutableName, &r.DisplayName, &r.Publisher,
			&r.InstallPath, &r.Status, &r.DailyLimitMinutes, &r.AllowedHours,
			&newInt, &updated, &usedSec,
		); err != nil {
			continue
		}
		r.IsNewlyInstalled = newInt == 1
		r.UsedTodayMinutes = usedSec / 60
		r.UpdatedAt = updated.Format(time.RFC3339)
		rules = append(rules, r)
	}
	if rules == nil {
		rules = []AppRule{}
	}
	return rules, nil
}

// SaveRule creates or updates an application rule. Refuses to block system-critical processes.
func (m *Manager) SaveRule(rule AppRule) (AppRule, error) {
	exeLower := strings.ToLower(strings.TrimSpace(filepath.Base(rule.ExecutableName)))
	if exeLower == "" {
		return AppRule{}, errors.New("executable name is required")
	}
	if !strings.HasSuffix(exeLower, ".exe") {
		exeLower += ".exe"
	}
	if rule.Status != "allowed" && windows.IsSystemCriticalProcess(exeLower) {
		return AppRule{}, fmt.Errorf("cannot restrict Windows system-critical process: %s", exeLower)
	}
	if rule.DeviceID == "" {
		rule.DeviceID = "local"
	}
	if rule.DisplayName == "" {
		rule.DisplayName = exeLower
	}
	if rule.Status != "allowed" && rule.Status != "blocked" && rule.Status != "scheduled" {
		rule.Status = "allowed"
	}
	if rule.ID == "" {
		rule.ID = "rule-" + uuid.NewString()[:8]
	}
	newInt := 0
	if rule.IsNewlyInstalled {
		newInt = 1
	}

	_, err := m.store.DB().Exec(`
		INSERT INTO app_rules (
			id, device_id, executable_name, display_name, publisher, install_path,
			status, daily_limit_minutes, allowed_hours_json, is_newly_installed, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(device_id, executable_name) DO UPDATE SET
			display_name = excluded.display_name,
			status = excluded.status,
			daily_limit_minutes = excluded.daily_limit_minutes,
			allowed_hours_json = excluded.allowed_hours_json,
			is_newly_installed = excluded.is_newly_installed,
			updated_at = CURRENT_TIMESTAMP
	`, rule.ID, rule.DeviceID, exeLower, rule.DisplayName, rule.Publisher, rule.InstallPath,
		rule.Status, rule.DailyLimitMinutes, strings.TrimSpace(rule.AllowedHours), newInt)
	if err != nil {
		return AppRule{}, err
	}

	// Acknowledge newly installed flag when parent sets a rule
	_, _ = m.store.DB().Exec(`
		UPDATE installed_apps SET newly_installed = 0 WHERE device_id = ? AND executable_name = ?
	`, rule.DeviceID, exeLower)

	rule.ExecutableName = exeLower
	rule.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return rule, nil
}

// DeleteRule removes a custom application rule.
func (m *Manager) DeleteRule(deviceID, executableName string) error {
	exeLower := strings.ToLower(strings.TrimSpace(executableName))
	_, err := m.store.DB().Exec(`DELETE FROM app_rules WHERE device_id = ? AND executable_name = ?`, deviceID, exeLower)
	return err
}

// AcknowledgeNewApp clears the "newly installed" badge on an application.
func (m *Manager) AcknowledgeNewApp(deviceID, executableName string) error {
	exeLower := strings.ToLower(strings.TrimSpace(executableName))
	_, err := m.store.DB().Exec(`
		UPDATE installed_apps SET newly_installed = 0 WHERE device_id = ? AND executable_name = ?
	`, deviceID, exeLower)
	return err
}

// EvaluateExecutable checks whether a running process is permitted at the given time.
func (m *Manager) EvaluateExecutable(deviceID, exeName string, now time.Time) (allowed bool, reason string) {
	exeLower := strings.ToLower(strings.TrimSpace(filepath.Base(exeName)))
	if windows.IsSystemCriticalProcess(exeLower) {
		return true, ""
	}

	var status, allowedHours string
	var dailyLimitMin int
	err := m.store.DB().QueryRow(`
		SELECT status, daily_limit_minutes, allowed_hours_json
		FROM app_rules
		WHERE device_id = ? AND executable_name = ?
	`, deviceID, exeLower).Scan(&status, &dailyLimitMin, &allowedHours)
	if err != nil {
		return true, ""
	}

	if status == "blocked" {
		return false, "Application blocked by parent policy"
	}

	if status == "scheduled" || dailyLimitMin > 0 {
		if allowedHours != "" {
			if !isWithinHoursRange(now, allowedHours) {
				return false, fmt.Sprintf("Outside allowed application schedule (%s)", allowedHours)
			}
		}
		if dailyLimitMin > 0 {
			dateStr := now.UTC().Format("2006-01-02")
			var usedSec int
			_ = m.store.DB().QueryRow(`
				SELECT COALESCE(total_seconds, 0) FROM app_usage_daily
				WHERE device_id = ? AND date_str = ? AND executable_name = ?
			`, deviceID, dateStr, exeLower).Scan(&usedSec)
			if usedSec/60 >= dailyLimitMin {
				return false, fmt.Sprintf("Application daily limit (%d min) reached", dailyLimitMin)
			}
		}
	}

	return true, ""
}

// EnforceRunningApplications inspects running processes on the child machine and safely terminates blocked non-system processes.
func (m *Manager) EnforceRunningApplications(deviceID string, now time.Time) ([]string, error) {
	procs, err := windows.ListRunningProcesses()
	if err != nil {
		return nil, err
	}
	var terminated []string
	for _, p := range procs {
		if windows.IsSystemCriticalProcess(p.ExecutableName) {
			continue
		}
		allowed, _ := m.EvaluateExecutable(deviceID, p.ExecutableName, now)
		if !allowed {
			if err := windows.TerminateAllowedProcess(p.PID, p.ExecutableName); err == nil {
				terminated = append(terminated, p.ExecutableName)
			}
		}
	}
	return terminated, nil
}

func isWithinHoursRange(now time.Time, rangeSpec string) bool {
	parts := strings.Split(strings.TrimSpace(rangeSpec), "-")
	if len(parts) != 2 {
		return true
	}
	var sh, sm, eh, em int
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[0]), "%d:%d", &sh, &sm); err != nil {
		return true
	}
	if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%d:%d", &eh, &em); err != nil {
		return true
	}
	cur := now.Hour()*60 + now.Minute()
	start := sh*60 + sm
	end := eh*60 + em
	if start <= end {
		return cur >= start && cur <= end
	}
	return cur >= start || cur <= end
}
