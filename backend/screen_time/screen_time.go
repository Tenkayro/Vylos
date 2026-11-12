package screen_time

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"familycontrol/backend/database"
)

type DayWindow struct {
	AllowedStart string `json:"allowedStart"` // e.g. "08:00"
	AllowedEnd   string `json:"allowedEnd"`   // e.g. "20:30"
	Enabled      bool   `json:"enabled"`
}

type Policy struct {
	DeviceID             string               `json:"deviceId"`
	DailyLimitMinutes    int                  `json:"dailyLimitMinutes"`
	WeekendLimitMinutes  int                  `json:"weekendLimitMinutes"`
	BedtimeStart         string               `json:"bedtimeStart"`
	BedtimeEnd           string               `json:"bedtimeEnd"`
	WeeklySchedule       map[string]DayWindow `json:"weeklySchedule"`
	BreakIntervalMinutes int                  `json:"breakIntervalMinutes"`
	BreakDurationMinutes int                  `json:"breakDurationMinutes"`
	ManualPause          bool                 `json:"manualPause"`
	ScreenLocked         bool                 `json:"screenLocked"`
	BonusMinutesToday    int                  `json:"bonusMinutesToday"`
	BonusDate            string               `json:"bonusDate"`
	UpdatedAt            string               `json:"updatedAt"`
}

type EvaluationStatus struct {
	DeviceID              string `json:"deviceId"`
	AllowedNow            bool   `json:"allowedNow"`
	BlockReason           string `json:"blockReason"`
	EffectiveLimitMinutes int    `json:"effectiveLimitMinutes"`
	UsedTodayMinutes      int    `json:"usedTodayMinutes"`
	RemainingMinutes      int    `json:"remainingMinutes"`
	InBedtime             bool   `json:"inBedtime"`
	InAllowedWindow       bool   `json:"inAllowedWindow"`
	ManualPause           bool   `json:"manualPause"`
	ScreenLocked          bool   `json:"screenLocked"`
	BreakReminderDue      bool   `json:"breakReminderDue"`
	NextTransitionLabel   string `json:"nextTransitionLabel"`
}

type Manager struct {
	store *database.Store
}

func NewManager(store *database.Store) *Manager {
	return &Manager{store: store}
}

func defaultWeeklySchedule() map[string]DayWindow {
	days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	m := make(map[string]DayWindow, len(days))
	for _, d := range days {
		end := "20:30"
		if d == "Saturday" || d == "Sunday" {
			end = "21:30"
		}
		m[d] = DayWindow{
			AllowedStart: "07:30",
			AllowedEnd:   end,
			Enabled:      true,
		}
	}
	return m
}

// GetPolicy retrieves the screen-time policy for a device (or creates a sensible default if missing).
func (m *Manager) GetPolicy(deviceID string) (Policy, error) {
	if deviceID == "" {
		deviceID = "default"
	}
	_, _ = m.store.DB().Exec(`INSERT OR IGNORE INTO screen_time_policies (device_id) VALUES (?)`, deviceID)

	var p Policy
	var schedJSON string
	var pauseInt, lockInt int
	var updated time.Time

	err := m.store.DB().QueryRow(`
		SELECT device_id, daily_limit_minutes, weekend_limit_minutes,
		       bedtime_start, bedtime_end, weekly_schedule_json,
		       break_interval_minutes, break_duration_minutes,
		       manual_pause, screen_locked, bonus_minutes_today, bonus_date, updated_at
		FROM screen_time_policies WHERE device_id = ?
	`, deviceID).Scan(
		&p.DeviceID, &p.DailyLimitMinutes, &p.WeekendLimitMinutes,
		&p.BedtimeStart, &p.BedtimeEnd, &schedJSON,
		&p.BreakIntervalMinutes, &p.BreakDurationMinutes,
		&pauseInt, &lockInt, &p.BonusMinutesToday, &p.BonusDate, &updated,
	)
	if err != nil {
		return p, err
	}

	p.ManualPause = pauseInt == 1
	p.ScreenLocked = lockInt == 1
	p.UpdatedAt = updated.Format(time.RFC3339)

	todayStr := time.Now().Format("2006-01-02")
	if p.BonusDate != todayStr {
		p.BonusMinutesToday = 0
	}

	p.WeeklySchedule = defaultWeeklySchedule()
	if schedJSON != "" && schedJSON != "{}" {
		var parsed map[string]DayWindow
		if err := json.Unmarshal([]byte(schedJSON), &parsed); err == nil && len(parsed) > 0 {
			for k, v := range parsed {
				p.WeeklySchedule[k] = v
			}
		}
	}
	return p, nil
}

// SavePolicy updates the screen-time policy for a device.
func (m *Manager) SavePolicy(p Policy) (Policy, error) {
	if p.DeviceID == "" {
		p.DeviceID = "default"
	}
	if p.DailyLimitMinutes < 15 {
		p.DailyLimitMinutes = 15
	}
	if p.WeekendLimitMinutes < 15 {
		p.WeekendLimitMinutes = 15
	}
	if p.BedtimeStart == "" {
		p.BedtimeStart = "21:00"
	}
	if p.BedtimeEnd == "" {
		p.BedtimeEnd = "07:00"
	}
	if p.BreakIntervalMinutes < 10 {
		p.BreakIntervalMinutes = 45
	}
	if p.BreakDurationMinutes < 2 {
		p.BreakDurationMinutes = 10
	}
	if len(p.WeeklySchedule) == 0 {
		p.WeeklySchedule = defaultWeeklySchedule()
	}
	schedBytes, _ := json.Marshal(p.WeeklySchedule)

	pauseInt := 0
	if p.ManualPause {
		pauseInt = 1
	}
	lockInt := 0
	if p.ScreenLocked {
		lockInt = 1
	}
	todayStr := time.Now().Format("2006-01-02")
	if p.BonusMinutesToday != 0 && p.BonusDate == "" {
		p.BonusDate = todayStr
	}

	_, err := m.store.DB().Exec(`
		INSERT INTO screen_time_policies (
			device_id, daily_limit_minutes, weekend_limit_minutes,
			bedtime_start, bedtime_end, weekly_schedule_json,
			break_interval_minutes, break_duration_minutes,
			manual_pause, screen_locked, bonus_minutes_today, bonus_date, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(device_id) DO UPDATE SET
			daily_limit_minutes = excluded.daily_limit_minutes,
			weekend_limit_minutes = excluded.weekend_limit_minutes,
			bedtime_start = excluded.bedtime_start,
			bedtime_end = excluded.bedtime_end,
			weekly_schedule_json = excluded.weekly_schedule_json,
			break_interval_minutes = excluded.break_interval_minutes,
			break_duration_minutes = excluded.break_duration_minutes,
			manual_pause = excluded.manual_pause,
			screen_locked = excluded.screen_locked,
			bonus_minutes_today = excluded.bonus_minutes_today,
			bonus_date = excluded.bonus_date,
			updated_at = CURRENT_TIMESTAMP
	`, p.DeviceID, p.DailyLimitMinutes, p.WeekendLimitMinutes,
		p.BedtimeStart, p.BedtimeEnd, string(schedBytes),
		p.BreakIntervalMinutes, p.BreakDurationMinutes,
		pauseInt, lockInt, p.BonusMinutesToday, p.BonusDate)
	if err != nil {
		return Policy{}, err
	}

	// Keep device summary table aligned
	_, _ = m.store.DB().Exec(`
		UPDATE devices SET manual_pause = ?, screen_locked = ? WHERE id = ?
	`, pauseInt, lockInt, p.DeviceID)

	return m.GetPolicy(p.DeviceID)
}

// SetLockAndPause toggles parent-controlled screen lock or manual pause for a device.
func (m *Manager) SetLockAndPause(deviceID string, screenLocked, manualPause bool) (Policy, error) {
	pol, err := m.GetPolicy(deviceID)
	if err != nil {
		return Policy{}, err
	}
	pol.ScreenLocked = screenLocked
	pol.ManualPause = manualPause
	return m.SavePolicy(pol)
}

// GrantBonusMinutes adds extra minutes for today on a device.
func (m *Manager) GrantBonusMinutes(deviceID string, additionalMinutes int) (Policy, error) {
	pol, err := m.GetPolicy(deviceID)
	if err != nil {
		return Policy{}, err
	}
	pol.BonusMinutesToday += additionalMinutes
	if pol.BonusMinutesToday < 0 {
		pol.BonusMinutesToday = 0
	}
	pol.BonusDate = time.Now().Format("2006-01-02")
	return m.SavePolicy(pol)
}

// Evaluate computes the live screen-time status for a device at a given local time.
func (m *Manager) Evaluate(deviceID string, now time.Time) (EvaluationStatus, error) {
	pol, err := m.GetPolicy(deviceID)
	if err != nil {
		return EvaluationStatus{}, err
	}

	dateStr := now.UTC().Format("2006-01-02")
	var usedSec int
	_ = m.store.DB().QueryRow(`
		SELECT COALESCE(total_seconds, 0) FROM daily_usage WHERE device_id = ? AND date_str = ?
	`, deviceID, dateStr).Scan(&usedSec)
	usedMin := usedSec / 60

	wd := now.Weekday()
	isWeekend := wd == time.Saturday || wd == time.Sunday
	baseLimit := pol.DailyLimitMinutes
	if isWeekend {
		baseLimit = pol.WeekendLimitMinutes
	}
	effectiveLimit := baseLimit + pol.BonusMinutesToday
	remaining := effectiveLimit - usedMin
	if remaining < 0 {
		remaining = 0
	}

	inBedtime := isTimeInBedtimeWindow(now, pol.BedtimeStart, pol.BedtimeEnd)
	inWindow := true
	if dw, ok := pol.WeeklySchedule[wd.String()]; ok && dw.Enabled {
		inWindow = isTimeInDayWindow(now, dw.AllowedStart, dw.AllowedEnd)
	}

	allowed := true
	var reason string
	switch {
	case pol.ScreenLocked:
		allowed = false
		reason = "Screen locked by parent"
	case pol.ManualPause:
		allowed = false
		reason = "Screen time paused by parent"
	case inBedtime:
		allowed = false
		reason = fmt.Sprintf("Bedtime active (%s – %s)", pol.BedtimeStart, pol.BedtimeEnd)
	case !inWindow:
		allowed = false
		reason = "Outside scheduled daily hours"
	case remaining <= 0:
		allowed = false
		reason = fmt.Sprintf("Daily limit of %d min reached", effectiveLimit)
	}

	breakDue := false
	if allowed && pol.BreakIntervalMinutes > 0 && usedMin > 0 && (usedMin%pol.BreakIntervalMinutes == 0) {
		breakDue = true
	}

	return EvaluationStatus{
		DeviceID:              deviceID,
		AllowedNow:            allowed,
		BlockReason:           reason,
		EffectiveLimitMinutes: effectiveLimit,
		UsedTodayMinutes:      usedMin,
		RemainingMinutes:      remaining,
		InBedtime:             inBedtime,
		InAllowedWindow:       inWindow,
		ManualPause:           pol.ManualPause,
		ScreenLocked:          pol.ScreenLocked,
		BreakReminderDue:      breakDue,
		NextTransitionLabel:   fmt.Sprintf("Bedtime at %s", pol.BedtimeStart),
	}, nil
}

func parseHHMM(hhmm string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(hhmm), ":")
	if len(parts) != 2 {
		return 0, false
	}
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err != nil {
		return 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, false
	}
	return h*60 + m, true
}

func isTimeInBedtimeWindow(now time.Time, startHHMM, endHHMM string) bool {
	startMin, ok1 := parseHHMM(startHHMM)
	endMin, ok2 := parseHHMM(endHHMM)
	if !ok1 || !ok2 {
		return false
	}
	curMin := now.Hour()*60 + now.Minute()
	// Overnight bedtime (e.g. 21:00 to 07:00)
	if startMin > endMin {
		return curMin >= startMin || curMin < endMin
	}
	return curMin >= startMin && curMin < endMin
}

func isTimeInDayWindow(now time.Time, startHHMM, endHHMM string) bool {
	startMin, ok1 := parseHHMM(startHHMM)
	endMin, ok2 := parseHHMM(endHHMM)
	if !ok1 || !ok2 {
		return true
	}
	curMin := now.Hour()*60 + now.Minute()
	if startMin <= endMin {
		return curMin >= startMin && curMin <= endMin
	}
	return curMin >= startMin || curMin <= endMin
}
