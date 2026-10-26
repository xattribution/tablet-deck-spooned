package panels

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCleanFillsAndNormalizes(t *testing.T) {
	p := Panel{Name: "Scan Ops", Controls: []string{"sc:v_toggle_scan_mode"},
		Layout: json.RawMessage(`{"flip":[{"key":"sc:v_scanning_trigger_scan","size":"hero"},{"grid":"rest"}]}`)}
	if err := Clean(&p); err != nil {
		t.Fatal(err)
	}
	if p.ID != "scan-ops" || p.Code != "SO" || p.Category != "ui" {
		t.Fatalf("filled %q %q %q", p.ID, p.Code, p.Category)
	}
	if p.Controls[0] != "sc.v_toggle_scan_mode" || !strings.Contains(string(p.Layout), `"sc.v_scanning_trigger_scan"`) {
		t.Fatalf("sc: refs not normalized: %v %s", p.Controls, p.Layout)
	}
	refs := Refs(p.Layout)
	if len(refs) != 3 { // the key, "hero" and "rest"
		t.Fatalf("refs %v", refs)
	}
}

func TestCleanRejects(t *testing.T) {
	cases := map[string]Panel{
		"no name":       {},
		"bad category":  {Name: "A", Category: "boats"},
		"no keys":       {Name: "A", Actions: []Action{{ID: "x", Label: "X"}}},
		"dup action":    {Name: "A", Actions: []Action{{ID: "x", Label: "X", Keys: "f1"}, {ID: "x", Label: "Y", Keys: "f2"}}},
		"bad mode":      {Name: "A", Actions: []Action{{ID: "x", Label: "X", Keys: "f1", Mode: "spam"}}},
		"no widget":     {Name: "A", Layout: json.RawMessage(`{"flex":1}`)},
		"two widgets":   {Name: "A", Layout: json.RawMessage(`{"key":"a","toggle":"b"}`)},
		"row not list":  {Name: "A", Layout: json.RawMessage(`{"row":"a"}`)},
		"one mode":      {Name: "A", Layout: json.RawMessage(`{"modes":"m","options":[{"name":"x","value":"x","layout":{"key":"a"}}]}`)},
		"bad child":     {Name: "A", Layout: json.RawMessage(`{"col":[{"key":"a"},{"wat":1}]}`)},
		"bad id":        {Name: "A", ID: "Has Spaces"},
		"negative size": {Name: "A", Size: &Size{Weight: -1}},
	}
	for name, p := range cases {
		if err := Clean(&p); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestEveryBuiltinWidgetIsKnown(t *testing.T) {
	for _, w := range []string{"row", "col", "flip", "key", "toggle", "lever", "fader", "pair", "dial", "shieldpad", "guard", "slide", "grid", "modes", "talk", "route", "transcript", "radio"} {
		if _, ok := Widgets[w]; !ok {
			t.Errorf("widget %s missing", w)
		}
	}
}

func TestControlRefs(t *testing.T) {
	l := json.RawMessage(`{"flip":[{"key":"a","size":"hero","label":"Big"},{"fader":{"up":"u","down":"d","label":"Weapons","icon":"x"}},
		{"modes":"m","options":[{"name":"One","value":"1","layout":{"pair":["p1","p2"],"names":["A","B"]}},{"name":"Two","value":"2","layout":{"grid":"rest"}}]},
		{"radio":{"discord":"r1","game":"r2"}},{"dial":["k1"],"center":"c"}]}`)
	got := strings.Join(ControlRefs(l), ",")
	for _, want := range []string{"a", "u", "d", "p1", "p2", "r1", "r2", "k1", "c"} {
		if !strings.Contains(","+got+",", ","+want+",") {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	for _, not := range []string{"hero", "Big", "Weapons", "rest", "One", "x"} {
		if strings.Contains(","+got+",", ","+not+",") {
			t.Errorf("%s isn't a control: %s", not, got)
		}
	}
}
