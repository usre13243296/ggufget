//go:build !windows

package lmstudio

import "os/exec"

func hideWindow(cmd *exec.Cmd) {}
