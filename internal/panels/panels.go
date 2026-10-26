// Package panels defines a deck panel as data: a named collection of controls (game
// actions, bound keys, or other apps' hotkeys) and a layout of widgets that shows
// them. The built-in panels, the player's own panels and an agent's panels are all
// this one format, so any of them can be exported, shared and imported. The format
// is documented in docs/PANELS.md.
package panels

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/xattribution/tablet-deck-spooned/internal/keys"
)

// Format names the export format, so imports can tell what they were given.
const Format = "tabletdeck/1"

// Panel is one panel.
type Panel struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Code     string          `json:"code,omitempty"`     // 2-5 letters on the plate; made from the name when empty
	Category string          `json:"category,omitempty"` // flight nav eng def combat ind crew ui emerg
	Size     *Size           `json:"size,omitempty"`
	Actions  []Action        `json:"actions,omitempty"`  // controls this panel adds
	Controls []string        `json:"controls,omitempty"` // existing controls it shows (ids, labels or "sc:<game action>")
	Layout   json.RawMessage `json:"layout,omitempty"`   // widget tree; omitted means a grid of every control
	Source   string          `json:"source,omitempty"`   // built-in, folder, saved (set by the app)
}

// Size is how much room a panel asks for in a column.
type Size struct {
	Weight float64 `json:"weight,omitempty"` // share of a column's height
	Min    int     `json:"min,omitempty"`    // smallest height in px on a tablet
}

// Action is a control a panel brings: a key combo, a game action, or a hotkey for
// another app (Global, which presses whatever window is in front).
type Action struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Keys   string `json:"keys,omitempty"`   // "ralt+k"; or
	SC     string `json:"sc,omitempty"`     // a game action, pressed with the player's bind
	Mode   string `json:"mode,omitempty"`   // tap (default), hold, timed
	Ms     int    `json:"ms,omitempty"`     // for timed: how long to hold
	Icon   string `json:"icon,omitempty"`   // an icon name from the deck's set
	State  bool   `json:"state,omitempty"`  // lights up on and off with each press
	Danger bool   `json:"danger,omitempty"` // guarded: needs a deliberate press
	Global bool   `json:"global,omitempty"` // not for the game: sent to whatever is in front (Discord, OBS)
}

// Bundle is what export writes and import reads.
type Bundle struct {
	Format string          `json:"format"`
	Panels []Panel         `json:"panels,omitempty"`
	Views  json.RawMessage `json:"views,omitempty"`  // []config.View
	Layout json.RawMessage `json:"layout,omitempty"` // a deck's own arrangement, kept by the deck
}

var Categories = []string{"flight", "nav", "eng", "def", "combat", "ind", "crew", "ui", "emerg"}

var reID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,47}$`)

// Widgets are the layout node types, each with what its value holds.
var Widgets = map[string]string{
	"row": "children side by side", "col": "children stacked", "flip": "stacked in a tall panel, side by side in a wide one",
	"key": "a key (id)", "toggle": "a key that lights while on (id)", "lever": "the gear lever (id)",
	"fader": "a slider: {up, down, max?, min?, label, icon?, cat?, horizontal?}", "pair": "two keys as one switch ([id, id], names)",
	"dial": "a rotary selector ([ids], center?)", "shieldpad": "shield faces {front, back, left, right, top?, bottom?, reset?}",
	"guard": "a covered key (id)", "slide": "slide the handle to fire (id)", "grid": "keys in a grid ([ids] or \"rest\")",
	"modes": "a switch between layouts {options: [{name, value, layout}]}",
	"talk":  "the voice key with tap, hold and live modes", "route": "where voice goes: commands, auto or the agent",
	"transcript": "the last lines said to and by the agent", "radio": "push to talk with a channel switch {discord, game}",
}

// Clean checks a panel and fills what can be filled. It returns a description of the
// first problem.
func Clean(p *Panel) error {
	p.ID = strings.TrimSpace(strings.ToLower(p.ID))
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" || len(p.Name) > 40 {
		return fmt.Errorf("a panel needs a name, up to 40 characters")
	}
	if p.ID == "" {
		p.ID = Slug(p.Name)
	}
	if !reID.MatchString(p.ID) {
		return fmt.Errorf("panel id %q: use lower case letters, digits, dots and dashes", p.ID)
	}
	if p.Code == "" {
		p.Code = codeOf(p.Name)
	}
	p.Code = strings.ToUpper(strings.TrimSpace(p.Code))
	if len(p.Code) > 5 {
		p.Code = p.Code[:5]
	}
	if p.Category == "" {
		p.Category = "ui"
	}
	ok := false
	for _, c := range Categories {
		ok = ok || c == p.Category
	}
	if !ok {
		return fmt.Errorf("category %q: one of %s", p.Category, strings.Join(Categories, ", "))
	}
	seen := map[string]bool{}
	for i := range p.Actions {
		a := &p.Actions[i]
		a.ID = strings.TrimSpace(strings.ToLower(a.ID))
		if a.ID == "" {
			a.ID = Slug(a.Label)
		}
		if !reID.MatchString(a.ID) || seen[a.ID] {
			return fmt.Errorf("action %q: needs a unique id of lower case letters, digits, dots and dashes", a.ID)
		}
		seen[a.ID] = true
		if strings.TrimSpace(a.Label) == "" {
			return fmt.Errorf("action %q needs a label", a.ID)
		}
		a.SC = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(a.SC), "sc:"), "sc.")
		a.Keys = strings.ToLower(strings.TrimSpace(a.Keys))
		if a.Keys == "" && a.SC == "" {
			return fmt.Errorf("action %q needs keys (\"ralt+k\") or a game action (sc)", a.ID)
		}
		if a.Keys != "" {
			if _, err := keys.ParseCombo(a.Keys); err != nil {
				return fmt.Errorf("action %q: %v", a.ID, err)
			}
		}
		switch a.Mode {
		case "", "tap", "hold", "timed":
		default:
			return fmt.Errorf("action %q: mode is tap, hold or timed", a.ID)
		}
	}
	for i, c := range p.Controls {
		p.Controls[i] = strings.Replace(strings.TrimSpace(c), "sc:", "sc.", 1)
	}
	if len(p.Layout) > 0 && string(p.Layout) != "null" {
		var node any
		if err := json.Unmarshal(p.Layout, &node); err != nil {
			return fmt.Errorf("layout isn't valid JSON: %v", err)
		}
		if err := checkNode(node, "layout"); err != nil {
			return err
		}
		b, _ := json.Marshal(normalize(node))
		p.Layout = b
	} else {
		p.Layout = nil
	}
	if p.Size != nil && (p.Size.Weight < 0 || p.Size.Min < 0) {
		return fmt.Errorf("size can't be negative")
	}
	return nil
}

func checkNode(n any, where string) error {
	m, ok := n.(map[string]any)
	if !ok {
		return fmt.Errorf("%s: each layout node is an object like {\"key\": \"landing-gear\"}", where)
	}
	kind := ""
	for k := range m {
		if _, ok := Widgets[k]; ok {
			if kind != "" {
				return fmt.Errorf("%s: a node is one widget, not %s and %s", where, kind, k)
			}
			kind = k
		}
	}
	if kind == "" {
		return fmt.Errorf("%s: no widget (one of %s)", where, strings.Join(WidgetNames(), ", "))
	}
	switch kind {
	case "row", "col", "flip":
		kids, ok := m[kind].([]any)
		if !ok {
			return fmt.Errorf("%s.%s: a list of nodes", where, kind)
		}
		for i, k := range kids {
			if err := checkNode(k, fmt.Sprintf("%s.%s[%d]", where, kind, i)); err != nil {
				return err
			}
		}
	case "modes":
		opts, _ := m["options"].([]any)
		if len(opts) < 2 {
			return fmt.Errorf("%s.modes: options needs two or more {name, value, layout}", where)
		}
		for i, o := range opts {
			om, _ := o.(map[string]any)
			if om == nil || om["layout"] == nil {
				return fmt.Errorf("%s.modes.options[%d]: needs a layout", where, i)
			}
			if err := checkNode(om["layout"], fmt.Sprintf("%s.modes.options[%d]", where, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// normalize writes "sc:<action>" references as the catalog's "sc.<action>" ids.
func normalize(n any) any {
	switch v := n.(type) {
	case map[string]any:
		for k, x := range v {
			v[k] = normalize(x)
		}
	case []any:
		for i, x := range v {
			v[i] = normalize(x)
		}
	case string:
		if strings.HasPrefix(v, "sc:") {
			return "sc." + v[3:]
		}
	}
	return n
}

// Refs lists every string in a layout, so the app can include the controls it names.
func Refs(layout json.RawMessage) []string {
	var node any
	if json.Unmarshal(layout, &node) != nil {
		return nil
	}
	var out []string
	var walk func(any)
	walk = func(n any) {
		switch v := n.(type) {
		case map[string]any:
			for _, x := range v {
				walk(x)
			}
		case []any:
			for _, x := range v {
				walk(x)
			}
		case string:
			out = append(out, v)
		}
	}
	walk(node)
	return out
}

// ControlRefs lists the controls a layout places (not its labels, icons or options).
func ControlRefs(layout json.RawMessage) []string {
	var node any
	if json.Unmarshal(layout, &node) != nil {
		return nil
	}
	var out []string
	add := func(v any) {
		switch x := v.(type) {
		case string:
			if x != "rest" {
				out = append(out, x)
			}
		case []any:
			for _, y := range x {
				if s, ok := y.(string); ok {
					out = append(out, s)
				}
			}
		case map[string]any:
			for _, k := range []string{"up", "down", "max", "min", "front", "back", "left", "right", "top", "bottom", "reset", "discord", "game"} {
				if s, ok := x[k].(string); ok {
					out = append(out, s)
				}
			}
		}
	}
	var walk func(any)
	walk = func(n any) {
		m, ok := n.(map[string]any)
		if !ok {
			return
		}
		for k, v := range m {
			switch k {
			case "row", "col", "flip":
				if kids, ok := v.([]any); ok {
					for _, c := range kids {
						walk(c)
					}
				}
			case "options":
				if opts, ok := v.([]any); ok {
					for _, o := range opts {
						if om, ok := o.(map[string]any); ok {
							walk(om["layout"])
						}
					}
				}
			case "key", "toggle", "lever", "guard", "slide", "pair", "dial", "center", "grid", "fader", "shieldpad", "radio":
				add(v)
			}
		}
	}
	walk(node)
	return out
}

// WidgetNames lists the layout node types.
func WidgetNames() []string {
	var out []string
	for k := range Widgets {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Slug makes an id from a name.
func Slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func codeOf(name string) string {
	w := strings.Fields(strings.ToUpper(regexp.MustCompile(`[^A-Za-z0-9 ]`).ReplaceAllString(name, " ")))
	switch {
	case len(w) == 0:
		return "PNL"
	case len(w) == 1:
		if len(w[0]) > 4 {
			return w[0][:3]
		}
		return w[0]
	default:
		s := ""
		for _, x := range w[:min(4, len(w))] {
			s += x[:1]
		}
		return s
	}
}
