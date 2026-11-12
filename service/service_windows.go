//go:build windows

package service

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows/svc"
)

const ServiceName = "FamilyControlService"
const ServiceDisplayName = "FamilyControl Parental Control Service"
const ServiceDescription = "Enforces local FamilyControl screen-time, application, and DNS protection rules and synchronizes with the Parent computer over LAN."

type windowsServiceHandler struct {
	agent *BackgroundAgent
}

func (h *windowsServiceHandler) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	if h.agent != nil {
		h.agent.Start()
	}

	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}

loop:
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				break loop
			}
		}
	}

	changes <- svc.Status{State: svc.StopPending}
	if h.agent != nil {
		h.agent.Stop()
	}
	return false, 0
}

// IsWindowsServiceSession detects whether the current process is running inside the Windows Service Control Manager.
func IsWindowsServiceSession() bool {
	inService, err := svc.IsWindowsService()
	if err != nil {
		return false
	}
	return inService
}

// RunWindowsService starts the Windows Service dispatcher loop.
func RunWindowsService(agent *BackgroundAgent) error {
	return svc.Run(ServiceName, &windowsServiceHandler{agent: agent})
}

// InstallWindowsService registers FamilyControlService with automatic startup and hardened service permissions (SDDL).
func InstallWindowsService(exePath string) error {
	binPath := fmt.Sprintf(`"%s" --service`, exePath)
	createCmd := exec.Command("sc.exe", "create", ServiceName,
		"binPath=", binPath,
		"start=", "auto",
		"DisplayName=", ServiceDisplayName)
	createCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := createCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sc create failed: %w (%s)", err, string(out))
	}

	descCmd := exec.Command("sc.exe", "description", ServiceName, ServiceDescription)
	descCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = descCmd.Run()

	// Configure automatic restart on failure
	failCmd := exec.Command("sc.exe", "failure", ServiceName, "reset=", "86400", "actions=", "restart/5000/restart/5000/restart/5000")
	failCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = failCmd.Run()

	// Harden SDDL so standard users (children) can read status but cannot stop, pause, or delete the service;
	// only SYSTEM (SY) and Administrators (BA) have full control.
	sddl := "D:(A;;CCLCSWRPWPDTLOCRRC;;;SY)(A;;CCDCLCSWRPWPDTLOCRSDRCWDWO;;;BA)(A;;CCLCSWLOCRRC;;;IU)"
	sdCmd := exec.Command("sc.exe", "sdset", ServiceName, sddl)
	sdCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = sdCmd.Run()

	startCmd := exec.Command("sc.exe", "start", ServiceName)
	startCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = startCmd.Run()
	return nil
}

// UninstallWindowsService stops and deletes the FamilyControl Windows Service (requires Administrator + Parent Authorization).
func UninstallWindowsService() error {
	stopCmd := exec.Command("sc.exe", "stop", ServiceName)
	stopCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = stopCmd.Run()

	delCmd := exec.Command("sc.exe", "delete", ServiceName)
	delCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, err := delCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sc delete failed: %w (%s)", err, string(out))
	}
	return nil
}
