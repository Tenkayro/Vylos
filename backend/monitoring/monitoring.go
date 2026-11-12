package monitoring

import (
	"database/sql"
	"path/filepath"
	"strings"
	"time"

	"familycontrol/backend/database"
)

type ActivityEvent struct {
	ID              int64  `json:"id"`
	DeviceID        string `json:"deviceId"`
	EventType       string `json:"eventType"`
	AppName         string `json:"appName"`
	ExecutableName  string `json:"executableName"`
	WindowTitle     string `json:"windowTitle"`
	DurationSeconds int    `json:"durationSeconds"`
	OccurredAt      string `json:"occurredAt"`
	Synced          bool   `json:"synced"`
}

type DailySummary struct {
	DeviceID        string          `json:"deviceId"`
	DateStr         string          `json:"dateStr"`
	TotalSeconds    int             `json:"totalSeconds"`
	TotalMinutes    int             `json:"totalMinutes"`
	BlockedAttempts int             `json:"blockedAttempts"`
	LoginCount      int             `json:"loginCount"`
	TopApps         []AppUsageEntry `json:"topApps"`
}

type AppUsageEntry struct {
	DeviceID       string `json:"deviceId"`
	DateStr        string `json:"dateStr"`
	ExecutableName string `json:"executableName"`
	DisplayName    string `json:"displayName"`
	Category       string `json:"category"`
	TotalSeconds   int    `json:"totalSeconds"`
	TotalMinutes   int    `json:"totalMinutes"`
	LaunchCount    int    `json:"launchCount"`
	UpdatedAt      string `json:"updatedAt"`
}

type Manager struct {
	store *database.Store
}

func NewManager(store *database.Store) *Manager {
	return &Manager{store: store}
}

// RecordSessionEvent records a computer login, logout, lock, or break reminder event.
func (m *Manager) RecordSessionEvent(deviceID, eventType, details string) error {
	now := time.Now().UTC()
	dateStr := now.Format("2006-01-02")

	return m.store.WithTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO activity_events (device_id, event_type, app_name, executable_name, window_title, duration_seconds, occurred_at, synced)
			VALUES (?, ?, 'Windows Session', 'explorer.exe', ?, 0, ?, 0)
		`, deviceID, eventType, details, now)
		if err != nil {
			return err
		}

		loginInc := 0
		if eventType == "login" {
			loginInc = 1
		}
		_, err = tx.Exec(`
			INSERT INTO daily_usage (device_id, date_str, total_seconds, blocked_attempts, login_count, updated_at)
			VALUES (?, ?, 0, 0, ?, ?)
			ON CONFLICT(device_id, date_str) DO UPDATE SET
				login_count = daily_usage.login_count + ?,
				updated_at = excluded.updated_at
		`, deviceID, dateStr, loginInc, now, loginInc)
		return err
	})
}

// RecordAppUsage Tick records incremental active duration (in seconds) for the foreground application.
// Transparent monitoring: tracks only application executable name, friendly name, and active duration.
func (m *Manager) RecordAppUsage(deviceID, exeName, displayName, windowTitle string, deltaSeconds int) error {
	if deltaSeconds <= 0 || deviceID == "" {
		return nil
	}
	exeName = strings.TrimSpace(filepath.Base(exeName))
	if exeName == "" {
		exeName = "System"
	}
	if displayName == "" {
		displayName = FriendlyAppName(exeName)
	}
	category := CategorizeApp(exeName)
	now := time.Now().UTC()
	dateStr := now.Format("2006-01-02")

	return m.store.WithTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`
			INSERT INTO daily_usage (device_id, date_str, total_seconds, blocked_attempts, login_count, updated_at)
			VALUES (?, ?, ?, 0, 1, ?)
			ON CONFLICT(device_id, date_str) DO UPDATE SET
				total_seconds = daily_usage.total_seconds + excluded.total_seconds,
				updated_at = excluded.updated_at
		`, deviceID, dateStr, deltaSeconds, now)
		if err != nil {
			return err
		}

		_, err = tx.Exec(`
			INSERT INTO app_usage_daily (device_id, date_str, executable_name, display_name, category, total_seconds, launch_count, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 1, ?)
			ON CONFLICT(device_id, date_str, executable_name) DO UPDATE SET
				total_seconds = app_usage_daily.total_seconds + excluded.total_seconds,
				display_name = excluded.display_name,
				updated_at = excluded.updated_at
		`, deviceID, dateStr, strings.ToLower(exeName), displayName, category, deltaSeconds, now)
		if err != nil {
			return err
		}

		// Coalesce recent focus event if same app was active within last 5 minutes
		var lastID int64
		var lastExe string
		err = tx.QueryRow(`
			SELECT id, executable_name FROM activity_events
			WHERE device_id = ? AND event_type = 'app_focus' AND occurred_at >= ?
			ORDER BY id DESC LIMIT 1
		`, deviceID, now.Add(-5*time.Minute)).Scan(&lastID, &lastExe)
		if err == nil && strings.EqualFold(lastExe, exeName) {
			_, err = tx.Exec(`
				UPDATE activity_events
				SET duration_seconds = duration_seconds + ?, window_title = ?, synced = 0
				WHERE id = ?
			`, deltaSeconds, sanitizeWindowTitle(windowTitle), lastID)
			return err
		}

		_, err = tx.Exec(`
			INSERT INTO activity_events (device_id, event_type, app_name, executable_name, window_title, duration_seconds, occurred_at, synced)
			VALUES (?, 'app_focus', ?, ?, ?, ?, ?, 0)
		`, deviceID, displayName, strings.ToLower(exeName), sanitizeWindowTitle(windowTitle), deltaSeconds, now)
		return err
	})
}

// RecordBlockedAttempt logs when a blocked application or domain rule is triggered.
func (m *Manager) RecordBlockedAttempt(deviceID, exeName, displayName, reason string) error {
	now := time.Now().UTC()
	dateStr := now.Format("2006-01-02")

	return m.store.WithTx(func(tx *sql.Tx) error {
		_, _ = tx.Exec(`
			INSERT INTO daily_usage (device_id, date_str, total_seconds, blocked_attempts, login_count, updated_at)
			VALUES (?, ?, 0, 1, 1, ?)
			ON CONFLICT(device_id, date_str) DO UPDATE SET
				blocked_attempts = daily_usage.blocked_attempts + 1,
				updated_at = excluded.updated_at
		`, deviceID, dateStr, now)

		_, err := tx.Exec(`
			INSERT INTO activity_events (device_id, event_type, app_name, executable_name, window_title, duration_seconds, occurred_at, synced)
			VALUES (?, 'app_blocked', ?, ?, ?, 0, ?, 0)
		`, deviceID, displayName, strings.ToLower(exeName), reason, now)
		return err
	})
}

// GetDailySummary returns today's (or a specified date's) usage summary and top applications for a device.
func (m *Manager) GetDailySummary(deviceID, dateStr string) (DailySummary, error) {
	if dateStr == "" {
		dateStr = time.Now().UTC().Format("2006-01-02")
	}
	summary := DailySummary{
		DeviceID: deviceID,
		DateStr:  dateStr,
		TopApps:  []AppUsageEntry{},
	}

	if deviceID == "" || deviceID == "all" {
		_ = m.store.DB().QueryRow(`
			SELECT COALESCE(SUM(total_seconds), 0), COALESCE(SUM(blocked_attempts), 0), COALESCE(SUM(login_count), 0)
			FROM daily_usage WHERE date_str = ?
		`, dateStr).Scan(&summary.TotalSeconds, &summary.BlockedAttempts, &summary.LoginCount)
	} else {
		_ = m.store.DB().QueryRow(`
			SELECT total_seconds, blocked_attempts, login_count
			FROM daily_usage WHERE device_id = ? AND date_str = ?
		`, deviceID, dateStr).Scan(&summary.TotalSeconds, &summary.BlockedAttempts, &summary.LoginCount)
	}
	summary.TotalMinutes = summary.TotalSeconds / 60

	apps, err := m.ListAppUsage(deviceID, dateStr)
	if err == nil {
		summary.TopApps = apps
	}
	return summary, nil
}

// ListAppUsage returns per-application usage for a device and date.
func (m *Manager) ListAppUsage(deviceID, dateStr string) ([]AppUsageEntry, error) {
	if dateStr == "" {
		dateStr = time.Now().UTC().Format("2006-01-02")
	}

	var rows *sql.Rows
	var err error
	if deviceID == "" || deviceID == "all" {
		rows, err = m.store.DB().Query(`
			SELECT device_id, date_str, executable_name, display_name, category,
			       SUM(total_seconds) as total_sec, SUM(launch_count) as launches, MAX(updated_at)
			FROM app_usage_daily
			WHERE date_str = ?
			GROUP BY executable_name
			ORDER BY total_sec DESC
			LIMIT 30
		`, dateStr)
	} else {
		rows, err = m.store.DB().Query(`
			SELECT device_id, date_str, executable_name, display_name, category,
			       total_seconds, launch_count, updated_at
			FROM app_usage_daily
			WHERE device_id = ? AND date_str = ?
			ORDER BY total_seconds DESC
			LIMIT 30
		`, deviceID, dateStr)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []AppUsageEntry
	for rows.Next() {
		var item AppUsageEntry
		var updated any
		if err := rows.Scan(&item.DeviceID, &item.DateStr, &item.ExecutableName, &item.DisplayName,
			&item.Category, &item.TotalSeconds, &item.LaunchCount, &updated); err != nil {
			continue
		}
		item.TotalMinutes = item.TotalSeconds / 60
		if item.TotalMinutes == 0 && item.TotalSeconds > 0 {
			item.TotalMinutes = 1
		}
		list = append(list, item)
	}
	if list == nil {
		list = []AppUsageEntry{}
	}
	return list, nil
}

// ListHistoricalDailyUsage returns the last N days of daily usage summaries.
func (m *Manager) ListHistoricalDailyUsage(deviceID string, days int) ([]DailySummary, error) {
	if days <= 0 || days > 90 {
		days = 7
	}
	var results []DailySummary
	now := time.Now().UTC()
	for i := days - 1; i >= 0; i-- {
		dStr := now.AddDate(0, 0, -i).Format("2006-01-02")
		s, _ := m.GetDailySummary(deviceID, dStr)
		results = append(results, s)
	}
	return results, nil
}

// ListRecentEvents returns the latest activity events for a device (or all devices).
func (m *Manager) ListRecentEvents(deviceID string, limit int) ([]ActivityEvent, error) {
	if limit <= 0 || limit > 250 {
		limit = 60
	}
	var rows *sql.Rows
	var err error
	if deviceID == "" || deviceID == "all" {
		rows, err = m.store.DB().Query(`
			SELECT id, device_id, event_type, app_name, executable_name, window_title, duration_seconds, occurred_at, synced
			FROM activity_events
			ORDER BY occurred_at DESC, id DESC
			LIMIT ?
		`, limit)
	} else {
		rows, err = m.store.DB().Query(`
			SELECT id, device_id, event_type, app_name, executable_name, window_title, duration_seconds, occurred_at, synced
			FROM activity_events
			WHERE device_id = ?
			ORDER BY occurred_at DESC, id DESC
			LIMIT ?
		`, deviceID, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []ActivityEvent
	for rows.Next() {
		var ev ActivityEvent
		var occ time.Time
		var syncedInt int
		if err := rows.Scan(&ev.ID, &ev.DeviceID, &ev.EventType, &ev.AppName, &ev.ExecutableName,
			&ev.WindowTitle, &ev.DurationSeconds, &occ, &syncedInt); err != nil {
			continue
		}
		ev.OccurredAt = occ.Format(time.RFC3339)
		ev.Synced = syncedInt == 1
		events = append(events, ev)
	}
	if events == nil {
		events = []ActivityEvent{}
	}
	return events, nil
}

func sanitizeWindowTitle(title string) string {
	title = strings.TrimSpace(title)
	if len(title) > 80 {
		return title[:80]
	}
	return title
}

func FriendlyAppName(exeName string) string {
	lower := strings.ToLower(strings.TrimSuffix(filepath.Base(exeName), ".exe"))
	known := map[string]string{
		"msedge":           "Microsoft Edge",
		"chrome":           "Google Chrome",
		"firefox":          "Mozilla Firefox",
		"brave":            "Brave Browser",
		"code":             "Visual Studio Code",
		"explorer":         "Windows Explorer",
		"minecraft":        "Minecraft",
		"javaw":            "Minecraft (Java)",
		"robloxplayerbeta": "Roblox",
		"steam":            "Steam Client",
		"discord":          "Discord",
		"spotify":          "Spotify",
		"vlc":              "VLC Media Player",
		"winword":          "Microsoft Word",
		"excel":            "Microsoft Excel",
		"powerpnt":         "Microsoft PowerPoint",
		"notepad":          "Notepad",
		"windowsterminal":  "Windows Terminal",
		"powershell":       "PowerShell",
	}
	if v, ok := known[lower]; ok {
		return v
	}
	if len(lower) == 0 {
		return "Application"
	}
	return strings.ToUpper(lower[:1]) + lower[1:]
}

func CategorizeApp(exeName string) string {
	lower := strings.ToLower(exeName)
	switch {
	case strings.Contains(lower, "edge") || strings.Contains(lower, "chrome") || strings.Contains(lower, "firefox") || strings.Contains(lower, "brave"):
		return "Browser"
	case strings.Contains(lower, "steam") || strings.Contains(lower, "roblox") || strings.Contains(lower, "minecraft") || strings.Contains(lower, "epicgames"):
		return "Gaming"
	case strings.Contains(lower, "discord") || strings.Contains(lower, "teams") || strings.Contains(lower, "zoom"):
		return "Communication"
	case strings.Contains(lower, "code") || strings.Contains(lower, "word") || strings.Contains(lower, "excel") || strings.Contains(lower, "notepad"):
		return "Education & Productivity"
	case strings.Contains(lower, "spotify") || strings.Contains(lower, "vlc"):
		return "Media"
	default:
		return "General"
	}
}
