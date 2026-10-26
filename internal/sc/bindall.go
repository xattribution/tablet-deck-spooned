package sc

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Rebind is one keyboard bind as written in actionmaps.xml.
type Rebind struct{ Map, Action, Input string }

// KeyboardRebinds reads the keyboard binds (with their action maps) from an
// actionmaps.xml or a profile.
func KeyboardRebinds(r io.Reader) []Rebind {
	dec := xml.NewDecoder(r)
	dec.Strict = false
	var out []Rebind
	var m, action string
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		if t, ok := tok.(xml.StartElement); ok {
			switch t.Name.Local {
			case "actionmap":
				m = attr(t, "name")
			case "action":
				action = attr(t, "name")
			case "rebind":
				if in := attr(t, "input"); strings.HasPrefix(in, "kb") && m != "" && action != "" {
					out = append(out, Rebind{m, action, in})
				}
			}
		}
	}
}

// Free key combinations, in the order they're handed out: right-hand modifiers that
// the game leaves alone by default, on every ordinary key.
func comboPool() []string {
	mods := []string{"ralt", "rctrl", "ralt+rshift", "rctrl+rshift", "ralt+rctrl", "ralt+rctrl+rshift"}
	var ks []string
	for c := 'a'; c <= 'z'; c++ {
		ks = append(ks, string(c))
	}
	for c := '0'; c <= '9'; c++ {
		ks = append(ks, string(c))
	}
	for i := 0; i <= 9; i++ {
		ks = append(ks, fmt.Sprintf("np_%d", i))
	}
	for i := 1; i <= 12; i++ {
		ks = append(ks, fmt.Sprintf("f%d", i))
	}
	ks = append(ks, "comma", "period", "slash", "semicolon", "apostrophe", "lbracket", "rbracket", "backslash", "minus", "equals",
		"insert", "home", "pgup", "delete", "end", "pgdn", "np_add", "np_subtract", "np_multiply", "np_divide")
	var out []string
	for _, m := range mods {
		for _, k := range ks {
			out = append(out, m+"+"+k)
		}
	}
	return out
}

func normCombo(s string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "+")
	sort.Strings(parts)
	return strings.Join(parts, "+")
}

// BindAll builds a keyboard profile that keeps every keyboard bind in current (the
// player's actionmaps.xml, may be empty) and gives each action without a key a free
// combination. It returns the profile and the new binds (action -> keys).
//
// Loading a profile in the game replaces the keyboard binds with the profile's, which
// is why the player's own binds are written into it.
func BindAll(acts []Action, current []byte, name string) ([]byte, map[string]string) {
	mine := KeyboardRebinds(bytes.NewReader(current))
	bound := map[string]string{} // action -> keys
	used := map[string]bool{}
	for _, a := range acts {
		if k := strings.TrimSpace(a.Key); k != "" {
			bound[a.Name] = k
		}
	}
	for _, r := range mine {
		k := strings.TrimSpace(r.Input[strings.IndexByte(r.Input, '_')+1:])
		if k == "" {
			delete(bound, r.Action) // the player cleared it
			continue
		}
		bound[r.Action] = k
	}
	for _, k := range bound {
		used[normCombo(k)] = true
	}
	pool := comboPool()
	added := map[string]string{}
	type entry struct{ Map, Action, Input string }
	var rows []entry
	next := 0
	for _, a := range acts {
		if bound[a.Name] != "" {
			continue
		}
		for next < len(pool) && used[normCombo(pool[next])] {
			next++
		}
		if next >= len(pool) {
			break
		}
		k := pool[next]
		used[normCombo(k)] = true
		added[a.Name] = k
		rows = append(rows, entry{a.Map, a.Name, "kb1_" + k})
	}
	// The player's rows first (minus the ones cleared and now given a key), then ours.
	var keep []entry
	for _, r := range mine {
		if added[r.Action] == "" {
			keep = append(keep, entry{r.Map, r.Action, r.Input})
		}
	}
	rows = append(keep, rows...)
	// Group by action map, in the order the maps first appear.
	var maps []string
	byMap := map[string][]entry{}
	for _, e := range rows {
		if _, ok := byMap[e.Map]; !ok {
			maps = append(maps, e.Map)
		}
		byMap[e.Map] = append(byMap[e.Map], e)
	}
	var w bytes.Buffer
	esc := func(s string) string { var b bytes.Buffer; _ = xml.EscapeText(&b, []byte(s)); return b.String() }
	fmt.Fprintf(&w, "<ActionMaps version=\"1\" optionsVersion=\"2\" rebindVersion=\"2\" profileName=%q>\n", name)
	fmt.Fprintf(&w, " <CustomisationUIHeader label=%q description=\"\" image=\"\">\n  <devices>\n   <keyboard instance=\"1\"/>\n  </devices>\n  <categories />\n </CustomisationUIHeader>\n <modifiers />\n", name)
	for _, m := range maps {
		fmt.Fprintf(&w, " <actionmap name=\"%s\">\n", esc(m))
		for _, e := range byMap[m] {
			fmt.Fprintf(&w, "  <action name=\"%s\">\n   <rebind input=\"%s\"/>\n  </action>\n", esc(e.Action), esc(e.Input))
		}
		w.WriteString(" </actionmap>\n")
	}
	w.WriteString("</ActionMaps>\n")
	return w.Bytes(), added
}
