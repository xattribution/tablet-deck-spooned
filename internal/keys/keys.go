// Package keys maps Star Citizen key names ("lalt", "np_1", "f12") to PC/AT set 1
// scancodes. Scancodes, not virtual keys, because Star Citizen reads raw input and
// ignores virtual-key-only events.
package keys

import (
	"fmt"
	"sort"
	"strings"
)

// Scan is one physical key: its set 1 scancode and whether it's an extended (E0) key.
type Scan struct {
	Code uint16 `json:"code"`
	Ext  bool   `json:"ext"`
}

func (s Scan) String() string {
	if s.Ext {
		return fmt.Sprintf("e0%02x", s.Code)
	}
	return fmt.Sprintf("%02x", s.Code)
}

// IsModifier reports whether the key is a shift, ctrl or alt key.
func (s Scan) IsModifier() bool {
	switch s {
	case table["lshift"], table["rshift"], table["lctrl"], table["rctrl"], table["lalt"], table["ralt"]:
		return true
	}
	return false
}

func k(code uint16) Scan  { return Scan{Code: code} }
func ke(code uint16) Scan { return Scan{Code: code, Ext: true} }

var table = map[string]Scan{
	"escape": k(0x01), "1": k(0x02), "2": k(0x03), "3": k(0x04), "4": k(0x05), "5": k(0x06),
	"6": k(0x07), "7": k(0x08), "8": k(0x09), "9": k(0x0a), "0": k(0x0b), "minus": k(0x0c),
	"equals": k(0x0d), "backspace": k(0x0e), "tab": k(0x0f), "q": k(0x10), "w": k(0x11), "e": k(0x12),
	"r": k(0x13), "t": k(0x14), "y": k(0x15), "u": k(0x16), "i": k(0x17), "o": k(0x18), "p": k(0x19),
	"lbracket": k(0x1a), "rbracket": k(0x1b), "enter": k(0x1c), "lctrl": k(0x1d), "a": k(0x1e),
	"s": k(0x1f), "d": k(0x20), "f": k(0x21), "g": k(0x22), "h": k(0x23), "j": k(0x24), "k": k(0x25),
	"l": k(0x26), "semicolon": k(0x27), "apostrophe": k(0x28), "grave": k(0x29), "lshift": k(0x2a),
	"backslash": k(0x2b), "z": k(0x2c), "x": k(0x2d), "c": k(0x2e), "v": k(0x2f), "b": k(0x30),
	"n": k(0x31), "m": k(0x32), "comma": k(0x33), "period": k(0x34), "slash": k(0x35),
	"rshift": k(0x36), "np_multiply": k(0x37), "lalt": k(0x38), "space": k(0x39), "capslock": k(0x3a),
	"f1": k(0x3b), "f2": k(0x3c), "f3": k(0x3d), "f4": k(0x3e), "f5": k(0x3f), "f6": k(0x40),
	"f7": k(0x41), "f8": k(0x42), "f9": k(0x43), "f10": k(0x44), "numlock": k(0x45),
	"scrolllock": k(0x46), "np_7": k(0x47), "np_8": k(0x48), "np_9": k(0x49), "np_subtract": k(0x4a),
	"np_4": k(0x4b), "np_5": k(0x4c), "np_6": k(0x4d), "np_add": k(0x4e), "np_1": k(0x4f),
	"np_2": k(0x50), "np_3": k(0x51), "np_0": k(0x52), "np_period": k(0x53), "f11": k(0x57),
	"f12": k(0x58), "f13": k(0x64), "f14": k(0x65), "f15": k(0x66), "f16": k(0x67), "f17": k(0x68),
	"f18": k(0x69), "f19": k(0x6a), "f20": k(0x6b), "f21": k(0x6c), "f22": k(0x6d), "f23": k(0x6e),
	"f24": k(0x76),
	"np_enter": ke(0x1c), "rctrl": ke(0x1d), "np_divide": ke(0x35), "print": ke(0x37), "ralt": ke(0x38),
	"home": ke(0x47), "up": ke(0x48), "pgup": ke(0x49), "left": ke(0x4b), "right": ke(0x4d), "end": ke(0x4f),
	"down": ke(0x50), "pgdn": ke(0x51), "insert": ke(0x52), "delete": ke(0x53),
	"lwin": ke(0x5b), "rwin": ke(0x5c),
}

// Aliases that turn up in Star Citizen profiles and hand-written layouts.
var aliases = map[string]string{
	"esc": "escape", "return": "enter", "ctrl": "lctrl", "shift": "lshift", "alt": "lalt",
	"pageup": "pgup", "pagedown": "pgdn", "del": "delete", "ins": "insert", "backquote": "grave",
	"tilde": "grave", "printscreen": "print",
}

func init() {
	for d := 0; d <= 9; d++ {
		aliases[fmt.Sprintf("num%d", d)] = fmt.Sprintf("np_%d", d)
	}
}

// Lookup returns the scancode for one key name.
func Lookup(name string) (Scan, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if a, ok := aliases[name]; ok {
		name = a
	}
	s, ok := table[name]
	return s, ok
}

// ParseCombo turns "lalt+n" into its scancodes, modifiers first and the main key last,
// in the order they're written.
func ParseCombo(combo string) ([]Scan, error) {
	var out []Scan
	for _, part := range strings.Split(combo, "+") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		s, ok := Lookup(part)
		if !ok {
			return nil, fmt.Errorf("unknown key %q in %q", part, combo)
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no keys in %q", combo)
	}
	return out, nil
}

// Names lists every key name a layout may use, sorted.
func Names() []string {
	out := make([]string, 0, len(table))
	for n := range table {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
