package sc

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

type bindRow struct{ Map, Action, Input string }

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
// the game leaves alone by default, on every ordinary key. Combinations with Right
// Shift come last (Right Shift alone has a default bind). F4 is left out: Alt+F4
// closes windows.
func comboPool() []string {
	mods := []string{"ralt", "rctrl", "ralt+rctrl", "ralt+rshift", "rctrl+rshift", "ralt+rctrl+rshift"}
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
		if i != 4 {
			ks = append(ks, fmt.Sprintf("f%d", i))
		}
	}
	ks = append(ks, "comma", "period", "slash", "semicolon", "apostrophe", "lbracket", "rbracket", "backslash", "minus", "equals",
		"insert", "home", "pgup", "delete", "end", "pgdn", "np_add", "np_subtract", "np_multiply", "np_divide", "np_period",
		"up", "down", "left", "right")
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

// isKeyboard says whether a default bind is a key combination (not a mouse button,
// wheel or axis, which the deck can't press).
func isKeyboard(k string) bool {
	k = strings.ToLower(strings.TrimSpace(k))
	if k == "" {
		return false
	}
	for _, p := range strings.Split(k, "+") {
		if strings.HasPrefix(p, "mouse") || strings.HasPrefix(p, "mwheel") || strings.HasPrefix(p, "maxis") || strings.HasPrefix(p, "hmd") {
			return false
		}
	}
	return true
}

// BindAll builds a profile from the player's actionmaps.xml (current; may be empty)
// that keeps everything in it (joystick, gamepad and mouse binds, device options,
// every keyboard bind) and adds a free key for each action that has none. Actions the
// player cleared on purpose stay cleared. reserved are combinations something else
// already uses (the deck's own profile). It returns the profile and the new binds
// (action -> keys).
//
// Loading a profile replaces all of the game's binds with the profile's, which is why
// the player's own binds are carried over whole.
func BindAll(acts []Action, current []byte, name string, reserved ...string) ([]byte, map[string]string) {
	mine := KeyboardRebinds(bytes.NewReader(current))
	bound := map[string]bool{}
	used := map[string]bool{}
	for _, k := range reserved {
		if isKeyboard(k) {
			used[normCombo(k)] = true
		}
	}
	for _, a := range acts {
		if isKeyboard(a.Key) {
			bound[a.Name] = true
			used[normCombo(a.Key)] = true
		}
	}
	for _, r := range mine {
		k := strings.TrimSpace(r.Input[strings.IndexByte(r.Input, '_')+1:])
		bound[r.Action] = true // a key, or cleared on purpose: either way, leave it
		if k != "" {
			used[normCombo(k)] = true
		}
	}
	pool := comboPool()
	added := map[string]string{}
	var rows []bindRow
	next := 0
	for _, a := range acts {
		if bound[a.Name] {
			continue
		}
		k := added[a.Name] // the same action in two maps gets the same key
		if k == "" {
			for next < len(pool) && used[normCombo(pool[next])] {
				next++
			}
			if next >= len(pool) {
				continue
			}
			k = pool[next]
			used[normCombo(k)] = true
			added[a.Name] = k
		}
		rows = append(rows, bindRow{a.Map, a.Name, "kb1_" + k})
	}
	byMap := map[string][]bindRow{}
	var maps []string
	for _, e := range rows {
		if _, ok := byMap[e.Map]; !ok {
			maps = append(maps, e.Map)
		}
		byMap[e.Map] = append(byMap[e.Map], e)
	}

	esc := func(s string) string { var b bytes.Buffer; _ = xml.EscapeText(&b, []byte(s)); return b.String() }
	body, devices := profileBody(current)
	for _, m := range maps {
		body = addToMap(body, m, byMap[m], esc)
	}
	var w bytes.Buffer
	fmt.Fprintf(&w, "<ActionMaps version=\"1\" optionsVersion=\"2\" rebindVersion=\"2\" profileName=\"%s\">\n", esc(name))
	fmt.Fprintf(&w, " <CustomisationUIHeader label=\"%s\" description=\"\" image=\"\">\n  <devices>\n", esc(name))
	for _, d := range devices {
		fmt.Fprintf(&w, "   <%s instance=\"%s\"/>\n", d[0], d[1])
	}
	w.WriteString("  </devices>\n  <categories />\n </CustomisationUIHeader>\n")
	w.WriteString(body)
	w.WriteString("\n</ActionMaps>\n")
	return w.Bytes(), added
}

var (
	reProfiles = regexp.MustCompile(`(?s)<ActionProfiles\b[^>]*?(?:/>|>(.*?)</ActionProfiles>)`)
	reRootOpen = regexp.MustCompile(`(?s)^.*?<ActionMaps\b[^>]*>`)
	reHeader   = regexp.MustCompile(`(?s)<CustomisationUIHeader\b.*?</CustomisationUIHeader>|<CustomisationUIHeader\b[^>]*/>`)
	reOptions  = regexp.MustCompile(`<options\s+type="(\w+)"\s+instance="(\d+)"\s+Product=`)
	reXMLDecl  = regexp.MustCompile(`^\s*<\?xml[^>]*\?>`)
)

// profileBody returns what goes inside the profile's ActionMaps element: the player's
// default profile from actionmaps.xml (options, modifiers, action maps), or an
// exported profile's content; and the devices it uses.
func profileBody(current []byte) (string, [][2]string) {
	src := reXMLDecl.ReplaceAllString(string(current), "")
	body := ""
	if ms := reProfiles.FindAllStringSubmatch(src, -1); len(ms) > 0 {
		pick := ms[0]
		for _, m := range ms {
			if strings.Contains(m[0][:min(len(m[0]), 300)], `profileName="default"`) {
				pick = m
				break
			}
		}
		body = pick[1]
	} else if loc := reRootOpen.FindStringIndex(src); loc != nil {
		body = src[loc[1]:]
		if i := strings.LastIndex(body, "</ActionMaps>"); i >= 0 {
			body = body[:i]
		}
		body = reHeader.ReplaceAllString(body, "")
	}
	devices := [][2]string{{"keyboard", "1"}}
	seen := map[string]bool{"keyboard1": true}
	for _, m := range reOptions.FindAllStringSubmatch(body, -1) {
		if !seen[m[1]+m[2]] {
			seen[m[1]+m[2]] = true
			devices = append(devices, [2]string{m[1], m[2]})
		}
	}
	if !strings.Contains(body, "<modifiers") {
		body = " <modifiers />\n" + body
	}
	return strings.TrimRight(body, " \t\r\n"), devices
}

// addToMap puts keyboard binds into an action map in body, adding to an action's
// existing binds (a joystick button, say), or adding the action or the map.
func addToMap(body, mapName string, rows []bindRow, esc func(string) string) string {
	qm := regexp.QuoteMeta(esc(mapName))
	reMap := regexp.MustCompile(`(?s)<actionmap\s+name="` + qm + `"\s*(/>|>(.*?)</actionmap>)`)
	loc := reMap.FindStringSubmatchIndex(body)
	if loc == nil {
		var b strings.Builder
		fmt.Fprintf(&b, "\n  <actionmap name=\"%s\">\n", esc(mapName))
		for _, r := range rows {
			fmt.Fprintf(&b, "   <action name=\"%s\">\n    <rebind input=\"%s\"/>\n   </action>\n", esc(r.Action), esc(r.Input))
		}
		b.WriteString("  </actionmap>")
		return body + b.String()
	}
	inner := ""
	if loc[4] >= 0 && body[loc[2]:loc[3]] != "/>" {
		inner = body[loc[4]:loc[5]]
	}
	for _, r := range rows {
		qa := regexp.QuoteMeta(esc(r.Action))
		rebind := fmt.Sprintf("\n    <rebind input=\"%s\"/>", esc(r.Input))
		if m := regexp.MustCompile(`<action\s+name="` + qa + `"\s*/>`).FindStringIndex(inner); m != nil {
			inner = inner[:m[0]] + fmt.Sprintf("<action name=\"%s\">%s\n   </action>", esc(r.Action), rebind) + inner[m[1]:]
		} else if m := regexp.MustCompile(`<action\s+name="` + qa + `"\s*>`).FindStringIndex(inner); m != nil {
			inner = inner[:m[1]] + rebind + inner[m[1]:]
		} else {
			inner = strings.TrimRight(inner, " \t\r\n") + fmt.Sprintf("\n   <action name=\"%s\">%s\n   </action>\n  ", esc(r.Action), rebind)
		}
	}
	return body[:loc[0]] + fmt.Sprintf("<actionmap name=\"%s\">%s</actionmap>", esc(mapName), inner) + body[loc[1]:]
}
