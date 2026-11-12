//go:build !windows

package windows

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
)

type ProcessInfo struct {
	PID            uint32 `json:"pid"`
	ExecutableName string `json:"executableName"`
	FullPath       string `json:"fullPath"`
	WindowTitle    string `json:"windowTitle"`
}

type InstalledAppInfo struct {
	ExecutableName string `json:"executableName"`
	DisplayName    string `json:"displayName"`
	Version        string `json:"version"`
	Publisher      string `json:"publisher"`
	InstallPath    string `json:"installPath"`
}

func IsSystemCriticalProcess(exeName string) bool {
	cleaned := strings.ToLower(strings.TrimSpace(filepath.Base(exeName)))
	return cleaned == "" || cleaned == "system" || cleaned == "explorer.exe" || cleaned == "svchost.exe" || cleaned == "csrss.exe" || cleaned == "lsass.exe" || cleaned == "vylos.exe" || cleaned == "vylos-child.exe" || cleaned == "familycontrol.exe"
}

func GetForegroundActivity() (ProcessInfo, error) {
	return ProcessInfo{}, errors.New("not supported on non-windows platform")
}

func ListRunningProcesses() ([]ProcessInfo, error) {
	return []ProcessInfo{}, nil
}

func TerminateAllowedProcess(pid uint32, exeName string) error {
	if IsSystemCriticalProcess(exeName) {
		return errors.New("refusing to terminate protected system process")
	}
	return nil
}

func LockWorkStation() error {
	return nil
}

func ScanInstalledApplications() ([]InstalledAppInfo, error) {
	return []InstalledAppInfo{}, nil
}

func CheckWindowsFirewallEnabled() bool {
	return true
}

func GetConfiguredDNSServers() []string {
	return []string{"1.1.1.3", "1.0.0.3"}
}

func ApplyDNSServers(primaryIPv4, secondaryIPv4, primaryIPv6, secondaryIPv6 string) error {
	return nil
}

func GetOSVersionString() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func GetHostsFilePath() string {
	return "/etc/hosts"
}
