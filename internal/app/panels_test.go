package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xattribution/tablet-deck-spooned/internal/config"
	"github.com/xattribution/tablet-deck-spooned/internal/panels"
)

func useTool(t *testing.T, a *App, name string, args map[string]any) map[string]any {
	t.Helper()
	for _, tl := range mcpTools {
		if tl.Name == name {
			res, err := tl.run(a, args)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			b, _ := json.Marshal(res)
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			return m
		}
	}
	t.Fatalf("no tool %s", name)
	return nil
}

func TestAgentBuildsAPanelAndItPresses(t *testing.T) {
	a, f := startApp(t, "Pilot")
	panel := map[string]any{
		"name": "Stream", "category": "crew",
		"actions": []any{map[string]any{"id": "obs-record", "label": "Record", "keys": "rctrl+ralt+f9", "state": true, "global": true}},
		"layout":  map[string]any{"flip": []any{map[string]any{"toggle": "obs-record", "size": "hero"}, map[string]any{"key": "sc:v_toggle_landing_system"}}},
	}
	res := useTool(t, a, "save_panel", map[string]any{"panel": panel})
	if res["id"] != "stream" || res["controls"].(float64) != 2 {
		t.Fatalf("save_panel: %v", res)
	}
	g, ok := a.GroupByID("stream")
	if !ok || g.Category != "crew" {
		t.Fatalf("no stream group: %+v", g)
	}
	rg := a.Resolve(config.View{ID: "x", Name: "x", Groups: []string{"stream"}})
	if len(rg) != 1 || len(rg[0].Layout) == 0 || len(rg[0].Controls) != 2 {
		t.Fatalf("resolved: %+v", rg)
	}

	// It presses, even with the game behind (global).
	f.SetGame(true, false, false)
	cfg := a.Store.Config()
	c := dial(t, fmt.Sprintf("ws://%s:%d/ws?t=%s", a.LanAddress(), cfg.Port, cfg.Token))
	c.wait("hello", nil)
	c.send(map[string]any{"t": "tap", "id": "obs-record"})
	c.wait("state", func(m map[string]any) bool { return m["group"] == "obs-record" && m["on"] == true })
	if n := len(f.Events()); n != 6 {
		t.Fatalf("want rctrl+ralt+f9 down and up (6 events), got %d", n)
	}

	// A made-up control is reported, not silently dropped.
	res = useTool(t, a, "save_panel", map[string]any{"panel": map[string]any{"name": "Typo", "controls": []any{"sc:v_no_such_thing", "landing-gear"}}})
	if fmt.Sprint(res["missing"]) != "[sc.v_no_such_thing]" {
		t.Fatalf("missing: %v", res)
	}

	// An action id another group owns is refused.
	_, err := a.SavePanel(mustPanel(`{"name":"Dup","actions":[{"id":"discord-mute","label":"M","keys":"f1"}]}`))
	if err == nil {
		t.Fatal("clashing action id accepted")
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	a, _ := startApp(t, "Pilot")
	if _, err := a.SavePanel(mustPanel(`{"name":"Scan","controls":["sc:v_toggle_scan_mode"]}`)); err != nil {
		t.Fatal(err)
	}
	b := a.Export("")
	if b.Format != "tabletdeck/1" || len(b.Panels) != 1 {
		t.Fatalf("export: %+v", b)
	}
	data, _ := json.Marshal(b)
	if !a.DeletePanel("scan") {
		t.Fatal("delete failed")
	}
	if _, ok := a.GroupByID("scan"); ok {
		t.Fatal("still there after delete")
	}
	res, err := a.Import(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.GroupByID("scan"); !ok || fmt.Sprint(res["panels"]) != "[scan]" {
		t.Fatalf("import: %v", res)
	}
	// A single panel on its own imports too, and a stranger's file doesn't.
	if _, err := a.Import([]byte(`{"name":"Solo","controls":["landing-gear"]}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Import([]byte(`{"hello":1}`)); err == nil {
		t.Fatal("imported a file that isn't a deck file")
	}
}

func TestUnboundGameActionReports(t *testing.T) {
	a, f := startApp(t, "Pilot")
	var id string
	for _, act := range a.GameActions().Actions {
		if act.Key == "" {
			id = "sc." + act.Name
			break
		}
	}
	if id == "" {
		t.Skip("no unbound action in the built-in list")
	}
	all, _ := a.Catalog()
	found := false
	for _, c := range all {
		if c.ID == id {
			found = c.Unbound
		}
	}
	if !found {
		t.Fatalf("%s isn't in the catalog as unbound", id)
	}
	cfg := a.Store.Config()
	c := dial(t, fmt.Sprintf("ws://%s:%d/ws?t=%s", a.LanAddress(), cfg.Port, cfg.Token))
	c.wait("hello", nil)
	c.send(map[string]any{"t": "tap", "id": id})
	if m := c.wait("blocked", nil); m["reason"] != "unbound" {
		t.Fatalf("blocked: %v", m)
	}
	if len(f.Events()) != 0 {
		t.Fatal("an unbound action sent keys")
	}
}

func TestAgentAnswersAndPresses(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			http.Error(w, "bad request", 400)
			return
		}
		var body struct {
			Tools []any `json:"tools"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if calls.Add(1) == 1 {
			if len(body.Tools) == 0 {
				http.Error(w, "no tools offered", 400)
				return
			}
			fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"c1","type":"function","function":{"name":"press","arguments":"{\"control\":\"landing-gear\"}"}}]}}]}`)
			return
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"Gear coming down."}}]}`)
	}))
	defer srv.Close()
	a, f := startApp(t, "Pilot")
	_ = a.Store.Update(func(c *config.Config) { c.AgentURL = srv.URL + "/v1"; c.AgentKey = "k"; c.AgentModel = "m" })
	a.AskAgent("drop the gear", "test")
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		log := a.chatLog()
		if len(log) >= 2 && log[len(log)-1].Who == "agent" {
			if log[len(log)-1].Text != "Gear coming down." {
				t.Fatalf("reply: %+v", log)
			}
			if len(f.Events()) == 0 {
				t.Fatal("the agent's press sent no keys")
			}
			return
		}
		if len(log) > 0 && log[len(log)-1].Who == "system" {
			t.Fatalf("agent failed: %+v", log)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("no answer: %+v", a.chatLog())
}

func mustPanel(s string) (p panels.Panel) {
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &p); err != nil {
		panic(err)
	}
	return p
}

func TestAgentDangerGuard(t *testing.T) {
	a, f := startApp(t, "Pilot")
	c := a.FindControl("sc.v_self_destruct")
	if c == nil || !c.Danger {
		t.Fatalf("self-destruct isn't marked dangerous: %+v", c)
	}
	if r := a.runTool("press", map[string]any{"control": "sc.v_self_destruct", "__player": "drop the gear"}); !strings.Contains(r, "ask for it") {
		t.Fatalf("the agent pressed self-destruct unasked: %s", r)
	}
	if r := a.runTool("press", map[string]any{"control": "sc.v_self_destruct", "mode": "hold_start", "__player": "self destruct"}); !strings.Contains(r, "ask for it") {
		t.Fatalf("the agent held self-destruct: %s", r)
	}
	if len(f.Events()) != 0 {
		t.Fatal("keys went out")
	}
	// Holding twice and letting go once leaves nothing down; the turn's end lets go too.
	a.runTool("press", map[string]any{"control": "landing-gear", "mode": "hold_start", "__player": "hold the gear"})
	a.runTool("press", map[string]any{"control": "landing-gear", "mode": "hold_start", "__player": "hold the gear"})
	a.releaseAgentHolds()
	if n := a.Engine.HeldKeys(); n != 0 {
		t.Fatalf("%d keys left down", n)
	}
}

func TestVoiceNeverPressesOnNegationsOrQuestions(t *testing.T) {
	for _, s := range []string{"don't eject", "never touch the landing gear", "how are my shields doing", "is the gear down", "do not fire"} {
		if !reNotACommand.MatchString(s) {
			t.Errorf("%q would press", s)
		}
	}
	for _, s := range []string{"landing gear", "eject", "shields front", "engage quantum", "lights"} {
		if reNotACommand.MatchString(s) {
			t.Errorf("%q is a command", s)
		}
	}
}
