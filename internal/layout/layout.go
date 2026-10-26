// Package layout reads deck layouts. Two shapes exist and share one code path: tabbed
// grid layouts ({"pages": [{"name", "buttons": [...]}]}) and canvas layouts ({"canvas":
// [w, h], "chrome": [...], "panels": {id: {"title", "elements": [...]}}}). A button is any
// object with a string "id" and "keys", wherever it sits.
package layout

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/xattribution/tablet-deck-spooned/internal/keys"
)

// Button is one pressable control.
type Button struct {
	ID       string      `json:"id"`
	Label    string      `json:"label"`
	Keys     string      `json:"keys"`
	Mode     string      `json:"mode,omitempty"` // tap (default), hold, timed
	Ms       int         `json:"ms,omitempty"`
	VoiceMs  int         `json:"voiceMs,omitempty"`
	ScAction string      `json:"scAction,omitempty"`
	Icon     string      `json:"icon,omitempty"`
	Voice    []string    `json:"voice,omitempty"`
	NoVoice  bool        `json:"-"`
	State    *State      `json:"state,omitempty"`
	Panel    string      `json:"panel,omitempty"`  // panel id (canvas) or page name (grid)
	Global   bool        `json:"global,omitempty"` // for another app (Discord): sent whatever window is in front
	Scans    []keys.Scan `json:"-"`
}

// State makes a button light up while its group is on. Set forces on or off; without
// it a press flips the group.
type State struct {
	Group string `json:"group"`
	Set   *bool  `json:"set,omitempty"`
}

// Panel is a named group of buttons: a canvas panel or a grid page.
type Panel struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Buttons []string `json:"buttons"`
}

// Layout is a parsed layout plus the raw JSON the deck draws from.
type Layout struct {
	File    string             `json:"file"`
	Name    string             `json:"name"`
	Custom  bool               `json:"custom"`
	Canvas  []int              `json:"canvas,omitempty"`
	Raw     map[string]any     `json:"-"`
	Buttons map[string]*Button `json:"-"`
	Order   []string           `json:"-"` // button ids in layout order
	Panels  []Panel            `json:"panels"`
}

// Phone reports whether the layout is made for phones (canvas under 500 px tall).
func (l *Layout) Phone() bool { return len(l.Canvas) == 2 && l.Canvas[1] < 500 }

// Parse reads a layout, gives buttons with an scAction the player's own key for it
// (binds: action -> combo), and checks every key. It errors on an unknown key or a
// duplicate id, so the editor's saves get the same checks as loading.
func Parse(file string, data []byte, binds map[string]string) (*Layout, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	l := &Layout{File: file, Raw: raw, Buttons: map[string]*Button{}}
	l.Name, _ = raw["name"].(string)
	if l.Name == "" {
		l.Name = file
	}
	l.Custom, _ = raw["custom"].(bool)
	if c, ok := raw["canvas"].([]any); ok && len(c) == 2 {
		w, _ := c[0].(float64)
		h, _ := c[1].(float64)
		l.Canvas = []int{int(w), int(h)}
	}

	var walkErr error
	var walk func(node any, panel string)
	walk = func(node any, panel string) {
		if walkErr != nil {
			return
		}
		switch n := node.(type) {
		case []any:
			for _, v := range n {
				walk(v, panel)
			}
		case map[string]any:
			id, okID := n["id"].(string)
			ks, okKeys := n["keys"].(string)
			if okID && okKeys {
				b, err := buttonFrom(n, id, ks, binds)
				if err != nil {
					walkErr = fmt.Errorf("button %q: %w", id, err)
					return
				}
				if _, dup := l.Buttons[id]; dup {
					walkErr = fmt.Errorf("duplicate button id %q", id)
					return
				}
				b.Panel = panel
				l.Buttons[id] = b
				l.Order = append(l.Order, id)
			}
			// Sorted so the order is stable; panels and pages set their own name below.
			names := make([]string, 0, len(n))
			for k := range n {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, k := range names {
				walk(n[k], panel)
			}
		}
	}

	addPanel := func(id, title string, before int) {
		p := Panel{ID: id, Title: title}
		p.Buttons = append(p.Buttons, l.Order[before:]...)
		if len(p.Buttons) > 0 {
			l.Panels = append(l.Panels, p)
		}
	}
	if pages, ok := raw["pages"].([]any); ok {
		for _, pg := range pages {
			m, _ := pg.(map[string]any)
			name, _ := m["name"].(string)
			before := len(l.Order)
			walk(m["buttons"], name)
			addPanel(name, name, before)
		}
	}
	if before := len(l.Order); raw["chrome"] != nil {
		walk(raw["chrome"], "chrome")
		addPanel("chrome", "Top bar", before)
	}
	if panels, ok := raw["panels"].(map[string]any); ok {
		// The regions list panels in the order they're paged through; use it when present.
		var order []string
		if regions, ok := raw["regions"].([]any); ok {
			for _, r := range regions {
				rm, _ := r.(map[string]any)
				ps, _ := rm["panels"].([]any)
				for _, p := range ps {
					if s, ok := p.(string); ok {
						order = append(order, s)
					}
				}
			}
		}
		seen := map[string]bool{}
		for _, id := range order {
			seen[id] = true
		}
		var rest []string
		for id := range panels {
			if !seen[id] {
				rest = append(rest, id)
			}
		}
		sort.Strings(rest)
		for _, id := range append(order, rest...) {
			pm, ok := panels[id].(map[string]any)
			if !ok {
				continue
			}
			title, _ := pm["title"].(string)
			before := len(l.Order)
			walk(pm, id)
			addPanel(id, title, before)
		}
	}
	// Anything else (unusual layouts): one catch-all pass.
	for k, v := range raw {
		if k == "pages" || k == "chrome" || k == "panels" {
			continue
		}
		before := len(l.Order)
		walk(v, "")
		addPanel("other", "Other", before)
	}
	if walkErr != nil {
		return nil, fmt.Errorf("%s: %w", file, walkErr)
	}
	return l, nil
}

func buttonFrom(n map[string]any, id, ks string, binds map[string]string) (*Button, error) {
	b := &Button{ID: id, Keys: ks}
	b.Label, _ = n["label"].(string)
	if b.Label == "" {
		b.Label = id
	}
	b.Mode, _ = n["mode"].(string)
	b.ScAction, _ = n["scAction"].(string)
	b.Icon, _ = n["icon"].(string)
	if v, ok := n["ms"].(float64); ok {
		b.Ms = int(v)
	}
	if v, ok := n["voiceMs"].(float64); ok {
		b.VoiceMs = int(v)
	}
	switch v := n["voice"].(type) {
	case bool:
		b.NoVoice = !v
	case []any:
		for _, p := range v {
			if s, ok := p.(string); ok {
				b.Voice = append(b.Voice, s)
			}
		}
	}
	if s, ok := n["state"].(map[string]any); ok {
		if g, ok := s["group"].(string); ok && g != "" {
			st := &State{Group: g}
			if set, ok := s["set"].(bool); ok {
				st.Set = &set
			}
			b.State = st
		}
	}
	// The player's own key for this action wins, and the deck shows it too.
	cleared := false
	if b.ScAction != "" {
		if combo, ok := binds[b.ScAction]; ok {
			if combo == "" {
				cleared = true // the player removed this bind in the game
				n["keys"] = ""
			} else if _, err := keys.ParseCombo(combo); err == nil {
				b.Keys = combo
				n["keys"] = combo
			}
		}
	}
	scans, err := keys.ParseCombo(b.Keys)
	if err != nil {
		return nil, err
	}
	if cleared {
		scans = nil // pressing reports "unbound" instead of pressing the old default
	}
	b.Scans = scans
	return b, nil
}

// Spoken turns a label into a voice phrase: "Shields +" -> "shields up".
func Spoken(label string) string {
	s := strings.ToLower(strings.TrimSpace(label))
	switch {
	case strings.HasSuffix(s, "+"):
		s = strings.TrimSpace(strings.TrimSuffix(s, "+")) + " up"
	case strings.HasSuffix(s, "−"), strings.HasSuffix(s, "-"):
		s = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(s, "−"), "-")) + " down"
	}
	s = strings.ReplaceAll(s, "&", " and ")
	var b strings.Builder
	space := false
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '\''
		if ok {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		} else {
			space = true
		}
	}
	return b.String()
}

// VoicePhrases maps each phrase to a button id. A phrase that fits buttons with
// different keys is dropped rather than guessed. Emotes answer to "emote <name>" (and
// to "<name>" with short emotes on), so a stray "wait" doesn't set one off.
func (l *Layout) VoicePhrases(shortEmotes bool) (map[string]string, []string) {
	type pick struct {
		id, keys string
		bad      bool
	}
	by := map[string]*pick{}
	var warnings []string
	for _, id := range l.Order {
		b := l.Buttons[id]
		if b.NoVoice {
			continue
		}
		var phrases []string
		for _, p := range append([]string{b.Label}, b.Voice...) {
			if s := Spoken(p); s != "" {
				phrases = append(phrases, s)
			}
		}
		if strings.HasPrefix(b.ScAction, "emote_") {
			var pre []string
			for _, p := range phrases {
				if !strings.HasPrefix(p, "emote ") {
					p = "emote " + p
				}
				pre = append(pre, p)
			}
			if shortEmotes {
				phrases = append(pre, phrases...)
			} else {
				phrases = pre
			}
		}
		for _, p := range phrases {
			cur := by[p]
			switch {
			case cur == nil:
				by[p] = &pick{id: id, keys: b.Keys}
			case !cur.bad && cur.keys != b.Keys:
				cur.bad = true
				warnings = append(warnings, fmt.Sprintf("voice: %q fits both %s and %s, so it's left out", p, l.Buttons[cur.id].Label, b.Label))
			}
		}
	}
	out := map[string]string{}
	for p, v := range by {
		if !v.bad {
			out[p] = v.id
		}
	}
	return out, warnings
}
