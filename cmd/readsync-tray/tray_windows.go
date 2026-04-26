//go:build windows
// +build windows

// cmd/readsync-tray/tray_windows.go
//
// Native Windows system-tray icon using Shell_NotifyIconW. The
// right-click menu calls the local service via the loopback API
// (CSRF-protected). Tooltip refreshes every 5s with the aggregated
// adapter health (ok / degraded / needs_user_action / failed).

package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
)

const (
	wmApp     = 0x8000
	wmTrayCB  = wmApp + 1
	wmDestroy = 0x0002
	wmCommand = 0x0111

	wmRButtonUp   = 0x0205
	wmLButtonDown = 0x0201

	nimAdd    = 0x0
	nimModify = 0x1
	nimDelete = 0x2

	nifMessage = 0x1
	nifTip     = 0x4

	idmDashboard = 1001
	idmSync      = 1002
	idmConflicts = 1003
	idmActivity  = 1004
	idmRestart   = 1005
	idmQuit      = 1006
)

type pointStruct struct{ X, Y int32 }

type notifyIconData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	Flags            uint32
	CallbackMessage  uint32
	HIcon            uintptr
	Tip              [128]uint16
	State            uint32
	StateMask        uint32
	Info             [256]uint16
	TimeoutOrVersion uint32
	InfoTitle        [64]uint16
	InfoFlags        uint32
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

type msgStruct struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      pointStruct
}

var (
	hwndTray uintptr
	trayMu   sync.Mutex
	trayCli  *ServiceClient
)

// runNativeTray installs a tray icon and runs the Win32 message loop.
func runNativeTray(ctx context.Context, client *ServiceClient) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	trayCli = client
	hInstance, _, _ := procGetModuleHandleW.Call(0)
	className, _ := syscall.UTF16PtrFromString("ReadSyncTrayClass")

	wc := wndClassExW{
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     hInstance,
		LpszClassName: className,
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	if r, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("RegisterClassExW failed")
	}
	winName, _ := syscall.UTF16PtrFromString("ReadSync")
	hwnd, _, _ := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(winName)),
		0, 0, 0, 0, 0, 0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW failed")
	}
	hwndTray = hwnd

	if err := addTrayIcon(hwnd, "ReadSync starting…"); err != nil {
		return err
	}
	defer removeTrayIcon(hwnd)

	go pollLoop(ctx, hwnd, client)

	var m msgStruct
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		select {
		case <-ctx.Done():
			return nil
		default:
		}
	}
	return nil
}

func pollLoop(ctx context.Context, hwnd uintptr, client *ServiceClient) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		updateTrayTip(hwnd, computeTip(client))
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func computeTip(client *ServiceClient) string {
	if !client.Healthz() {
		return "ReadSync: service unreachable"
	}
	adapters, err := client.Adapters()
	if err != nil {
		return "ReadSync: " + err.Error()
	}
	return "ReadSync: " + OverallHealth(adapters)
}
