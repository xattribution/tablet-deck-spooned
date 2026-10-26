package layout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xattribution/tablet-deck-spooned/internal/sc"
)

// Every bundled layout must load, with the bundled keybind profile applied.
func TestBundledLayouts(t *testing.T) {
	binds := sc.ReadBinds("../../keybinds/TabletDeck-StarCitizen.xml")
	if len(binds) < 20 {
		t.Fatalf("only %d binds read from the profile", len(binds))
	}
	files, _ := filepath.Glob("../../layouts/*.json")
	if len(files) == 0 {
		t.Fatal("no layouts found")
	}
	for _, f := range files {
		data, _ := os.ReadFile(f)
		l, err := Parse(filepath.Base(f), data, binds)
		if err != nil {
			t.Errorf("%s: %v", f, err)
			continue
		}
		if len(l.Buttons) < 10 || len(l.Panels) == 0 {
			t.Errorf("%s: %d buttons, %d panels", f, len(l.Buttons), len(l.Panels))
		}
		total := 0
		for _, p := range l.Panels {
			total += len(p.Buttons)
		}
		if total != len(l.Buttons) {
			t.Errorf("%s: panels hold %d buttons, layout has %d", f, total, len(l.Buttons))
		}
	}
}

func TestOverridesAndVoice(t *testing.T) {
	data := []byte(`{"name":"T","pages":[{"name":"Flight","buttons":[
		{"id":"gear","label":"Landing Gear","keys":"n","scAction":"v_toggle_landing_system"},
		{"id":"sh","label":"Shields +","keys":"f5"},
		{"id":"wave","label":"Wave","keys":"f13","scAction":"emote_wave"},
		{"id":"x1","label":"Eject","keys":"lalt+y","voice":false}]}]}`)
	binds := sc.ParseBinds(strings.NewReader(`<ActionMaps><actionmap name="spaceship_view">
		<action name="v_toggle_landing_system"><rebind input="kb1_lalt+g"/><rebind input="js1_button3"/></action>
		<action name="v_unbound"><rebind input="kb1_ "/></action></actionmap></ActionMaps>`))
	l, err := Parse("t.json", data, binds)
	if err != nil {
		t.Fatal(err)
	}
	if l.Buttons["gear"].Keys != "lalt+g" {
		t.Fatalf("override not applied: %s", l.Buttons["gear"].Keys)
	}
	pages := l.Raw["pages"].([]any)
	if pages[0].(map[string]any)["buttons"].([]any)[0].(map[string]any)["keys"] != "lalt+g" {
		t.Fatal("deck JSON doesn't show the player's key")
	}
	phrases, _ := l.VoicePhrases(false)
	if phrases["shields up"] != "sh" || phrases["emote wave"] != "wave" || phrases["wave"] != "" || phrases["eject"] != "" {
		t.Fatalf("phrases: %v", phrases)
	}
	if _, err := Parse("bad.json", []byte(`{"pages":[{"name":"a","buttons":[{"id":"a","keys":"nope"}]}]}`), nil); err == nil {
		t.Fatal("unknown key accepted")
	}
}
