//go:build windows
// +build windows

// cmd/readsync-tray/tray_windows_menu.go
//
// Menu / icon helpers split off so the main file stays under the
// 6 kB editor limit.

package main

import (
	"syscall"
	"unsafe"
)

func addTrayIcon(hwnd uintptr, tip string) error {
	nid := notifyIconData{
		HWnd: hwnd, UID: 1,
		Flags:           nifMessage | nifTip,
		CallbackMessage: wmTrayCB,
	}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	copyTip(&nid.Tip, tip)
	if r, _, _ := procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid))); r == 0 {
		return errShellNotifyIcon
	}
	return nil
}

var errShellNotifyIcon = winErr("Shell_NotifyIconW NIM_ADD failed")

type winErr string

func (e winErr) Error() string { return string(e) }

func removeTrayIcon(hwnd uintptr) {
	nid := notifyIconData{HWnd: hwnd, UID: 1}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
}

func updateTrayTip(hwnd uintptr, tip string) {
	nid := notifyIconData{HWnd: hwnd, UID: 1, Flags: nifTip}
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	copyTip(&nid.Tip, tip)
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func copyTip(dst *[128]uint16, src string) {
	u, _ := syscall.UTF16FromString(src)
	if len(u) > 127 {
		u = u[:127]
	}
	for i, c := range u {
		dst[i] = c
	}
}

func wndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmTrayCB:
		if lParam == wmRButtonUp || lParam == wmLButtonDown {
			showMenu(hwnd)
		}
		return 0
	case wmCommand:
		go handleMenuCommand(int(wParam & 0xffff))
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func showMenu(hwnd uintptr) {
	hMenu, _, _ := procCreatePopupMenu.Call()
	defer procDestroyMenu.Call(hMenu)
	add := func(id uintptr, label string) {
		s, _ := syscall.UTF16PtrFromString(label)
		procAppendMenuW.Call(hMenu, 0x0000, id, uintptr(unsafe.Pointer(s)))
	}
	add(idmDashboard, "Open dashboard")
	add(idmSync, "Sync now")
	add(idmConflicts, "View conflicts")
	add(idmActivity, "View activity log")
	add(idmRestart, "Restart service")
	add(idmQuit, "Quit")

	procSetForegroundWindow.Call(hwnd)
	var pt pointStruct
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procTrackPopupMenu.Call(hMenu, 0, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
	procPostMessageW.Call(hwnd, 0, 0, 0)
}

func handleMenuCommand(id int) {
	trayMu.Lock()
	cli := trayCli
	trayMu.Unlock()
	if cli == nil {
		return
	}
	switch id {
	case idmDashboard:
		openURL(cli.base + "/ui/dashboard")
	case idmSync:
		_ = cli.SyncNow()
	case idmConflicts:
		openURL(cli.base + "/ui/conflicts")
	case idmActivity:
		openURL(cli.base + "/ui/activity")
	case idmRestart:
		_ = cli.RestartService()
	case idmQuit:
		procPostMessageW.Call(hwndTray, wmDestroy, 0, 0)
	}
}
