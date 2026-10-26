//go:build !windows

package app

import (
	"os/exec"
	"runtime"
)

func hideWindow(*exec.Cmd) {}

func openURL(u string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", u).Start()
	}
	return exec.Command("xdg-open", u).Start()
}
