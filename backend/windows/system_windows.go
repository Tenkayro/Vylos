//go:build windows

package windows

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	user32                       = windows.NewLazySystemDLL("user32.dll")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
	procLockWorkStation          = user32.NewProc("LockWorkStation")
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

// criticalSystemProcesses is a strict hard-coded denylist of Windows system-critical processes
// that FamilyControl must NEVER terminate under any circumstance.
var criticalSystemProcesses = map[string]bool{
	"system":                      true,
	"registry":                    true,
	"smss.exe":                    true,
	"csrss.exe":                   true,
	"wininit.exe":                 true,
	"services.exe":                true,
	"lsass.exe":                   true,
	"svchost.exe":                 true,
	"winlogon.exe":                true,
	"explorer.exe":                true,
	"dwm.exe":                     true,
	"fontdrvhost.exe":             true,
	"spoolsv.exe":                 true,
	"sihost.exe":                  true,
	"taskhostw.exe":               true,
	"ctfmon.exe":                  true,
	"runtimebroker.exe":           true,
	"searchhost.exe":              true,
	"startmenuexperiencehost.exe": true,
	"shellexperiencehost.exe":     true,
	"msmpeng.exe":                 true,
	"securityhealthservice.exe":   true,
	"securityhealthsystray.exe":   true,
	"audiodg.exe":                 true,
	"conhost.exe":                 true,
	"dllhost.exe":                 true,
	"wudfhost.exe":                true,
	"vylos.exe":                   true,
	"vylosservice.exe":            true,
	"familycontrol.exe":           true,
	"familycontrolservice.exe":    true,
}

// IsSystemCriticalProcess returns true if the executable name belongs to a protected OS or FamilyControl binary.
func IsSystemCriticalProcess(exeName string) bool {
	cleaned := strings.ToLower(strings.TrimSpace(filepath.Base(exeName)))
	if cleaned == "" {
		return true
	}
	return criticalSystemProcesses[cleaned]
}

// GetForegroundActivity returns the currently focused window's process and title.
func GetForegroundActivity() (ProcessInfo, error) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return ProcessInfo{}, errors.New("no foreground window")
	}

	titleBuf := make([]uint16, 256)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&titleBuf[0])), uintptr(len(titleBuf)))
	windowTitle := windows.UTF16ToString(titleBuf)

	var pid uint32
	procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid == 0 {
		return ProcessInfo{WindowTitle: windowTitle}, nil
	}

	fullPath, exeName := queryProcessPath(pid)
	if exeName == "" {
		exeName = "Application"
	}

	return ProcessInfo{
		PID:            pid,
		ExecutableName: exeName,
		FullPath:       fullPath,
		WindowTitle:    windowTitle,
	}, nil
}

func queryProcessPath(pid uint32) (fullPath string, exeName string) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", ""
	}
	defer windows.CloseHandle(handle)

	buf := make([]uint16, windows.MAX_PATH*2)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return "", ""
	}
	fullPath = windows.UTF16ToString(buf[:size])
	exeName = filepath.Base(fullPath)
	return fullPath, exeName
}

// ListRunningProcesses enumerates active user/system processes using CreateToolhelp32Snapshot.
func ListRunningProcesses() ([]ProcessInfo, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("CreateToolhelp32Snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)

	var pe32 windows.ProcessEntry32
	pe32.Size = uint32(unsafe.Sizeof(pe32))
	if err := windows.Process32First(snap, &pe32); err != nil {
		return nil, err
	}

	var results []ProcessInfo
	myPID := uint32(os.Getpid())

	for {
		exeName := windows.UTF16ToString(pe32.ExeFile[:])
		if pe32.ProcessID > 4 && pe32.ProcessID != myPID && exeName != "" {
			fullPath, _ := queryProcessPath(pe32.ProcessID)
			results = append(results, ProcessInfo{
				PID:            pe32.ProcessID,
				ExecutableName: exeName,
				FullPath:       fullPath,
			})
		}
		if err := windows.Process32Next(snap, &pe32); err != nil {
			break
		}
	}
	return results, nil
}

// TerminateAllowedProcess safely terminates a blocked user application if and only if it is not a system-critical process.
func TerminateAllowedProcess(pid uint32, exeName string) error {
	if IsSystemCriticalProcess(exeName) {
		return fmt.Errorf("refusing to terminate protected system process: %s", exeName)
	}
	if pid <= 4 || pid == uint32(os.Getpid()) {
		return errors.New("refusing to terminate reserved PID")
	}

	// Double-check actual process executable name from PID
	_, actualExe := queryProcessPath(pid)
	if actualExe != "" && IsSystemCriticalProcess(actualExe) {
		return fmt.Errorf("refusing to terminate protected system process: %s", actualExe)
	}

	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return fmt.Errorf("OpenProcess(%d): %w", pid, err)
	}
	defer windows.CloseHandle(handle)

	if err := windows.TerminateProcess(handle, 1); err != nil {
		return fmt.Errorf("TerminateProcess(%d): %w", pid, err)
	}
	return nil
}

// LockWorkStation locks the interactive Windows user session when enforced by parent rules.
func LockWorkStation() error {
	r1, _, err := procLockWorkStation.Call()
	if r1 == 0 {
		return fmt.Errorf("LockWorkStation failed: %v", err)
	}
	return nil
}

// ScanInstalledApplications reads installed applications from Windows Registry Uninstall keys.
func ScanInstalledApplications() ([]InstalledAppInfo, error) {
	type regRoot struct {
		key  registry.Key
		path string
	}
	roots := []regRoot{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`},
	}

	seen := make(map[string]bool)
	var apps []InstalledAppInfo

	for _, r := range roots {
		k, err := registry.OpenKey(r.key, r.path, registry.ENUMERATE_SUB_KEYS|registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		subkeys, err := k.ReadSubKeyNames(-1)
		k.Close()
		if err != nil {
			continue
		}

		for _, sub := range subkeys {
			sk, err := registry.OpenKey(r.key, r.path+`\`+sub, registry.QUERY_VALUE)
			if err != nil {
				continue
			}
			displayName, _, _ := sk.GetStringValue("DisplayName")
			version, _, _ := sk.GetStringValue("DisplayVersion")
			publisher, _, _ := sk.GetStringValue("Publisher")
			installLoc, _, _ := sk.GetStringValue("InstallLocation")
			displayIcon, _, _ := sk.GetStringValue("DisplayIcon")
			sysComponent, _, _ := sk.GetIntegerValue("SystemComponent")
			sk.Close()

			displayName = strings.TrimSpace(displayName)
			if displayName == "" || sysComponent == 1 {
				continue
			}

			exeName := deriveExecutableName(displayName, installLoc, displayIcon)
			keyLower := strings.ToLower(exeName)
			if seen[keyLower] {
				continue
			}
			seen[keyLower] = true

			apps = append(apps, InstalledAppInfo{
				ExecutableName: exeName,
				DisplayName:    displayName,
				Version:        strings.TrimSpace(version),
				Publisher:      strings.TrimSpace(publisher),
				InstallPath:    strings.TrimSpace(installLoc),
			})
			if len(apps) >= 120 {
				break
			}
		}
	}
	return apps, nil
}

func deriveExecutableName(displayName, installLoc, displayIcon string) string {
	iconClean := strings.Trim(strings.Split(displayIcon, ",")[0], `" `)
	if strings.HasSuffix(strings.ToLower(iconClean), ".exe") {
		base := filepath.Base(iconClean)
		if !strings.Contains(strings.ToLower(base), "unins") && !strings.Contains(strings.ToLower(base), "msiexec") {
			return base
		}
	}
	if installLoc != "" {
		if entries, err := os.ReadDir(installLoc); err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".exe") && !strings.Contains(strings.ToLower(e.Name()), "unins") {
					return e.Name()
				}
			}
		}
	}
	// Fallback slug from displayName
	slug := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return -1
	}, displayName)
	if len(slug) > 24 {
		slug = slug[:24]
	}
	if slug == "" {
		slug = "app"
	}
	return strings.ToLower(slug) + ".exe"
}

// CheckWindowsFirewallEnabled checks the Windows Firewall status in the Registry for Standard and Public profiles.
func CheckWindowsFirewallEnabled() bool {
	profiles := []string{
		`SYSTEM\CurrentControlSet\Services\SharedAccess\Parameters\FirewallPolicy\StandardProfile`,
		`SYSTEM\CurrentControlSet\Services\SharedAccess\Parameters\FirewallPolicy\PublicProfile`,
	}
	for _, p := range profiles {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, p, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		val, _, err := k.GetIntegerValue("EnableFirewall")
		k.Close()
		if err == nil && val == 1 {
			return true
		}
	}
	return false
}

// GetConfiguredDNSServers reads the active DNS server addresses from Windows network adapter registry keys.
func GetConfiguredDNSServers() []string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces`, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	defer k.Close()

	subs, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}
	var servers []string
	seen := make(map[string]bool)

	for _, s := range subs {
		ik, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Tcpip\Parameters\Interfaces\`+s, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		ns, _, _ := ik.GetStringValue("NameServer")
		dhcpNs, _, _ := ik.GetStringValue("DhcpNameServer")
		ik.Close()

		for _, raw := range []string{ns, dhcpNs} {
			for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
				part = strings.TrimSpace(part)
				if part != "" && !seen[part] {
					seen[part] = true
					servers = append(servers, part)
				}
			}
		}
	}
	return servers
}

// ApplyDNSServers attempts to configure the primary and secondary IPv4/IPv6 DNS servers on active LAN interfaces.
// Requires Administrator / Windows Service privileges.
func ApplyDNSServers(primaryIPv4, secondaryIPv4, primaryIPv6, secondaryIPv6 string) error {
	if net.ParseIP(primaryIPv4) == nil {
		return errors.New("invalid primary IPv4 DNS address")
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	applied := false
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		cmd1 := exec.Command("netsh", "interface", "ipv4", "set", "dnsservers",
			fmt.Sprintf("name=%s", iface.Name), "static", primaryIPv4, "primary", "validate=no")
		cmd1.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd1.Run(); err == nil {
			applied = true
			if net.ParseIP(secondaryIPv4) != nil {
				cmd2 := exec.Command("netsh", "interface", "ipv4", "add", "dnsservers",
					fmt.Sprintf("name=%s", iface.Name), secondaryIPv4, "index=2", "validate=no")
				cmd2.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				_ = cmd2.Run()
			}
			if net.ParseIP(primaryIPv6) != nil {
				cmd3 := exec.Command("netsh", "interface", "ipv6", "set", "dnsservers",
					fmt.Sprintf("name=%s", iface.Name), "static", primaryIPv6, "primary", "validate=no")
				cmd3.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				_ = cmd3.Run()
			}
			if net.ParseIP(secondaryIPv6) != nil {
				cmd4 := exec.Command("netsh", "interface", "ipv6", "add", "dnsservers",
					fmt.Sprintf("name=%s", iface.Name), secondaryIPv6, "index=2", "validate=no")
				cmd4.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
				_ = cmd4.Run()
			}
		}
	}
	if !applied {
		return errors.New("elevated privileges required to modify adapter DNS settings")
	}
	return nil
}

// GetOSVersionString returns a friendly Windows version string.
func GetOSVersionString() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return "Windows (" + runtime.GOARCH + ")"
	}
	defer k.Close()
	prod, _, _ := k.GetStringValue("ProductName")
	build, _, _ := k.GetStringValue("CurrentBuildNumber")
	if prod != "" {
		if build >= "22000" && strings.Contains(prod, "Windows 10") {
			prod = strings.Replace(prod, "Windows 10", "Windows 11", 1)
		}
		if build != "" {
			return fmt.Sprintf("%s (Build %s)", prod, build)
		}
		return prod
	}
	return "Windows 11"
}

// GetHostsFilePath returns the Windows system hosts file path.
func GetHostsFilePath() string {
	sysRoot := os.Getenv("SystemRoot")
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	return filepath.Join(sysRoot, "System32", "drivers", "etc", "hosts")
}
