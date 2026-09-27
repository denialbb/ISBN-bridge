//go:build !windows

package main

func initConsole()           {}
func hideConsole()           {}
func showConsole()           {}
func toggleConsole() bool    { return false }
func isConsoleVisible() bool { return false }
