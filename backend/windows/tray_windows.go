//go:build windows

package windows

import (
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32              = windows.NewLazySystemDLL("shell32.dll")
	kernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostMessageW     = user32.NewProc("PostMessageW")
	procLoadIconW        = user32.NewProc("LoadIconW")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenuW      = user32.NewProc("AppendMenuW")
	procTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	procDestroyMenu      = user32.NewProc("DestroyMenu")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procSetForegroundWnd = user32.NewProc("SetForegroundWindow")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
)

const (
	nimAdd         = 0x00000000
	nimModify      = 0x00000001
	nimDelete      = 0x00000002
	nifMessage     = 0x00000001
	nifIcon        = 0x00000002
	nifTip         = 0x00000004
	nifInfo        = 0x00000010
	niifInfo       = 0x00000001
	niifWarning    = 0x00000002
	wmAppTray      = 0x8000 + 42
	wmCommand      = 0x0111
	wmClose        = 0x0010
	wmLButtonUp    = 0x0202
	wmRButtonUp    = 0x0205
	idiShield      = 32518
	mfString       = 0x00000000
	mfSeparator    = 0x00000800
	tpmBottomAlign = 0x0020
	tpmLeftAlign   = 0x0000
	idMenuOpen     = 1001
	idMenuSync     = 1002
	idMenuExit     = 1003
)

type notifyIconDataW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
}

type wndClassExW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type point struct {
	X, Y int32
}

type msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
}

// TrayManager manages the native Windows system tray icon and balloon notifications.
type TrayManager struct {
	mu      sync.Mutex
	hwnd    uintptr
	running bool
	tooltip string
	onOpen  func()
	onSync  func()
	onExit  func()
}

var activeTray *TrayManager

func NewTrayManager(initialTooltip string, onOpen, onSync, onExit func()) *TrayManager {
	tm := &TrayManager{
		tooltip: initialTooltip,
		onOpen:  onOpen,
		onSync:  onSync,
		onExit:  onExit,
	}
	activeTray = tm
	return tm
}

func (t *TrayManager) Start() {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return
	}
	t.running = true
	t.mu.Unlock()

	go t.runLoop()
}

func (t *TrayManager) Stop() {
	t.mu.Lock()
	hwnd := t.hwnd
	t.running = false
	t.mu.Unlock()
	if hwnd != 0 {
		var nid notifyIconDataW
		nid.CbSize = uint32(unsafe.Sizeof(nid))
		nid.HWnd = hwnd
		nid.UID = 1
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
		procPostMessageW.Call(hwnd, wmClose, 0, 0)
	}
}

func (t *TrayManager) UpdateTooltip(tooltip string) {
	t.mu.Lock()
	t.tooltip = tooltip
	hwnd := t.hwnd
	t.mu.Unlock()
	if hwnd == 0 {
		return
	}
	var nid notifyIconDataW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = hwnd
	nid.UID = 1
	nid.UFlags = nifTip
	copyUTF16(nid.SzTip[:], tooltip)
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

// ShowNotification displays a native Windows notification balloon from the system tray icon.
func (t *TrayManager) ShowNotification(title, message string, warning bool) {
	t.mu.Lock()
	hwnd := t.hwnd
	t.mu.Unlock()
	if hwnd == 0 {
		return
	}
	var nid notifyIconDataW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = hwnd
	nid.UID = 1
	nid.UFlags = nifInfo
	copyUTF16(nid.SzInfoTitle[:], title)
	copyUTF16(nid.SzInfo[:], message)
	if warning {
		nid.DwInfoFlags = niifWarning
	} else {
		nid.DwInfoFlags = niifInfo
	}
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func (t *TrayManager) runLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := windows.UTF16PtrFromString("FamilyControlTrayWndClass")

	var wc wndClassExW
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.LpfnWndProc = windows.NewCallback(trayWndProc)
	wc.HInstance = hInst
	wc.LpszClassName = className
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		0, 0, 0, 0, 0, 0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return
	}

	hIcon, _, _ := procLoadIconW.Call(0, idiShield)

	t.mu.Lock()
	t.hwnd = hwnd
	tip := t.tooltip
	t.mu.Unlock()

	var nid notifyIconDataW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = hwnd
	nid.UID = 1
	nid.UFlags = nifMessage | nifIcon | nifTip
	nid.UCallbackMessage = wmAppTray
	nid.HIcon = hIcon
	copyUTF16(nid.SzTip[:], tip)

	procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))

	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func trayWndProc(hwnd uintptr, uMsg uint32, wParam, lParam uintptr) uintptr {
	switch uMsg {
	case wmAppTray:
		switch uint32(lParam) {
		case wmLButtonUp:
			if activeTray != nil && activeTray.onOpen != nil {
				go activeTray.onOpen()
			}
		case wmRButtonUp:
			showTrayMenu(hwnd)
		}
		return 0
	case wmCommand:
		switch uint32(wParam & 0xffff) {
		case idMenuOpen:
			if activeTray != nil && activeTray.onOpen != nil {
				go activeTray.onOpen()
			}
		case idMenuSync:
			if activeTray != nil && activeTray.onSync != nil {
				go activeTray.onSync()
			}
		case idMenuExit:
			if activeTray != nil && activeTray.onExit != nil {
				go activeTray.onExit()
			}
		}
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(uMsg), wParam, lParam)
	return ret
}

func showTrayMenu(hwnd uintptr) {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	lblOpen, _ := windows.UTF16PtrFromString("Open FamilyControl")
	lblSync, _ := windows.UTF16PtrFromString("Synchronize Rules Now")
	lblExit, _ := windows.UTF16PtrFromString("Minimize to Tray")

	procAppendMenuW.Call(hMenu, mfString, idMenuOpen, uintptr(unsafe.Pointer(lblOpen)))
	procAppendMenuW.Call(hMenu, mfString, idMenuSync, uintptr(unsafe.Pointer(lblSync)))
	procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	procAppendMenuW.Call(hMenu, mfString, idMenuExit, uintptr(unsafe.Pointer(lblExit)))

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWnd.Call(hwnd)
	procTrackPopupMenu.Call(hMenu, tpmBottomAlign|tpmLeftAlign, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
}

func copyUTF16(dst []uint16, s string) {
	u, err := windows.UTF16FromString(s)
	if err != nil {
		return
	}
	if len(u) >= len(dst) {
		copy(dst, u[:len(dst)-1])
		dst[len(dst)-1] = 0
	} else {
		copy(dst, u)
	}
}
