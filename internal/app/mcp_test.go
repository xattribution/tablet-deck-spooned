package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func mcpDo(t *testing.T, a *App, method string, params any) map[string]any {
	t.Helper()
	cfg := a.Store.Config()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://127.0.0.1:%d/mcp", cfg.Port), bytes.NewReader(body))
	req.Header.Set("X-Deck-Key", cfg.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if out["error"] != nil {
		t.Fatalf("%s: %v", method, out["error"])
	}
	return out["result"].(map[string]any)
}

func callTool(t *testing.T, a *App, name string, args map[string]any) (string, bool) {
	t.Helper()
	r := mcpDo(t, a, "tools/call", map[string]any{"name": name, "arguments": args})
	text := r["content"].([]any)[0].(map[string]any)["text"].(string)
	isErr, _ := r["isError"].(bool)
	return text, isErr
}

func TestMCP(t *testing.T) {
	a, f := startApp(t, "Pilot")
	cfg := a.Store.Config()
	init := mcpDo(t, a, "initialize", map[string]any{"protocolVersion": mcpProtocol, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "test"}})
	if init["protocolVersion"] != mcpProtocol {
		t.Fatalf("init: %v", init)
	}
	tools := mcpDo(t, a, "tools/list", nil)["tools"].([]any)
	if len(tools) < 15 {
		t.Fatalf("%d tools", len(tools))
	}

	text, _ := callTool(t, a, "find_controls", map[string]any{"query": "landing gear"})
	if !strings.Contains(text, `"id":"landing-gear"`) {
		t.Fatalf("find: %s", text)
	}
	f.Events()
	if text, isErr := callTool(t, a, "press", map[string]any{"control": "landing gear"}); isErr || !strings.Contains(text, `"ok":true`) {
		t.Fatalf("press: %s", text)
	}
	if len(f.Events()) != 2 {
		t.Fatal("press didn't reach the keyboard")
	}

	// "Bring up my flight controls on the iPad."
	ipad := dial(t, fmt.Sprintf("ws://127.0.0.1:%d/ws?t=%s&name=iPad%%20Air", cfg.Port, cfg.Token))
	ipad.wait("hello", nil)
	phone := dial(t, fmt.Sprintf("ws://127.0.0.1:%d/ws?t=%s&name=Pixel", cfg.Port, cfg.Token))
	phone.wait("hello", nil)
	if text, isErr := callTool(t, a, "show_view", map[string]any{"view": "flight", "device": "ipad"}); isErr {
		t.Fatal(text)
	}
	v := ipad.wait("view", nil)
	gs := v["groups"].([]any)
	if len(gs) < 4 || gs[0].(map[string]any)["id"] != "power" {
		t.Fatalf("flight view groups: %v", gs)
	}

	// A one-off view of single controls, then saved.
	if text, isErr := callTool(t, a, "show_view", map[string]any{"controls": []any{"landing gear", "vtol", "request landing"}, "title": "Landing", "device": "pixel"}); isErr {
		t.Fatal(text)
	}
	pv := phone.wait("view", func(m map[string]any) bool {
		return strings.HasPrefix(m["view"].(map[string]any)["id"].(string), "agent-")
	})
	if n := len(pv["groups"].([]any)[0].(map[string]any)["controls"].([]any)); n != 3 {
		t.Fatalf("one-off view has %d controls", n)
	}
	if text, isErr := callTool(t, a, "save_view", map[string]any{"name": "Bounty run", "groups": []any{"targeting", "combat"},
		"sections": []any{map[string]any{"name": "Quick", "controls": []any{"landing gear", "engage quantum"}}}}); isErr {
		t.Fatal(text)
	}
	if _, ok := a.ViewByID("bounty-run"); !ok {
		t.Fatal("view not saved")
	}
	text, _ = callTool(t, a, "share_controls", map[string]any{"for": "Mike", "groups": []any{"landing"}, "minutes": 30})
	if !strings.Contains(text, "/g/") {
		t.Fatalf("share: %s", text)
	}
	if _, isErr := callTool(t, a, "press", map[string]any{"control": "no such thing"}); !isErr {
		t.Fatal("unknown control didn't error")
	}
}

func TestCatalog(t *testing.T) {
	a, _ := startApp(t, "Pilot")
	all, byID := a.Catalog()
	count := map[string]int{}
	for _, c := range all {
		count[c.Group]++
	}
	t.Logf("%d controls: %v", len(all), count)
	for _, id := range []string{"landing-gear", "engage-quantum", "shields-up"} {
		if byID[id] == nil {
			t.Errorf("no %s", id)
		}
	}
	for _, g := range []string{"power", "landing", "targeting", "mining", "salvage", "emotes", "on-foot"} {
		if count[g] == 0 {
			t.Errorf("group %s is empty", g)
		}
	}
}
