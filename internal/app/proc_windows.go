//go:build windows

package app

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps helper processes (voice, cloudflared) from flashing a console.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// openURL opens an address in the default browser without a shell in between.
func openURL(u string) error {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", u)
	return cmd.Start()
}
