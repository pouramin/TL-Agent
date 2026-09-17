//go:build windows

package main

import "syscall"

func windowsHideProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{HideWindow: true}
}
