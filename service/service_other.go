//go:build !windows

package service

func IsWindowsServiceSession() bool {
	return false
}

func RunWindowsService(agent *BackgroundAgent) error {
	agent.Start()
	select {}
}

func InstallWindowsService(exePath string) error {
	return nil
}

func UninstallWindowsService() error {
	return nil
}
