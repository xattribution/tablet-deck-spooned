//go:build !windows

// Package winapp is Tablet Deck's Windows side; elsewhere these do little, so the
// server can run headless for development and testing.
package winapp

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
)

func SingleInstance(string) bool     { return true }
func ShowRunning(string) bool        { return false }
func AttachConsole()                 {}
func StartsWithWindows() bool        { return false }
func SetStartWithWindows(bool) error { return fmt.Errorf("Windows only") }
func RegisterLinks() error           { return nil }
func Message(title, text string)     { fmt.Fprintf(os.Stderr, "%s: %s\n", title, text) }

func OpenBrowser(url string) {
	if runtime.GOOS == "darwin" {
		_ = exec.Command("open", url).Start()
		return
	}
	_ = exec.Command("xdg-open", url).Start()
}

// Run has no status window or tray off Windows: it logs where to pair and blocks
// until the process is stopped.
func Run(h Host) {
	if h.Status != nil {
		s := h.Status()
		log.Printf("%s: %s. Pair at %s (code %s).", h.Title, s.Line(), s.Link, s.Code)
	}
	select {}
}
