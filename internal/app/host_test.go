package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"github.com/xattribution/tablet-deck-spooned/internal/config"
)

const (
	uaIPad    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15"
	uaAndroid = "Mozilla/5.0 (Linux; Android 14; SM-X710) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
)

// dialUA connects a deck with a browser's User-Agent.
func dialUA(t *testing.T, url, ua string) *wsClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	h := http.Header{}
	h.Set("User-Agent", ua)
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: h})
	if err != nil {
		t.Fatalf("dial %s: %v", url, err)
	}
	conn.SetReadLimit(8 << 20)
	c := &wsClient{t: t, conn: conn, msgs: make(chan map[string]any, 100)}
	go func() {
		for {
			_, b, err := conn.Read(context.Background())
			if err != nil {
				close(c.msgs)
				return
			}
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			c.msgs <- m
		}
	}()
	go func() {
		for range time.Tick(time.Second) {
			if conn.Write(context.Background(), websocket.MessageText, []byte(`{"t":"ping"}`)) != nil {
				return
			}
		}
	}()
	t.Cleanup(func() { conn.CloseNow() })
	return c
}

func consoles(a *App) []config.Console { return a.Store.State().Cockpit.Consoles }

// A tablet opened in Safari and again from its home screen pairs twice; it is still
// one console. The Android app's install id is matched first.
func TestOneDeviceOneConsole(t *testing.T) {
	a, _ := startApp(t, "Pilot")
	cfg := a.Store.Config()
	base := fmt.Sprintf("ws://%s:%d/ws?t=%s&device=tablet", a.LanAddress(), cfg.Port, cfg.Token)

	safari := dialUA(t, base, uaIPad)
	id1 := safari.wait("paired", nil)["id"].(string)
	safari.wait("hello", nil)
	home := dialUA(t, base, uaIPad)
	id2 := home.wait("paired", nil)["id"].(string)
	home.wait("hello", nil)

	cons := consoles(a)
	if len(cons) != 1 {
		t.Fatalf("want one console, got %+v", cons)
	}
	if cons[0].ID != id1 || !slices.Equal(cons[0].Aliases, []string{id2}) || cons[0].Platform != "ipad" || cons[0].LastIP == "" {
		t.Fatalf("console %+v", cons[0])
	}
	if got := a.Store.ConsoleOf(id2); got != id1 {
		t.Fatalf("alias maps to %q", got)
	}
	n := 0
	a.each(func(c *client) {
		if c.consoleKey() == id1 {
			n++
		}
	})
	if n != 2 {
		t.Fatalf("%d decks answer to the console, want 2", n)
	}

	// The Android app, then Chrome on the same tablet: one console, found by address
	// and platform; the app's install id is kept.
	app := dialUA(t, base+"&did=install-a&native=android", "okhttp/4")
	appID := app.wait("paired", nil)["id"].(string)
	app.wait("hello", nil)
	app.send(map[string]any{"t": "hello-native", "text": `{"model":"Tab S9","app":"1.0"}`})
	chrome := dialUA(t, base, uaAndroid)
	chromeID := chrome.wait("paired", nil)["id"].(string)
	chrome.wait("hello", nil)
	if got := a.Store.ConsoleOf(chromeID); got != appID {
		t.Fatalf("Chrome on the same tablet made console %q, want %q", got, appID)
	}
	// A second install of the app is another screen.
	other := dialUA(t, base+"&did=install-b&native=android", "okhttp/4")
	otherID := other.wait("paired", nil)["id"].(string)
	other.wait("hello", nil)
	if a.Store.ConsoleOf(otherID) != otherID || len(consoles(a)) != 3 {
		t.Fatalf("second install merged: %+v", consoles(a))
	}
	time.Sleep(50 * time.Millisecond)
	var native map[string]any
	for _, d := range a.deviceList() {
		if d["id"] == appID {
			native, _ = d["native"].(map[string]any)
		}
	}
	if info, _ := native["info"].(map[string]any); info["model"] != "Tab S9" {
		t.Fatalf("devices list native info: %v", native)
	}

	// Unpairing the console's first device hands the console to the alias.
	a.unpair(id1)
	if cons := consoles(a); cons[0].ID != id2 || len(cons[0].Aliases) != 0 {
		t.Fatalf("after unpair: %+v", cons[0])
	}
}

func TestSameScreen(t *testing.T) {
	cons := []config.Console{
		{ID: "this-pc", LastIP: "127.0.0.1", Platform: "windows"},
		{ID: "a", LastIP: "192.168.1.20", Platform: "ipad"},
		{ID: "b", LastIP: "192.168.1.21", Platform: "android", InstallID: "x"},
	}
	cases := []struct {
		ip, platform, did string
		want              int
	}{
		{"192.168.1.20", "ipad", "", 1},
		{"192.168.1.20", "iphone", "", -1},
		{"192.168.1.30", "ipad", "", -1},
		{"192.168.1.99", "android", "x", 2}, // the install id wins over a new address
		{"192.168.1.21", "android", "y", -1},
		{"192.168.1.21", "android", "", 2},
		{"127.0.0.1", "windows", "", -1}, // never the PC's own window
		{"192.168.1.20", "other", "", -1},
	}
	for _, c := range cases {
		if got := sameScreen(cons, c.ip, c.platform, c.did); got != c.want {
			t.Errorf("sameScreen(%s, %s, %s) = %d, want %d", c.ip, c.platform, c.did, got, c.want)
		}
	}
	if platformOf(uaIPad, "tablet", "") != "ipad" || platformOf(uaIPad, "", "") != "mac" || platformOf("okhttp", "", "android") != "android" {
		t.Error("platformOf")
	}
}

// viewIDs waits for a view message that passes ok and returns its group ids.
func viewIDs(c *wsClient, ok func([]string) bool) []string {
	c.t.Helper()
	var ids []string
	c.wait("view", func(m map[string]any) bool {
		ids = nil
		gs, _ := m["groups"].([]any)
		for _, g := range gs {
			ids = append(ids, g.(map[string]any)["id"].(string))
		}
		return ok(ids)
	})
	return ids
}

// A console adds panels that aren't in the view and takes panels off; solo decks too.
func TestConsoleExtras(t *testing.T) {
	a, _ := startApp(t, "Pilot")
	cfg := a.Store.Config()
	url := fmt.Sprintf("ws://%s:%d/ws?t=%s&device=tablet", a.LanAddress(), cfg.Port, cfg.Token)
	d0, d1 := dial(t, url), dial(t, url)
	for _, d := range []*wsClient{d0, d1} {
		d.wait("hello", nil)
	}
	time.Sleep(100 * time.Millisecond)
	flight, _ := a.ViewByID("flight")
	extra := ""
	for _, g := range a.Groups() {
		if !slices.Contains(flight.Groups, g.ID) && len(a.Resolve(config.View{Groups: []string{g.ID}})) > 0 {
			extra = g.ID
			break
		}
	}
	if extra == "" {
		t.Skip("every panel is in the flight view")
	}
	own := func(i int) []string {
		var ids []string
		for _, p := range a.CockpitStatus()["consoles"].([]map[string]any)[i]["panels"].([]map[string]any) {
			ids = append(ids, p["id"].(string))
		}
		return ids
	}
	has := func(g string) func([]string) bool { return func(ids []string) bool { return slices.Contains(ids, g) } }
	hasNot := func(g string) func([]string) bool { return func(ids []string) bool { return !slices.Contains(ids, g) } }

	// Console 0 adds a panel that isn't in the view; the view's split stays.
	before0, before1 := own(0), own(1)
	d0.send(map[string]any{"t": "cockpit-add", "id": extra})
	viewIDs(d0, has(extra))
	if got := own(0); !slices.Equal(got, append(before0, extra)) || !slices.Equal(own(1), before1) {
		t.Fatalf("after add: %v %v (before %v %v)", got, own(1), before0, before1)
	}
	// ...and takes it off again.
	d0.send(map[string]any{"t": "cockpit-remove", "id": extra})
	viewIDs(d0, hasNot(extra))

	// A panel of the view taken off console 0 moves to console 1.
	g := own(0)[0]
	d0.send(map[string]any{"t": "cockpit-remove", "id": g})
	viewIDs(d1, has(g))
	if slices.Contains(own(0), g) || !slices.Contains(own(1), g) {
		t.Fatalf("view panel didn't move: %v %v", own(0), own(1))
	}

	// A solo deck (a phone) shows the whole view plus its extras, minus what it hid.
	phone := dial(t, strings.Replace(url, "device=tablet", "device=phone", 1))
	phone.wait("hello", nil)
	phone.send(map[string]any{"t": "view", "id": "flight"})
	viewIDs(phone, func(ids []string) bool { return len(ids) > 0 })
	phone.send(map[string]any{"t": "cockpit-add", "id": extra})
	ids := viewIDs(phone, has(extra))
	if len(ids) != len(a.Resolve(flight))+1 {
		t.Fatalf("solo deck shows %v", ids)
	}
	hide := flight.Groups[0]
	phone.send(map[string]any{"t": "cockpit-remove", "id": hide})
	viewIDs(phone, hasNot(hide))
	var phoneKey string
	a.each(func(c *client) {
		if c.form == "phone" {
			phoneKey = c.consoleKey()
		}
	})
	if as := a.Store.State().Cockpit.Assign["flight"]; !slices.Contains(as[hideKey(phoneKey)], hide) || !slices.Contains(as[phoneKey], extra) {
		t.Fatalf("solo assign %v", as)
	}
	phone.send(map[string]any{"t": "cockpit-add", "id": hide})
	viewIDs(phone, has(hide))

	// The cockpit's own moves keep the solo deck's changes.
	if err := a.MovePanel(own(1)[0], a.members()[0].ID, ""); err != nil {
		t.Fatal(err)
	}
	if as := a.Store.State().Cockpit.Assign["flight"]; !slices.Contains(as[phoneKey], extra) {
		t.Fatalf("a cockpit move dropped the solo deck's extra: %v", as)
	}
}

// A paired device manages settings like the PC does, but never sees the API key.
// Share links get none of it.
func TestOwnerDeviceManages(t *testing.T) {
	a, _ := startApp(t, "Pilot")
	cfg := a.Store.Config()
	d := dial(t, fmt.Sprintf("ws://%s:%d/ws?t=%s&device=tablet", a.LanAddress(), cfg.Port, cfg.Token))
	tok := d.wait("paired", nil)["token"].(string)
	api := func(method, route, cred string, body any) (int, map[string]any) {
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req, _ := http.NewRequest(method, fmt.Sprintf("http://%s:%d/api/v1/%s?%s", a.LanAddress(), cfg.Port, route, cred), rd)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp.StatusCode, out
	}
	code, c := api("GET", "config", "t="+tok, nil)
	if code != 200 || c["apiKey"] != "" || c["token"] != cfg.Token {
		t.Fatalf("config from a device: %d %v", code, c)
	}
	c["name"] = "Bridge"
	c["apiKey"] = "changed-by-a-tablet"
	if code, out := api("PUT", "config", "t="+tok, c); code != 200 {
		t.Fatalf("PUT config: %d %v", code, out)
	}
	if now := a.Store.Config(); now.Name != "Bridge" || now.APIKey != cfg.APIKey {
		t.Fatalf("after PUT: name %q, API key changed %v", now.Name, now.APIKey != cfg.APIKey)
	}
	code, st := api("GET", "status", "t="+tok, nil)
	if code != 200 || st["apiKey"] != nil || st["pairing"] == nil {
		t.Fatalf("status from a device: %d apiKey=%v pairing=%v", code, st["apiKey"], st["pairing"])
	}
	g, _ := a.CreateGrant(GrantSpec{Label: "Mike", All: true})
	if code, _ := api("GET", "config", "g="+g.Token, nil); code != 403 {
		t.Fatalf("share link read config: %d", code)
	}
	if code, _ := api("GET", "config", "t=wrongcode", nil); code != 403 {
		t.Fatalf("wrong code read config: %d", code)
	}
	if r := a.mcpCall(rpcReq{ID: json.RawMessage("1"), Method: "tools/call", Params: json.RawMessage(`{"name":"bind_all_actions"}`)}, false); r["result"].(map[string]any)["isError"] != true {
		t.Fatalf("bind_all_actions ran for a non-manager: %v", r)
	}
}

func TestServeApp(t *testing.T) {
	a, _ := startApp(t, "Pilot")
	get := func(srv *httptest.Server, path string) (int, string, http.Header) {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b), resp.Header
	}
	old := fstest.MapFS{
		"index.html": {Data: []byte("<html><head></head><body>old deck</body></html>")},
		"sw.js":      {Data: []byte("// sw")},
	}
	// Before the app is built: the old deck, and no app files.
	a.Public = old
	srv := httptest.NewServer(a.routes(false))
	defer srv.Close()
	if code, body, _ := get(srv, "/?t=1a2b3c4d"); code != 200 || !strings.Contains(body, "old deck") {
		t.Fatalf("/ without the app: %d %s", code, body)
	}
	for _, p := range []string{"/assets/index-abc.js", "/sw-app.js", "/app.webmanifest"} {
		if code, _, _ := get(srv, p); code != 404 {
			t.Fatalf("%s without the app: %d", p, code)
		}
	}

	app := fstest.MapFS{
		"app/index.html":          {Data: []byte("<html><head></head><body>new app</body></html>")},
		"app/assets/index-abc.js": {Data: []byte("console.log(1)")},
		"app/sw-app.js":           {Data: []byte("// app sw")},
		"app/app.webmanifest":     {Data: []byte("{}")},
	}
	for k, v := range old {
		app[k] = v
	}
	a.Public = app
	srv2 := httptest.NewServer(a.routes(false))
	defer srv2.Close()
	for _, p := range []string{"/?t=1a2b3c4d", "/g/sometoken"} {
		if code, body, _ := get(srv2, p); code != 200 || !strings.Contains(body, "new app") || !strings.Contains(body, `<base href="/">`) {
			t.Fatalf("%s: %d %s", p, code, body)
		}
	}
	if code, body, _ := get(srv2, "/legacy/?t=1a2b3c4d"); code != 200 || !strings.Contains(body, "old deck") || !strings.Contains(body, `<base href="/">`) {
		t.Fatalf("/legacy/: %d %s", code, body)
	}
	code, _, h := get(srv2, "/assets/index-abc.js")
	if code != 200 || h.Get("Cache-Control") != "public, max-age=31536000, immutable" || !strings.Contains(h.Get("Content-Type"), "javascript") {
		t.Fatalf("asset: %d %v", code, h)
	}
	if code, _, _ := get(srv2, "/assets/missing.js"); code != 404 {
		t.Fatalf("missing asset: %d", code)
	}
	if code, body, _ := get(srv2, "/sw-app.js"); code != 200 || body != "// app sw" {
		t.Fatalf("sw-app.js: %d %s", code, body)
	}

	// The guest port serves share links and their files, nothing else.
	guest := httptest.NewServer(a.routes(true))
	defer guest.Close()
	for p, want := range map[string]int{"/g/sometoken": 200, "/assets/index-abc.js": 200, "/": 404, "/legacy/": 404} {
		if code, _, _ := get(guest, p); code != want {
			t.Fatalf("guest port %s: %d, want %d", p, code, want)
		}
	}
}
