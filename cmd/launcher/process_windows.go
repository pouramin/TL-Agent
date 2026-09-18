//go:build windows

package main

import (
	"os/exec"
	"strconv"
)

func configureManagedCommand(cmd *exec.Cmd) {}

func terminateManagedProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if err := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
