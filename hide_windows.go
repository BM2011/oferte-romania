//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

func hideWindow(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// ascunde fereastra consolei când programul pornește în fundal (odată cu Windows)
func hideConsole() {
	k := syscall.NewLazyDLL("kernel32.dll")
	_, _, _ = k.NewProc("FreeConsole").Call()
}
