package app

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/xattribution/tablet-deck-spooned/internal/config"
	"github.com/xattribution/tablet-deck-spooned/internal/keys"
	"github.com/xattribution/tablet-deck-spooned/internal/layout"
	"github.com/xattribution/tablet-deck-spooned/internal/panels"
)

// The catalog is every control the deck knows, each once, with a stable id, a group
// (ship system) and tags, gathered from all layouts. Views are lists of groups (or of
// single controls) drawn from it, so any device can show any arrangement, and an
// agent can find and press anything by name.

// Group is a ship system: the unit views are built from.
type Group struct {
	ID       string        `json:"id"`
	Code     string        `json:"code"`
	Name     string        `json:"name"`
	Category string        `json:"category,omitempty"`
	Game     bool          `json:"game,omitempty"` // one of the game's own menus
	Order    int           `json:"-"`
	Panel    *panels.Panel `json:"-"`
}

var groups = []Group{
	{ID: "power", Code: "PWR", Name: "Power", Order: 0}, {ID: "landing", Code: "LND", Name: "Landing", Order: 1},
	{ID: "flight", Code: "FLT", Name: "Flight", Order: 2}, {ID: "quantum", Code: "QT", Name: "Quantum", Order: 3},
	{ID: "modes", Code: "MOD", Name: "Master modes", Order: 4}, {ID: "shields", Code: "SHD", Name: "Shields", Order: 5},
	{ID: "targeting", Code: "TGT", Name: "Targeting", Order: 6}, {ID: "combat", Code: "CMB", Name: "Weapons and countermeasures", Order: 7},
	{ID: "emergency", Code: "EMR", Name: "Emergency", Order: 8}, {ID: "crew", Code: "CRW", Name: "Crew and doors", Order: 9},
	{ID: "systems", Code: "SYS", Name: "Systems", Order: 10}, {ID: "mining", Code: "MIN", Name: "Mining", Order: 11},
	{ID: "salvage", Code: "SAL", Name: "Salvage", Order: 12}, {ID: "on-foot", Code: "FOOT", Name: "On foot", Order: 13},
	{ID: "emotes", Code: "EMO", Name: "Emotes", Order: 14}, {ID: "interface", Code: "UI", Name: "Interface", Order: 15},
	{ID: "comms", Code: "COM", Name: "Comms", Order: 16},
}

func groupByID(id string) (Group, bool) {
	for _, g := range groups {
		if g.ID == id {
			return g, true
		}
	}
	return Group{}, false
}

// Control is one catalog entry.
type Control struct {
	ID       string         `json:"id"`
	Label    string         `json:"label"`
	Keys     string         `json:"keys"`
	Mode     string         `json:"mode,omitempty"`
	Group    string         `json:"group"`
	Tags     []string       `json:"tags"`
	State    *layout.State  `json:"state,omitempty"`
	Danger   bool           `json:"danger,omitempty"`
	ScAction string         `json:"scAction,omitempty"`
	Icon     string         `json:"icon,omitempty"`
	Voice    []string       `json:"voice,omitempty"`
	Desc     string         `json:"desc,omitempty"`
	Unbound  bool           `json:"unbound,omitempty"` // a game action without a key yet
	Global   bool           `json:"global,omitempty"`  // a hotkey for another app
	Button   *layout.Button `json:"-"`
}

// panelGroup maps layout panels and pages to groups.
var panelGroup = map[string]string{
	"power-landing": "power", "power": "power", "flight-assist": "flight", "flight": "flight", "landing": "landing",
	"qt-systems": "modes", "systems": "modes", "modes": "modes", "shields": "shields", "targets": "targeting",
	"combat": "combat", "emergency": "emergency", "crew": "crew", "mining": "mining", "prospector": "mining",
	"prospector-2": "mining", "salvage": "salvage", "vulture": "salvage", "vulture-2": "salvage", "on-foot": "on-foot",
	"on foot": "on-foot", "emotes": "emotes", "emotes-1": "emotes", "emotes-2": "emotes", "chrome": "interface",
}

// Labels that belong to one system wherever they sit.
var labelGroup = []struct {
	re    *regexp.Regexp
	group string
}{
	{regexp.MustCompile(`(?i)self-?d|eject`), "emergency"},
	{regexp.MustCompile(`(?i)^(flight|scanning|scan|salvage|mining|guns|missiles?|quantum|scm/nav)( mode)?$`), "modes"},
	{regexp.MustCompile(`(?i)quantum jump|engage quantum|^quantum$`), "quantum"},
	{regexp.MustCompile(`(?i)landing gear|vtol|request (landing|docking)|auto land|docking camera|^atc$`), "landing"},
	{regexp.MustCompile(`(?i)flight ready|power on/off|^pwr$|^wep$|^thr$|^shld$`), "power"},
	{regexp.MustCompile(`(?i)^lights$|exit seat`), "systems"},
	{regexp.MustCompile(`(?i)look behind`), "flight"},
	{regexp.MustCompile(`(?i)star map|mobiglas|contacts|^chat$|inventory|^esc$`), "interface"},
	{regexp.MustCompile(`(?i)^power: `), "power"},
}

var reDanger = regexp.MustCompile(`(?i)self-?d|eject|jettison`)
var reParen = regexp.MustCompile(`\s*\([^)]*\)`)

// Words that don't tell controls apart within their group.
var noiseWords = map[string]bool{"shields": true, "shield": true, "target": true, "targets": true, "all": true, "remote": true,
	"toggle": true, "mode": true, "master": true, "power": true, "the": true, "hold": true, "cinematic": true}

func canonicalLabel(label string) string {
	var out []string
	for _, w := range strings.Split(slug(reParen.ReplaceAllString(label, "")), "-") {
		if !noiseWords[w] && w != "" {
			out = append(out, w)
		}
	}
	return strings.Join(out, "-")
}

func slug(s string) string {
	s = strings.ToLower(s)
	s = strings.NewReplacer("+", " up", "−", " down", "&", " and ", "/", " ").Replace(s)
	var b strings.Builder
	dash := false
	for _, r := range s {
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

type catalog struct {
	mu       sync.Mutex
	stamp    string
	controls []*Control
	byID     map[string]*Control
	ofButton map[string]*Control // "layout file|button id" -> its control
}

// Catalog returns every control, built from all layouts with the current keybinds.
func (a *App) Catalog() ([]*Control, map[string]*Control) {
	a.cat.mu.Lock()
	defer a.cat.mu.Unlock()
	stamp := fmt.Sprintf("%s%s|%d|%s|%s|%d", a.Binds().stamp, strings.Join(a.LayoutFiles(), ","), a.panelGen.Load(),
		a.folderStamp(), a.GameActions().Version, len(a.Store.Config().Rebinds))
	if a.cat.byID != nil && a.cat.stamp == stamp {
		return a.cat.controls, a.cat.byID
	}
	type cand struct {
		b      *layout.Button
		group  string
		layout string
	}
	merged := map[string][]cand{} // dedupe key -> candidates
	var order []string
	// The main tablet layout first, so its labels and ids win ties.
	files := a.LayoutFiles()
	sort.SliceStable(files, func(i, j int) bool { return files[i] == "layouts/hud.json" && files[j] != "layouts/hud.json" })
	for _, f := range files {
		l, err := a.LayoutFile(f)
		if err != nil {
			continue
		}
		for _, id := range l.Order {
			b := l.Buttons[id]
			g := panelGroup[strings.ToLower(b.Panel)]
			for _, lg := range labelGroup {
				if lg.re.MatchString(b.Label) {
					g = lg.group
					break
				}
			}
			if g == "" {
				g = "systems"
			}
			// The same control appears in many layouts under slightly different labels
			// ("Shields Back" and "Back", "Close All Doors" and "Close Doors"); the same
			// keys can also mean different controls (Landing Gear, and Auto Land held).
			key := g + "|" + b.Keys + "|" + canonicalLabel(b.Label)
			if _, ok := merged[key]; !ok {
				order = append(order, key)
			}
			merged[key] = append(merged[key], cand{b, g, f})
		}
	}
	// Second pass: entries that press exactly the same keys the same way, and never
	// appear in the same layout, are one control under two names ("Missiles" and
	// "Missile Mode"). Two buttons with the same keys in one layout were meant apart.
	type ent struct {
		key     string
		layouts map[string]bool
		sig     string
	}
	var ents []*ent
	for _, key := range order {
		e := &ent{key: key, layouts: map[string]bool{}}
		b := merged[key][0].b
		e.sig = fmt.Sprintf("%s|%s|%d", b.Keys, b.Mode, b.Ms)
		for _, c := range merged[key] {
			e.layouts[c.layout] = true
		}
		ents = append(ents, e)
	}
	var kept []string
	for i, e := range ents {
		if e == nil {
			continue
		}
		for j := i + 1; j < len(ents); j++ {
			o := ents[j]
			if o == nil || o.sig != e.sig {
				continue
			}
			overlap := false
			for l := range o.layouts {
				if e.layouts[l] {
					overlap = true
					break
				}
			}
			if overlap {
				continue
			}
			for l := range o.layouts {
				e.layouts[l] = true
			}
			merged[e.key] = append(merged[e.key], merged[o.key]...)
			ents[j] = nil
		}
		kept = append(kept, e.key)
	}
	order = kept

	var out []*Control
	byID := map[string]*Control{}
	ofButton := map[string]*Control{}
	for _, key := range order {
		cs := merged[key]
		// The clearest label: no "(cinematic)" asides, and the longest ("Shields Back"
		// over "Back", which only makes sense inside its panel).
		best := cs[0]
		bestLabel := reParen.ReplaceAllString(best.b.Label, "")
		for _, c := range cs[1:] {
			l := reParen.ReplaceAllString(c.b.Label, "")
			if (len(bestLabel) <= 4 && len(l) > len(bestLabel)) || (len(l) > len(bestLabel) && strings.Contains(strings.ToLower(l), strings.ToLower(bestLabel))) {
				best, bestLabel = c, l
			}
		}
		c := &Control{Label: bestLabel, Keys: best.b.Keys, Group: best.group, ScAction: best.b.ScAction, Button: best.b}
		tags := map[string]bool{best.group: true}
		voice := map[string]bool{}
		for _, x := range cs {
			if x.b.Mode == "hold" || (c.Mode == "" && x.b.Mode != "") {
				c.Mode = x.b.Mode
			}
			if c.State == nil && x.b.State != nil {
				c.State = x.b.State
			}
			if c.Icon == "" && x.b.Icon != "" {
				c.Icon = x.b.Icon
			}
			if x.b.Panel != "" {
				tags[slug(x.b.Panel)] = true
			}
			for _, v := range x.b.Voice {
				voice[v] = true
			}
		}
		for _, w := range strings.Fields(strings.ToLower(bestLabel)) {
			if len(w) > 2 {
				tags[slug(w)] = true
			}
		}
		if c.State != nil {
			tags["toggle"] = true
		}
		if c.Mode == "hold" {
			tags["hold"] = true
		}
		c.Danger = reDanger.MatchString(bestLabel)
		for t := range tags {
			if t != "" {
				c.Tags = append(c.Tags, t)
			}
		}
		sort.Strings(c.Tags)
		for v := range voice {
			c.Voice = append(c.Voice, v)
		}
		sort.Strings(c.Voice)
		id := slug(bestLabel)
		if id == "" {
			id = "control"
		}
		if _, taken := byID[id]; taken {
			id = best.group + "-" + id
		}
		for n := 2; byID[id] != nil; n++ {
			id = fmt.Sprintf("%s-%d", slug(bestLabel), n)
		}
		c.ID = id
		// The button the engine presses: the control's own id, keys and state.
		b := *best.b
		b.ID, b.Label, b.Mode, b.State = c.ID, c.Label, c.Mode, c.State
		if sc, err := keys.ParseCombo(b.Keys); err == nil {
			b.Scans = sc
		}
		c.Button = &b
		byID[id] = c
		for _, x := range cs {
			ofButton[x.layout+"|"+x.b.ID] = c
		}
		out = append(out, c)
	}
	// Rebinds made here apply to the ship systems' controls too.
	for id, k := range a.Store.Config().Rebinds {
		if c := byID[id]; c != nil {
			c.Keys = k
			c.Button = buttonFor(c, c.Button.Mode, c.Button.Ms, c.Button.Global)
		}
	}
	out = a.extendCatalog(out, byID)
	rank := map[string]int{}
	for _, g := range a.Groups() {
		rank[g.ID] = g.Order
	}
	sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Group] < rank[out[j].Group] })
	a.cat.controls, a.cat.byID, a.cat.ofButton, a.cat.stamp = out, byID, ofButton, stamp
	return out, byID
}

// ControlOf returns the catalog control a layout's button belongs to.
func (a *App) ControlOf(l *layout.Layout, buttonID string) *Control {
	a.Catalog()
	a.cat.mu.Lock()
	defer a.cat.mu.Unlock()
	return a.cat.ofButton[l.File+"|"+buttonID]
}

// ButtonsFor maps catalog groups and controls onto a layout's button ids, for share
// links (which work on a layout's buttons).
func (a *App) ButtonsFor(l *layout.Layout, groupIDs, controlRefs []string) []string {
	wantG := map[string]bool{}
	for _, g := range groupIDs {
		wantG[g] = true
	}
	wantC := map[string]bool{}
	for _, ref := range controlRefs {
		if c := a.FindControl(ref); c != nil {
			wantC[c.ID] = true
		}
	}
	var out []string
	for _, id := range l.Order {
		if c := a.ControlOf(l, id); c != nil && (wantG[c.Group] || wantC[c.ID]) {
			out = append(out, id)
		}
	}
	return out
}

// GuestGroups lays a share link's layout out by group, marking what the guest may press.
func (a *App) GuestGroups(l *layout.Layout, g *config.Grant, states map[string]bool) []map[string]any {
	type entry struct {
		group    Group
		controls []map[string]any
	}
	by := map[string]*entry{}
	seen := map[string]bool{}
	for _, id := range l.Order {
		b := l.Buttons[id]
		c := a.ControlOf(l, id)
		if c == nil || seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		e := by[c.Group]
		if e == nil {
			gr, _ := a.GroupByID(c.Group)
			e = &entry{group: gr}
			by[c.Group] = e
		}
		m := map[string]any{"id": b.ID, "cid": c.ID, "icon": c.Icon, "label": c.Label, "keys": c.Keys, "mode": c.Mode, "locked": !grantCovers(g, b), "danger": c.Danger}
		if c.State != nil {
			m["state"] = c.State
		}
		e.controls = append(e.controls, m)
	}
	var out []map[string]any
	for _, gr := range a.Groups() {
		if e := by[gr.ID]; e != nil {
			m := map[string]any{"id": gr.ID, "code": gr.Code, "name": gr.Name, "category": gr.Category, "controls": e.controls}
			if gr.Panel != nil && len(gr.Panel.Layout) > 0 {
				m["layout"], m["size"] = gr.Panel.Layout, gr.Panel.Size
			}
			out = append(out, m)
		}
	}
	// Groups with something shared come first.
	sort.SliceStable(out, func(i, j int) bool { return sharedIn(out[i]) && !sharedIn(out[j]) })
	return out
}

func sharedIn(g map[string]any) bool {
	for _, c := range g["controls"].([]map[string]any) {
		if c["locked"] == false {
			return true
		}
	}
	return false
}

// FindControl finds a control by id, label, Star Citizen action or voice phrase.
func (a *App) FindControl(ref string) *Control {
	all, byID := a.Catalog()
	if c := byID[ref]; c != nil {
		return c
	}
	s := slug(ref)
	if c := byID[s]; c != nil {
		return c
	}
	for _, c := range all {
		if strings.EqualFold(c.Label, ref) || c.ScAction == ref {
			return c
		}
	}
	for _, c := range all {
		for _, v := range c.Voice {
			if strings.EqualFold(v, ref) {
				return c
			}
		}
	}
	return nil
}

// SearchControls finds controls whose label, tags, group or voice phrases match every
// word of the query.
func (a *App) SearchControls(query, group string, limit int) []*Control {
	all, _ := a.Catalog()
	words := strings.Fields(strings.ToLower(query))
	var out []*Control
	for _, c := range all {
		if group != "" && c.Group != group {
			continue
		}
		hay := strings.ToLower(c.ID + " " + c.Label + " " + c.Group + " " + strings.Join(c.Tags, " ") + " " + strings.Join(c.Voice, " ") + " " + c.ScAction)
		ok := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, c)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out
}

// ---------- views ----------

// presets are the built-in views, by group.
var presets = []config.View{
	{ID: "flight", Name: "Flight", Groups: []string{"power", "landing", "flight", "quantum", "modes", "systems"}},
	{ID: "combat", Name: "Combat", Groups: []string{"targeting", "combat", "shields", "power", "modes", "emergency"}},
	{ID: "mining", Name: "Mining", Groups: []string{"mining", "modes", "power", "landing", "quantum"}},
	{ID: "salvage", Name: "Salvage", Groups: []string{"salvage", "modes", "power", "landing"}},
	{ID: "crew", Name: "Crew", Groups: []string{"comms", "crew", "systems", "landing", "interface"}},
	{ID: "on-foot", Name: "On foot", Groups: []string{"on-foot", "comms", "interface", "emotes"}},
}

// Views lists the presets and the player's own views.
func (a *App) Views() []config.View {
	out := append([]config.View(nil), presets...)
	for i := range out {
		out[i].Preset = true
	}
	return append(out, a.Store.State().Views...)
}

// ViewByID finds a view by id or name.
func (a *App) ViewByID(ref string) (config.View, bool) {
	for _, v := range a.Views() {
		if v.ID == ref || strings.EqualFold(v.Name, ref) || v.ID == slug(ref) {
			return v, true
		}
	}
	return config.View{}, false
}

// ResolvedGroup is a view's group with its controls, ready to draw.
type ResolvedGroup struct {
	ID       string          `json:"id"`
	Code     string          `json:"code"`
	Name     string          `json:"name"`
	Category string          `json:"category,omitempty"`
	Layout   json.RawMessage `json:"layout,omitempty"` // the panel's widget tree
	Size     *panels.Size    `json:"size,omitempty"`
	Controls []*Control      `json:"controls"`
}

// Resolve turns a view into groups of controls.
func (a *App) Resolve(v config.View) []ResolvedGroup {
	all, byID := a.Catalog()
	var out []ResolvedGroup
	for _, gid := range v.Groups {
		g, ok := a.GroupByID(gid)
		if !ok {
			continue
		}
		rg := ResolvedGroup{ID: g.ID, Code: g.Code, Name: g.Name, Category: g.Category, Controls: a.controlsOf(g, all, byID)}
		if g.Panel != nil {
			rg.Layout, rg.Size = g.Panel.Layout, g.Panel.Size
		}
		if len(rg.Controls) > 0 {
			out = append(out, rg)
		}
	}
	for _, s := range v.Sections {
		rg := ResolvedGroup{ID: slug(s.Name), Code: strings.ToUpper(firstN(slug(s.Name), 3)), Name: s.Name}
		for _, id := range s.Controls {
			if c := byID[id]; c != nil {
				rg.Controls = append(rg.Controls, c)
			} else if c := a.FindControl(id); c != nil {
				rg.Controls = append(rg.Controls, c)
			}
		}
		if len(rg.Controls) > 0 {
			out = append(out, rg)
		}
	}
	return out
}

func firstN(s string, n int) string {
	s = strings.ReplaceAll(s, "-", "")
	if len(s) > n {
		return s[:n]
	}
	return s
}

// SaveView stores (or replaces, by id) one of the player's views.
func (a *App) SaveView(v config.View) (config.View, error) {
	v.Name = strings.TrimSpace(v.Name)
	if v.Name == "" || len(v.Name) > 40 {
		return v, fmt.Errorf("a view needs a name, up to 40 characters")
	}
	if v.ID == "" {
		v.ID = slug(v.Name)
	}
	for _, p := range presets {
		if p.ID == v.ID {
			v.ID += "-mine"
		}
	}
	for _, g := range v.Groups {
		if _, ok := a.GroupByID(g); !ok {
			return v, fmt.Errorf("no group or panel %q (list_groups shows them)", g)
		}
	}
	v.Preset = false
	err := a.Store.UpdateState(func(st *config.State) {
		for i := range st.Views {
			if st.Views[i].ID == v.ID {
				st.Views[i] = v
				return
			}
		}
		st.Views = append(st.Views, v)
	})
	a.broadcastOwners(map[string]any{"t": "views", "views": a.Views()})
	return v, err
}

// DeleteView removes one of the player's views.
func (a *App) DeleteView(id string) bool {
	found := false
	_ = a.Store.UpdateState(func(st *config.State) {
		kept := st.Views[:0]
		for _, v := range st.Views {
			if v.ID == id {
				found = true
				continue
			}
			kept = append(kept, v)
		}
		st.Views = kept
	})
	a.broadcastOwners(map[string]any{"t": "views", "views": a.Views()})
	return found
}

// ShowView puts a view on decks: one device (by id or name), or every owner deck
// when device is "" or "all". Returns how many decks it reached.
func (a *App) ShowView(v config.View, device string) int {
	n := 0
	msg := map[string]any{"t": "view", "view": v, "groups": a.Resolve(v)}
	a.each(func(c *client) {
		if c.p.grant != nil || !c.matches(device) {
			return
		}
		c.mu.Lock()
		c.view = v.ID
		c.mu.Unlock()
		c.send(msg)
		n++
	})
	a.Bus.Publish("view", map[string]any{"view": v.ID, "device": device, "decks": n})
	return n
}

// matches reports whether a deck is the one named: its device id, its name, or part
// of its name ("ipad" finds "iPad Air").
func (c *client) matches(device string) bool {
	d := strings.ToLower(strings.TrimSpace(device))
	if d == "" || d == "all" {
		return true
	}
	name := strings.ToLower(c.name())
	return c.p.deviceID == device || c.id == device || name == d || strings.Contains(name, d)
}
