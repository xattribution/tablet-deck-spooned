//go:build !windows

// Package winapp is Tablet Deck's Windows side; elsewhere these do little, so the
// server can run headless for development and testing.
package winapp

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func SingleInstance(string) bool         { return true }
func AttachConsole()                     {}
func StartsWithWindows() bool            { return false }
func SetStartWithWindows(bool) error     { return fmt.Errorf("Windows only") }
func RegisterLinks() error               { return nil }
func OpenWindow(url, _ string, _, _ int) { OpenBrowser(url) }
func Message(title, text string)         { fmt.Fprintf(os.Stderr, "%s: %s\n", title, text) }

func OpenBrowser(url string) {
	if runtime.GOOS == "darwin" {
		_ = exec.Command("open", url).Start()
		return
	}
	_ = exec.Command("xdg-open", url).Start()
}

type Tray struct {
	Title       string
	OpenPanel   func()
	OpenPairing func()
	OpenTouch   func()
	ReleaseAll  func()
	ToggleAfk   func() bool
	Quit        func()
}

// RunTray has no tray off Windows; it blocks until the process is stopped.
func RunTray(Tray) { select {} }
