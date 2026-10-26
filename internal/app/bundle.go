package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/xattribution/tablet-deck-spooned/internal/config"
	"github.com/xattribution/tablet-deck-spooned/internal/panels"
	"github.com/xattribution/tablet-deck-spooned/internal/sc"
)

// Export writes the player's panels and views as one file: everything, or one view
// with the panels it uses. Built-in panels aren't written; every install has them.
func (a *App) Export(viewID string) panels.Bundle {
	b := panels.Bundle{Format: panels.Format}
	var views []config.View
	wantPanels := map[string]bool{}
	if viewID != "" {
		if v, ok := a.ViewByID(viewID); ok {
			if !v.Preset {
				views = append(views, v)
			}
			for _, g := range v.Groups {
				wantPanels[g] = true
			}
		}
	} else {
		views = a.Store.State().Views
	}
	for _, p := range a.Panels() {
		if p.Source == "built-in" || (viewID != "" && !wantPanels[p.ID]) {
			continue
		}
		p.Source = ""
		b.Panels = append(b.Panels, p)
	}
	if len(views) > 0 {
		b.Views, _ = json.Marshal(views)
	}
	return b
}

// Import reads what Export wrote, or a single panel, and saves it.
func (a *App) Import(data []byte) (map[string]any, error) {
	var b panels.Bundle
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("not a Tablet Deck file: %v", err)
	}
	if b.Format == "" {
		// A single panel on its own.
		var p panels.Panel
		if json.Unmarshal(data, &p) == nil && p.Name != "" {
			b.Panels = []panels.Panel{p}
		} else {
			return nil, fmt.Errorf("not a Tablet Deck file (no format or panel name)")
		}
	} else if !strings.HasPrefix(b.Format, "tabletdeck/") {
		return nil, fmt.Errorf("unknown format %q", b.Format)
	}
	if len(b.Panels) > 200 {
		return nil, fmt.Errorf("up to 200 panels in one file")
	}
	var views []config.View
	if len(b.Views) > 0 {
		if err := json.Unmarshal(b.Views, &views); err != nil {
			return nil, fmt.Errorf("views: %v", err)
		}
	}
	// Check the views before saving anything: their panels may be in this file.
	coming := map[string]bool{}
	for _, p := range b.Panels {
		id := strings.ToLower(strings.TrimSpace(p.ID))
		if id == "" {
			id = panels.Slug(p.Name)
		}
		coming[id] = true
	}
	for _, v := range views {
		for _, g := range v.Groups {
			if _, ok := a.GroupByID(g); !ok && !coming[g] {
				return nil, fmt.Errorf("view %q uses %q, which isn't on this PC or in the file", v.Name, g)
			}
		}
	}
	existing := map[string]string{}
	for _, p := range a.Panels() {
		existing[p.ID] = p.Source
	}
	saved, err := a.SavePanels(b.Panels)
	if err != nil {
		return nil, err
	}
	var ids, replaced []string
	for _, p := range saved {
		ids = append(ids, p.ID)
		if src, ok := existing[p.ID]; ok {
			replaced = append(replaced, p.ID+" ("+src+")")
		}
	}
	var vs []string
	for _, v := range views {
		sv, err := a.SaveView(v)
		if err != nil {
			return nil, fmt.Errorf("view %q: %v", v.Name, err)
		}
		vs = append(vs, sv.ID)
	}
	res := map[string]any{"ok": true, "panels": ids, "views": vs}
	if len(replaced) > 0 {
		res["replaced"] = replaced
	}
	return res, nil
}

// BindAll writes a keybind profile that keeps the player's keyboard binds and gives
// every unbound game action a free key, into each install's Controls\Mappings. The
// player loads it in the game: Options > Keybindings > Control Profiles.
func (a *App) BindAll() (map[string]any, error) {
	installs := a.Binds().Installs
	if len(installs) == 0 {
		installs = sc.Discover(a.Engine.Driver().Game().Exe, a.Store.Config().GameDirs)
	}
	if len(installs) == 0 {
		return nil, fmt.Errorf("couldn't find Star Citizen; add its folder under Keybinds > Game folders")
	}
	var current []byte
	for _, in := range installs {
		if in.Actionmaps != "" {
			current, _ = os.ReadFile(in.Actionmaps)
			break
		}
	}
	const name = "Tablet Deck - every action"
	var reserved []string // keys the deck's own binds use (its bundled profile)
	for _, k := range a.Binds().Binds {
		reserved = append(reserved, k)
	}
	profile, added := sc.BindAll(a.GameActions().Actions, current, name, reserved...)
	written, err := sc.InstallProfile(profile, "TabletDeck-EveryAction.xml", installs)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "added": len(added), "written": written, "profile": name,
		"next": "In Star Citizen: Options > Keybindings > Control Profiles > " + name + ". Your keyboard, joystick and mouse binds are kept."}, nil
}
