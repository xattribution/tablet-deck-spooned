//go:build !windows

package input

import "os"

// New returns a fake driver off Windows: nothing is pressed. TD_FAKE_GAME picks what it
// reports: "front" (default), "background" or "off", to try the focus rules.
func New(dry bool) Driver {
	f := NewFake()
	switch os.Getenv("TD_FAKE_GAME") {
	case "background":
		f.SetGame(true, false, true)
	case "off":
		f.SetGame(false, false, false)
	}
	return f
}
