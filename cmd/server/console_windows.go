//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	user32              = syscall.NewLazyDLL("user32.dll")
	procSetConsoleTitle = kernel32.NewProc("SetConsoleTitleW")
	procGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	procShowWindow      = user32.NewProc("ShowWindow")
	procIsWindowVisible = user32.NewProc("IsWindowVisible")
	procSetForeground   = user32.NewProc("SetForegroundWindow")
)

func initConsole() {
	title, err := syscall.UTF16PtrFromString("ISBN Bridge Server")
	if err == nil {
		procSetConsoleTitle.Call(uintptr(unsafe.Pointer(title)))
	}
}

func hideConsole() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd != 0 {
		// SW_HIDE = 0
		procShowWindow.Call(hwnd, 0)
	}
}

func showConsole() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd != 0 {
		// SW_RESTORE = 9, SW_SHOW = 5
		procShowWindow.Call(hwnd, 9)
		procSetForeground.Call(hwnd)
	}
}

func toggleConsole() bool {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return false
	}
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	if visible != 0 {
		hideConsole()
		return false
	}
	showConsole()
	return true
}

func isConsoleVisible() bool {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return false
	}
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	return visible != 0
}
