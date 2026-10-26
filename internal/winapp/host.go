package winapp

import (
	"fmt"
	"strings"
)

// Status is what the status window shows, read from the app once a second.
type Status struct {
	Code    string // the pairing code
	Link    string // the pairing link, for the QR code
	Address string // where tablets reach this PC
	Tablets int    // screens connected
	Voice   string // the voice engine's state: ready, off, loading...
	Game    string // a short line about Star Citizen, or ""
}

// Line is the status line under the title: "Running · 2 tablets connected · voice ready".
func (s Status) Line() string {
	parts := []string{"Running"}
	switch s.Tablets {
	case 0:
		parts = append(parts, "no tablets yet")
	case 1:
		parts = append(parts, "1 tablet connected")
	default:
		parts = append(parts, fmt.Sprintf("%d tablets connected", s.Tablets))
	}
	if v := strings.TrimSpace(s.Voice); v != "" {
		parts = append(parts, "voice "+v)
	}
	return strings.Join(parts, " · ")
}

// Host is what the status window and the tray can do.
type Host struct {
	Title        string
	DataDir      string        // the settings folder; another one is another app
	Hidden       bool          // start in the tray (--tray)
	Status       func() Status // called once a second while the window shows
	NewCode      func()        // a new pairing code; nil leaves the link out
	ReleaseAll   func()
	OpenSettings func() // the old control panel, in the browser
	Quit         func() // release keys and stop the servers; runs once
}
