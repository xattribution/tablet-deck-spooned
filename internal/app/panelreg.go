package app

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tabletdeck "github.com/xattribution/tablet-deck-spooned"
	"github.com/xattribution/tablet-deck-spooned/internal/config"
	"github.com/xattribution/tablet-deck-spooned/internal/keys"
	"github.com/xattribution/tablet-deck-spooned/internal/layout"
	"github.com/xattribution/tablet-deck-spooned/internal/panels"
	"github.com/xattribution/tablet-deck-spooned/internal/sc"
)

// Panels come from three places: built into the app (panels/*.json), dropped into the
// "panels" folder next to config.json, and saved from a deck, the PC or an agent (in
// state.json). A later source replaces an earlier one with the same id, so a player
// can restyle a built-in panel by saving one with its id.
//
// Every game action is also in the catalog, read from the installed game, under a
// group per game menu ("Vehicles - Weapons"), so anything the game can do can be put
// on a panel, found, pressed by an agent or spoken.

type gameSet struct {
	Actions []sc.Action
	Version string
	byName  map[string]*sc.Action
}

// GameActions returns the game's actions: the installed game's when it has been read,
// else the built-in list.
func (a *App) GameActions() *gameSet {
	a.gameOnce.Do(func() {
		set := newGameSet(sc.BuiltinActions(), "built in (4.10)")
		a.game.Store(set)
		go a.loadGameActions()
	})
	return a.game.Load()
}

func newGameSet(acts []sc.Action, version string) *gameSet {
	s := &gameSet{Actions: acts, Version: version, byName: map[string]*sc.Action{}}
	for i := range acts {
		s.byName[acts[i].Name] = &acts[i]
	}
	return s
}

// loadGameActions reads the installed game's actions (a few seconds the first time per
// game patch, then from the cache).
func (a *App) loadGameActions() {
	installs := a.Binds().Installs
	if len(installs) == 0 {
		installs = sc.Discover(a.Engine.Driver().Game().Exe, a.Store.Config().GameDirs)
	}
	for _, in := range installs {
		acts, version, err := sc.LoadActions(in.Dir, filepath.Join(a.Store.Dir, "game-actions-"+strings.ToLower(in.Channel)+".json"))
		if err != nil {
			a.Log.Printf("Game actions from %s: %v", in.Channel, err)
			continue
		}
		a.game.Store(newGameSet(acts, version))
		a.Log.Printf("Game actions: %d from %s %s", len(acts), in.Channel, version)
		a.panelGen.Add(1)
		a.broadcastOwners(map[string]any{"t": "reload"})
		return
	}
}

// ---------- panels ----------

type panelSet struct {
	list []panels.Panel
	byID map[string]*panels.Panel
}

var builtinPanels = sync.OnceValue(func() []panels.Panel {
	var out []panels.Panel
	_ = fs.WalkDir(tabletdeck.Files, "panels", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".json") {
			return nil
		}
		b, _ := tabletdeck.Files.ReadFile(p)
		var pn panels.Panel
		if json.Unmarshal(b, &pn) == nil && panels.Clean(&pn) == nil {
			pn.Source = "built-in"
			out = append(out, pn)
		}
		return nil
	})
	return out
})

// folderPanels reads the panels folder (and reports a stamp for change checks).
func (a *App) folderPanels() ([]panels.Panel, string) {
	dir := filepath.Join(a.Store.Dir, "panels")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, ""
	}
	var out []panels.Panel
	var stamp strings.Builder
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if fi, err := e.Info(); err == nil {
			fmt.Fprintf(&stamp, "%s|%d|%d;", e.Name(), fi.Size(), fi.ModTime().UnixNano())
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var bundle panels.Bundle
		if err := json.Unmarshal(b, &bundle); err != nil {
			a.Log.Printf("Panel %s: %v", e.Name(), err)
			continue
		}
		list := bundle.Panels
		if bundle.Format == "" { // one panel on its own
			var pn panels.Panel
			_ = json.Unmarshal(b, &pn)
			list = []panels.Panel{pn}
		}
		for _, pn := range list {
			if err := panels.Clean(&pn); err != nil {
				a.Log.Printf("Panel %s: %v", e.Name(), err)
				continue
			}
			pn.Source = "folder"
			out = append(out, pn)
		}
	}
	return out, stamp.String()
}

// folderStamp changes when a file in the panels folder changes (checked at most once
// every two seconds; the catalog asks often).
type stampCache struct {
	sync.Mutex
	at    time.Time
	stamp string
}

func (a *App) folderStamp() string {
	c := &a.folderStamps
	c.Lock()
	defer c.Unlock()
	if time.Since(c.at) < 2*time.Second {
		return c.stamp
	}
	c.at = time.Now()
	ents, err := os.ReadDir(filepath.Join(a.Store.Dir, "panels"))
	if err != nil {
		c.stamp = ""
		return ""
	}
	var b strings.Builder
	for _, e := range ents {
		if fi, err := e.Info(); err == nil {
			fmt.Fprintf(&b, "%s|%d|%d;", e.Name(), fi.Size(), fi.ModTime().UnixNano())
		}
	}
	c.stamp = b.String()
	return c.stamp
}

// Panels returns every panel, later sources replacing earlier ones by id.
func (a *App) Panels() []panels.Panel {
	ps := a.panelsNow()
	return append([]panels.Panel(nil), ps.list...)
}

func (a *App) panelsNow() *panelSet {
	folder, _ := a.folderPanels()
	saved := a.Store.State().Panels
	for i := range saved {
		saved[i].Source = "saved"
	}
	set := &panelSet{byID: map[string]*panels.Panel{}}
	var order []string
	for _, src := range [][]panels.Panel{builtinPanels(), folder, saved} {
		for _, p := range src {
			if _, ok := set.byID[p.ID]; !ok {
				order = append(order, p.ID)
			}
			pp := p
			set.byID[p.ID] = &pp
		}
	}
	for _, id := range order {
		set.list = append(set.list, *set.byID[id])
	}
	return set
}

// SavePanel stores one of the player's panels (replacing one with the same id).
func (a *App) SavePanel(p panels.Panel) (panels.Panel, error) {
	out, err := a.SavePanels([]panels.Panel{p})
	if err != nil {
		return p, err
	}
	return out[0], nil
}

// SavePanels checks every panel first and saves them all in one go, or none.
func (a *App) SavePanels(list []panels.Panel) ([]panels.Panel, error) {
	_, byID := a.Catalog()
	taken := map[string]string{} // action id -> panel, within this batch
	for i := range list {
		p := &list[i]
		if err := panels.Clean(p); err != nil {
			return nil, fmt.Errorf("panel %q: %v", p.Name, err)
		}
		p.Source = ""
		if len(p.Actions) > 100 {
			return nil, fmt.Errorf("panel %q: up to 100 actions", p.Name)
		}
		// Its actions mustn't take the ids of controls it doesn't own.
		for _, act := range p.Actions {
			if c := byID[act.ID]; c != nil && c.Group != p.ID {
				return nil, fmt.Errorf("panel %q: action id %q is already the control %q in %s; pick another id", p.Name, act.ID, c.Label, c.Group)
			}
			if other, dup := taken[act.ID]; dup && other != p.ID {
				return nil, fmt.Errorf("panel %q: action id %q is also in panel %s", p.Name, act.ID, other)
			}
			taken[act.ID] = p.ID
		}
	}
	err := a.Store.UpdateState(func(st *config.State) {
	next:
		for _, p := range list {
			for i := range st.Panels {
				if st.Panels[i].ID == p.ID {
					st.Panels[i] = p
					continue next
				}
			}
			st.Panels = append(st.Panels, p)
		}
	})
	a.panelGen.Add(1)
	a.broadcastOwners(map[string]any{"t": "reload"})
	return list, err
}

// DeletePanel removes a saved panel (a built-in one with the same id comes back).
func (a *App) DeletePanel(id string) bool {
	found := false
	_ = a.Store.UpdateState(func(st *config.State) {
		kept := st.Panels[:0]
		for _, p := range st.Panels {
			if p.ID == id {
				found = true
				continue
			}
			kept = append(kept, p)
		}
		st.Panels = kept
	})
	if found {
		a.panelGen.Add(1)
		a.broadcastOwners(map[string]any{"t": "reload"})
	}
	return found
}

// ---------- groups ----------

// Groups lists every group: the ship systems, panels that aren't one, and the game's
// own menus.
func (a *App) Groups() []Group {
	ps := a.panelsNow()
	out := make([]Group, 0, len(groups)+len(ps.list)+40)
	seen := map[string]bool{}
	for _, g := range groups {
		if p := ps.byID[g.ID]; p != nil {
			g.Category, g.Panel = p.Category, p
			if p.Name != "" {
				g.Name = p.Name
			}
		}
		out = append(out, g)
		seen[g.ID] = true
	}
	n := len(groups)
	for _, p := range ps.list {
		if seen[p.ID] {
			continue
		}
		pp := p
		out = append(out, Group{ID: p.ID, Code: p.Code, Name: p.Name, Order: n, Category: p.Category, Panel: &pp})
		seen[p.ID] = true
		n++
	}
	for _, act := range a.GameActions().Actions {
		id := gameGroupID(act.Map)
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, Group{ID: id, Code: gameCode(act.Map), Name: gameGroupName(act), Order: 100 + n, Category: gameCategory(act.Map), Game: true})
		n++
	}
	return out
}

// GroupByID finds a group.
func (a *App) GroupByID(id string) (Group, bool) {
	for _, g := range a.Groups() {
		if g.ID == id {
			return g, true
		}
	}
	return Group{}, false
}

func gameGroupID(m string) string { return "game-" + strings.ReplaceAll(strings.ToLower(m), "_", "-") }

func gameGroupName(act sc.Action) string {
	own := strings.ReplaceAll(act.Map, "_", " ")
	if act.MapLabel == "" {
		return "Game: " + own
	}
	// Several of the game's maps share a label ("On Foot - All"); name those by map.
	if strings.HasSuffix(act.MapLabel, " - All") && act.Map != "player" {
		return "Game: " + strings.TrimSuffix(act.MapLabel, " - All") + " - " + strings.ToUpper(own[:1]) + own[1:]
	}
	return "Game: " + act.MapLabel
}

var gameCats = map[string]string{
	"spaceship_movement": "flight", "spaceship_view": "flight", "spaceship_docking": "flight", "vehicle_driver": "flight", "vehicle_general": "flight",
	"spaceship_quantum": "nav", "spaceship_radar": "nav", "spaceship_scanning": "nav", "spaceship_target_hailing": "nav",
	"spaceship_power": "eng", "spaceship_general": "eng", "lights_controller": "eng",
	"spaceship_defensive": "def",
	"spaceship_weapons":   "combat", "spaceship_missiles": "combat", "spaceship_targeting": "combat", "spaceship_targeting_advanced": "combat",
	"turret_movement": "combat", "turret_advanced": "combat", "spaceship_auto_weapons": "combat",
	"spaceship_mining": "ind", "spaceship_salvage": "ind", "mining": "ind", "tractor_beam": "ind",
}

func gameCategory(m string) string {
	if c := gameCats[m]; c != "" {
		return c
	}
	if strings.HasPrefix(m, "player") || strings.HasPrefix(m, "zero_gravity") || m == "prone" || m == "incapacitated" || m == "ui_notification" {
		return "crew"
	}
	return "ui"
}

func gameCode(m string) string {
	w := strings.Split(strings.TrimPrefix(strings.TrimPrefix(m, "spaceship_"), "vehicle_"), "_")
	s := strings.ToUpper(w[0])
	if len(s) > 3 {
		s = s[:3]
	}
	if len(w) > 1 {
		s += strings.ToUpper(w[1][:1])
	}
	return s
}

// ---------- catalog additions ----------

// Movement maps are steered: their keys are held, not tapped.
var heldMaps = map[string]bool{"spaceship_movement": true, "turret_movement": true, "vehicle_driver": true, "zero_gravity_eva": true, "spaceship_view": true}

// pressOf turns a game activation mode into how the deck presses it.
func pressOf(act sc.Action) (mode string, ms int) {
	switch act.Mode {
	case "hold", "all", "hold_toggle", "hold_no_retrigger", "delayed_hold", "delayed_hold_long", "delayed_hold_no_retrigger":
		return "hold", 0
	case "delayed_press":
		return "timed", 350
	case "delayed_press_medium":
		return "timed", 650
	case "delayed_press_long":
		return "timed", 1700
	case "":
		if heldMaps[act.Map] || act.Map == "player" && strings.Contains(strings.ToLower(act.Label), "move") {
			return "hold", 0
		}
	}
	if strings.Contains(strings.ToLower(act.Label), "(hold)") {
		return "hold", 0
	}
	return "tap", 0
}

// keyFor is the keys a game action is pressed with: the player's bind, else the game's.
func (a *App) keyFor(name string) string {
	if k, ok := a.Binds().Binds[name]; ok {
		return k // "" when the player cleared it
	}
	if act := a.GameActions().byName[name]; act != nil {
		return act.Key
	}
	return ""
}

func buttonFor(c *Control, mode string, ms int, global bool) *layout.Button {
	b := &layout.Button{ID: c.ID, Label: c.Label, Keys: c.Keys, Mode: mode, Ms: ms, ScAction: c.ScAction, State: c.State, Global: global, Icon: c.Icon}
	if strings.TrimSpace(c.Keys) != "" {
		if s, err := keys.ParseCombo(c.Keys); err == nil {
			b.Scans = s
		}
	}
	c.Unbound = len(b.Scans) == 0
	return b
}

// extendCatalog adds the panels' own actions and every game action to the catalog.
func (a *App) extendCatalog(out []*Control, byID map[string]*Control) []*Control {
	rebinds := a.Store.Config().Rebinds
	game := a.GameActions() // one snapshot: the set can be swapped while this runs
	for _, p := range a.panelsNow().list {
		for _, act := range p.Actions {
			id := act.ID
			if c := byID[id]; c != nil && c.Group != p.ID {
				id = p.ID + "." + act.ID
			}
			danger := act.Danger || reDanger.MatchString(act.Label)
			if g := game.byName[act.SC]; g != nil && reDanger.MatchString(g.Label) {
				danger = true // a friendly label on self-destruct is still self-destruct
			}
			c := &Control{ID: id, Label: act.Label, Group: p.ID, Icon: act.Icon, Danger: danger, ScAction: act.SC, Global: act.Global,
				Tags: []string{p.ID, slug(act.Label)}}
			c.Keys = act.Keys
			if act.SC != "" {
				if k := a.keyFor(act.SC); k != "" || c.Keys == "" {
					c.Keys = k
				}
			}
			if k := rebinds[id]; k != "" {
				c.Keys = k
			}
			if act.State {
				c.State = &layout.State{Group: id}
				c.Tags = append(c.Tags, "toggle")
			}
			c.Mode = act.Mode
			if c.Mode == "" {
				c.Mode = "tap"
			}
			c.Button = buttonFor(c, c.Mode, act.Ms, act.Global)
			if c.Mode == "tap" {
				c.Mode = ""
			}
			byID[id] = c
			out = append(out, c)
		}
	}
	binds := a.Binds().Binds
	for i := range game.Actions {
		act := &game.Actions[i]
		id := "sc." + act.Name
		if byID[id] != nil {
			continue
		}
		key, ok := binds[act.Name]
		if !ok {
			key = act.Key
		}
		if k := rebinds[id]; k != "" {
			key = k
		}
		mode, ms := pressOf(*act)
		c := &Control{ID: id, Label: act.Label, Keys: strings.TrimSpace(key), Group: gameGroupID(act.Map), ScAction: act.Name, Desc: act.Desc,
			Tags: []string{"game", slug(act.MapLabel)}}
		if mode != "tap" {
			c.Mode = mode
		}
		c.Danger = reDanger.MatchString(act.Label)
		c.Button = buttonFor(c, mode, ms, false)
		byID[id] = c
		out = append(out, c)
	}
	return out
}

// controlsOf lists a group's controls: its own, plus any a panel names.
func (a *App) controlsOf(g Group, all []*Control, byID map[string]*Control) []*Control {
	var out []*Control
	seen := map[string]bool{}
	add := func(c *Control) {
		if c != nil && !seen[c.ID] {
			seen[c.ID] = true
			out = append(out, c)
		}
	}
	for _, c := range all {
		if c.Group == g.ID {
			add(c)
		}
	}
	if g.Panel != nil {
		for _, ref := range g.Panel.Controls {
			if c := byID[ref]; c != nil {
				add(c)
			} else {
				add(a.FindControl(ref))
			}
		}
		for _, ref := range panels.ControlRefs(g.Panel.Layout) {
			if c := byID[g.ID+"."+ref]; c != nil {
				add(c) // the panel's own action, renamed because another group has its id
			} else {
				add(byID[ref])
			}
		}
	}
	return out
}

// missingRefs lists the controls a panel names that don't exist.
func (a *App) missingRefs(p panels.Panel) []string {
	_, byID := a.Catalog()
	var out []string
	seen := map[string]bool{}
	game := a.GameActions()
	for _, act := range p.Actions {
		if act.SC != "" && game.byName[act.SC] == nil {
			out = append(out, "sc:"+act.SC+" (action "+act.ID+")")
		}
	}
	for _, ref := range append(append([]string(nil), p.Controls...), panels.ControlRefs(p.Layout)...) {
		if seen[ref] || byID[ref] != nil || byID[p.ID+"."+ref] != nil {
			continue
		}
		seen[ref] = true
		if strings.HasPrefix(ref, "sc.") || a.FindControl(ref) == nil {
			out = append(out, ref)
		}
	}
	return out
}
