//go:build windows

package lmstudio

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps a console window from flashing when lms runs from a GUI
// double-click.
func hideWindow(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
}
