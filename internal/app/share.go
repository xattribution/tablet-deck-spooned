package app

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/xattribution/tablet-deck-spooned/internal/config"
)

// ---------- share links (grants) ----------

// GrantSpec is what a new share link allows.
type GrantSpec struct {
	Label     string   `json:"label"`
	Kind      string   `json:"kind"`
	Layout    string   `json:"layout"`
	Minutes   int      `json:"minutes"` // 0 = until revoked
	All       bool     `json:"all"`
	Panels    []string `json:"panels"`
	Buttons   []string `json:"buttons"`
	AfkOnly   bool     `json:"afkOnly"`
	NoHold    bool     `json:"noHold"`
	PerMinute int      `json:"perMinute"`
}

// CreateGrant makes a share link. Without a layout it uses the tablet layout.
func (a *App) CreateGrant(s GrantSpec) (config.Grant, error) {
	s.Label = strings.TrimSpace(s.Label)
	if s.Label == "" || len(s.Label) > 32 {
		return config.Grant{}, fmt.Errorf("give the link a name (who it's for), up to 32 characters")
	}
	if s.Layout == "" {
		s.Layout = a.Store.Config().Layout
	}
	l, err := a.LayoutFile(s.Layout)
	if err != nil {
		return config.Grant{}, err
	}
	if !s.All && len(s.Panels) == 0 && len(s.Buttons) == 0 {
		return config.Grant{}, fmt.Errorf("pick some panels or buttons to share")
	}
	for _, id := range s.Buttons {
		if l.Buttons[id] == nil {
			return config.Grant{}, fmt.Errorf("no button %q in %s", id, l.Name)
		}
	}
	if s.Kind != "crew" {
		s.Kind = "share"
	}
	if s.PerMinute == 0 {
		s.PerMinute = 60
	}
	g := config.Grant{ID: config.RandomHex(6), Label: s.Label, Kind: s.Kind, Token: config.RandomHex(20), Layout: s.Layout,
		Buttons: s.Buttons, Panels: s.Panels, All: s.All, AfkOnly: s.AfkOnly, NoHold: s.NoHold, PerMinute: s.PerMinute,
		Created: time.Now()}
	if s.Minutes > 0 {
		g.Expires = time.Now().Add(time.Duration(s.Minutes) * time.Minute)
	}
	if err := a.Store.UpdateState(func(st *config.State) { st.Grants = append(st.Grants, g) }); err != nil {
		return config.Grant{}, err
	}
	a.Log.Printf("Share link for %s created.", g.Label)
	a.Bus.Publish("grants", nil)
	return g, nil
}

// RevokeGrant ends a share link and disconnects whoever is using it.
func (a *App) RevokeGrant(id string) bool {
	found := false
	_ = a.Store.UpdateState(func(st *config.State) {
		kept := st.Grants[:0]
		for _, g := range st.Grants {
			if g.ID == id {
				found = true
				continue
			}
			kept = append(kept, g)
		}
		st.Grants = kept
	})
	if found {
		a.each(func(c *client) {
			if c.p.grant != nil && c.p.grant.ID == id {
				c.kick(map[string]any{"t": "revoked"})
			}
		})
		a.Bus.Publish("grants", nil)
	}
	return found
}

func (a *App) grantByID(id string) *config.Grant {
	for _, g := range a.Store.State().Grants {
		if g.ID == id {
			g := g
			return &g
		}
	}
	return nil
}

func (a *App) countGrantPress(id string) {
	_ = a.Store.UpdateState(func(st *config.State) {
		for i := range st.Grants {
			if st.Grants[i].ID == id {
				st.Grants[i].Presses++
			}
		}
	})
}

// expireGrants disconnects guests whose link ran out.
func (a *App) expireGrants() {
	now := time.Now()
	a.each(func(c *client) {
		if c.p.grant == nil {
			return
		}
		if g := a.grantByID(c.p.grant.ID); g == nil || !g.Live(now) {
			c.kick(map[string]any{"t": "revoked", "reason": "expired"})
		}
	})
}

// GrantLinks are the addresses a share link can be opened at: the internet one when a
// tunnel is up, and the home network one.
func (a *App) GrantLinks(g config.Grant) map[string]string {
	out := map[string]string{}
	cfg := a.Store.Config()
	if ip := a.LanAddress(); ip != "" {
		out["lan"] = fmt.Sprintf("http://%s:%d/g/%s", ip, cfg.Port, g.Token)
	}
	if u := a.tunnel.URL(); u != "" {
		out["internet"] = fmt.Sprintf("%s/g/%s", u, g.Token)
	}
	return out
}

// shareFromDeck makes a share link from a tablet's Share sheet.
func (a *App) shareFromDeck(c *client, m msg) {
	g, err := a.CreateGrant(GrantSpec{Label: m.Label, Layout: c.layout.File, Minutes: m.Mins, All: m.All,
		Panels: m.Panels, Buttons: m.Btns, AfkOnly: m.AFK})
	if err != nil {
		c.send(map[string]any{"t": "toast", "text": err.Error()})
		return
	}
	links := a.GrantLinks(g)
	c.send(map[string]any{"t": "shared", "id": g.ID, "label": g.Label, "links": links, "expires": g.Expires})
}

// ---------- requests ----------

// A request is a crewmate asking: "press Landing Gear for me" or "brb, watch my ship".
type request struct {
	ID     string    `json:"id"`
	From   string    `json:"from"`
	Text   string    `json:"text,omitempty"`
	Button string    `json:"button,omitempty"`
	Label  string    `json:"label,omitempty"`
	Time   time.Time `json:"time"`
	layout string    // the layout Button is on
	reply  func(map[string]any)
}

type requests struct {
	mu   sync.Mutex
	list []*request
}

func newRequests() *requests { return &requests{} }

// open lists requests from the last 10 minutes that nobody answered.
func (r *requests) open() []*request {
	r.mu.Lock()
	defer r.mu.Unlock()
	cut := time.Now().Add(-10 * time.Minute)
	kept := r.list[:0]
	for _, q := range r.list {
		if q.Time.After(cut) {
			kept = append(kept, q)
		}
	}
	r.list = kept
	return append([]*request(nil), kept...)
}

func (r *requests) take(id string) *request {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, q := range r.list {
		if q.ID == id {
			r.list = append(r.list[:i], r.list[i+1:]...)
			return q
		}
	}
	return nil
}

// addRequest takes a request from a guest or crew link and shows it on the owner's
// decks. The owner's "Do it" presses the button as the owner, so a request can ask for
// buttons the share link doesn't cover.
func (a *App) addRequest(c *client, m msg) {
	text := strings.TrimSpace(m.Text)
	if len(text) > 140 {
		text = text[:140]
	}
	q := &request{ID: config.RandomHex(5), From: c.name(), Text: text, Time: time.Now(), layout: c.layout.File,
		reply: func(v map[string]any) { c.send(v) }}
	if b := c.layout.Buttons[m.ID]; b != nil && c.p.grant != nil {
		// A guest names a button on the owner's layout.
		q.Button, q.Label = b.ID, b.Label
	}
	if q.Text == "" && q.Button == "" {
		return
	}
	if c.p.grant == nil {
		return // owners send requests to crew through crew-request
	}
	a.reqs.mu.Lock()
	a.reqs.list = append(a.reqs.list, q)
	if len(a.reqs.list) > 50 {
		a.reqs.list = a.reqs.list[len(a.reqs.list)-50:]
	}
	a.reqs.mu.Unlock()
	a.Log.Printf("Request from %s: %s %s", q.From, q.Label, q.Text)
	a.broadcastOwners(map[string]any{"t": "request", "request": q})
	a.Bus.Publish("request", q)
	c.send(map[string]any{"t": "toast", "text": "Sent"})
}

func (a *App) answerRequest(c *client, id string, do bool) {
	q := a.reqs.take(id)
	if q == nil {
		return
	}
	a.broadcastOwners(map[string]any{"t": "request-gone", "id": id})
	if do && q.Button != "" {
		if l, err := a.LayoutFile(q.layout); err == nil {
			if b := l.Buttons[q.Button]; b != nil {
				src := c.source()
				src.Who = c.name() + " for " + q.From
				if r := a.Engine.Tap(b, src, 0, "request"); !r.OK {
					c.send(map[string]any{"t": "blocked", "id": b.ID, "reason": r.Reason})
					do = false
				}
			}
		}
	}
	verdict := map[bool]string{true: "done", false: "declined"}[do]
	if q.reply != nil {
		q.reply(map[string]any{"t": "answered", "id": id, "result": verdict, "label": q.Label, "text": q.Text})
	}
}
