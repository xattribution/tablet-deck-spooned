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
	var saved []string
	for _, p := range b.Panels {
		sp, err := a.SavePanel(p)
		if err != nil {
			return nil, fmt.Errorf("panel %q: %v", p.Name, err)
		}
		saved = append(saved, sp.ID)
	}
	var views []config.View
	if len(b.Views) > 0 {
		if err := json.Unmarshal(b.Views, &views); err != nil {
			return nil, fmt.Errorf("views: %v", err)
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
	return map[string]any{"ok": true, "panels": saved, "views": vs}, nil
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
	profile, added := sc.BindAll(a.GameActions().Actions, current, name)
	written, err := sc.InstallProfile(profile, "TabletDeck-EveryAction.xml", installs)
	if err != nil {
		return nil, err
	}
	return map[string]any{"ok": true, "added": len(added), "written": written, "profile": name,
		"next": "In Star Citizen: Options > Keybindings > Control Profiles > " + name + ". Your own keyboard binds are kept."}, nil
}
