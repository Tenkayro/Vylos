package service

import (
	"fmt"
	"sync"
	"time"

	"familycontrol/backend/applications"
	"familycontrol/backend/database"
	"familycontrol/backend/devices"
	"familycontrol/backend/family"
	"familycontrol/backend/monitoring"
	"familycontrol/backend/networking"
	"familycontrol/backend/protection"
	"familycontrol/backend/screen_time"
	"familycontrol/backend/websites"
	"familycontrol/backend/windows"
)

// BackgroundAgent runs the continuous enforcement, activity tracking, and LAN synchronization loop.
type BackgroundAgent struct {
	mu           sync.Mutex
	running      bool
	stopCh       chan struct{}
	store        *database.Store
	family       *family.Manager
	devices      *devices.Manager
	monitoring   *monitoring.Manager
	screenTime   *screen_time.Manager
	apps         *applications.Manager
	websites     *websites.Manager
	protection   *protection.Manager
	coordinator  *networking.Coordinator
	discovery    *networking.DiscoveryService
	tray         *windows.TrayManager
	enforceLock  bool
	lastBreakMin int
}

func NewBackgroundAgent(
	store *database.Store,
	fam *family.Manager,
	dev *devices.Manager,
	mon *monitoring.Manager,
	st *screen_time.Manager,
	apps *applications.Manager,
	web *websites.Manager,
	prot *protection.Manager,
	coord *networking.Coordinator,
	disc *networking.DiscoveryService,
	tray *windows.TrayManager,
	enforceLock bool,
) *BackgroundAgent {
	return &BackgroundAgent{
		store:       store,
		family:      fam,
		devices:     dev,
		monitoring:  mon,
		screenTime:  st,
		apps:        apps,
		websites:    web,
		protection:  prot,
		coordinator: coord,
		discovery:   disc,
		tray:        tray,
		enforceLock: enforceLock,
	}
}

func (a *BackgroundAgent) Start() {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return
	}
	a.running = true
	a.stopCh = make(chan struct{})
	a.mu.Unlock()

	if a.discovery != nil {
		_ = a.discovery.Start()
	}

	cfg, err := a.family.GetConfig()
	if err == nil {
		if cfg.Mode == family.ModeParent {
			_, _ = a.coordinator.StartParentServer(fmt.Sprintf(":%d", cfg.ListenPort))
		}
		if cfg.LocalDeviceID != "" {
			_ = a.monitoring.RecordSessionEvent(cfg.LocalDeviceID, "login", "Vylos Family Control agent started")
			_, _ = a.apps.RefreshInstalledApps(cfg.LocalDeviceID)
		}
	}

	go a.runLoop()
}

func (a *BackgroundAgent) Stop() {
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return
	}
	a.running = false
	close(a.stopCh)
	a.mu.Unlock()

	if a.discovery != nil {
		a.discovery.Stop()
	}
	_ = a.coordinator.StopParentServer()
}

func (a *BackgroundAgent) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}

// RunEnforcementTick executes one cycle of local activity monitoring, rule enforcement, and tray update.
func (a *BackgroundAgent) RunEnforcementTick() {
	cfg, err := a.family.GetConfig()
	if err != nil || !cfg.SetupCompleted {
		return
	}
	devID := cfg.LocalDeviceID
	if devID == "" {
		return
	}

	// Track active foreground window duration (5-second tick)
	if fg, err := windows.GetForegroundActivity(); err == nil && fg.ExecutableName != "" {
		_ = a.monitoring.RecordAppUsage(devID, fg.ExecutableName, monitoring.FriendlyAppName(fg.ExecutableName), fg.WindowTitle, 5)
	}

	if cfg.Mode == family.ModeChild {
		now := time.Now()
		eval, err := a.screenTime.Evaluate(devID, now)
		if err == nil {
			if a.tray != nil {
				statusLabel := fmt.Sprintf("Vylos Family Control — Protected (%dm left today)", eval.RemainingMinutes)
				if !eval.AllowedNow {
					statusLabel = fmt.Sprintf("Vylos Family Control — Restricted (%s)", eval.BlockReason)
				}
				a.tray.UpdateTooltip(statusLabel)

				if eval.BreakReminderDue && eval.UsedTodayMinutes != a.lastBreakMin {
					a.lastBreakMin = eval.UsedTodayMinutes
					a.tray.ShowNotification("Break Reminder", "Time for a short screen break!", false)
					_ = a.monitoring.RecordSessionEvent(devID, "break_reminder", "Scheduled break reminder")
				}
			}

			if !eval.AllowedNow && a.enforceLock {
				if a.tray != nil {
					a.tray.ShowNotification("Screen Time Rule Active", eval.BlockReason, true)
				}
				_ = windows.LockWorkStation()
			}
		}

		// Enforce blocked applications
		terminated, _ := a.apps.EnforceRunningApplications(devID, now)
		for _, exe := range terminated {
			_ = a.monitoring.RecordBlockedAttempt(devID, exe, monitoring.FriendlyAppName(exe), "Blocked by application policy")
			if a.tray != nil {
				a.tray.ShowNotification("Application Blocked", fmt.Sprintf("%s is restricted by family rules.", monitoring.FriendlyAppName(exe)), true)
			}
		}
	}
}

func (a *BackgroundAgent) runLoop() {
	enforceTicker := time.NewTicker(5 * time.Second)
	syncTicker := time.NewTicker(20 * time.Second)
	maintenanceTicker := time.NewTicker(10 * time.Minute)
	defer enforceTicker.Stop()
	defer syncTicker.Stop()
	defer maintenanceTicker.Stop()

	for {
		select {
		case <-a.stopCh:
			return
		case <-enforceTicker.C:
			a.RunEnforcementTick()
		case <-syncTicker.C:
			cfg, err := a.family.GetConfig()
			if err == nil {
				if cfg.Mode == family.ModeParent && a.coordinator.BoundAddr() == "" {
					_, _ = a.coordinator.StartParentServer(fmt.Sprintf(":%d", cfg.ListenPort))
				} else if cfg.Mode == family.ModeChild && cfg.ParentEndpoint != "" {
					_, _ = a.coordinator.SyncWithParent()
				}
			}
		case <-maintenanceTicker.C:
			if cfg, err := a.family.GetConfig(); err == nil {
				_, _ = a.store.PruneOldData(cfg.RetentionDays)
				_, _ = a.apps.RefreshInstalledApps(cfg.LocalDeviceID)
			}
		}
	}
}
