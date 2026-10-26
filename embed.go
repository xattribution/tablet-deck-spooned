// Package tabletdeck holds the files built into the app: the deck web app, the
// bundled layouts and the keybind profile.
package tabletdeck

import "embed"

//go:embed public layouts keybinds
var Files embed.FS

// Version is set at build time with -ldflags "-X github.com/xattribution/tablet-deck-spooned.Version=2.0.0".
var Version = "2.0.0-dev"
