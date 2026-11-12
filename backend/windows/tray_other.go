//go:build !windows

package windows

type TrayManager struct {
	tooltip string
}

func NewTrayManager(initialTooltip string, onOpen, onSync, onExit func()) *TrayManager {
	return &TrayManager{tooltip: initialTooltip}
}

func (t *TrayManager) Start() {}
func (t *TrayManager) Stop()  {}
func (t *TrayManager) UpdateTooltip(tooltip string) {
	t.tooltip = tooltip
}
func (t *TrayManager) ShowNotification(title, message string, warning bool) {}
