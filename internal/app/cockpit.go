package app

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/xattribution/tablet-deck-spooned/internal/config"
)

// The cockpit: every deck of one setup working as one. Each console (a tablet, a phone,
// the PC's touch window) shows its share of the same view; the PC or the consoles move
// panels between them. When a console drops off, its panels go to another console
// until it comes back. A console marked solo shows whole views on its own, as before.

var consoleGrace = 4 * time.Second // how long a console may vanish before its panels move

type cockpitRT struct {
	mu       sync.Mutex
	lost     map[string]time.Time // consoles gone, since when
	fallback map[string]string    // lost console -> the console showing its panels (picked)
	timers   map[string]*time.Timer
}

// consoleKey is the console a deck is: its paired device, or the PC's own window.
func (c *client) consoleKey() string {
	switch {
	case c.p.grant != nil || c.peer:
		return ""
	case c.p.deviceID != "":
		return c.p.deviceID
	case c.p.local:
		return "this-pc"
	}
	return ""
}

// inCockpit reports whether this deck is a console of the cockpit (not solo).
func (a *App) inCockpit(c *client) bool {
	k := c.consoleKey()
	if k == "" {
		return false
	}
	for _, con := range a.Store.State().Cockpit.Consoles {
		if con.ID == k {
			return !con.Solo
		}
	}
	return false
}

// joinCockpit adds a newly seen deck as a console: tablets and the PC join, phones
// start solo (a phone is usually a remote, not a screen of the cockpit).
func (a *App) joinCockpit(c *client) {
	k := c.consoleKey()
	if k == "" {
		return
	}
	name := c.name()
	if k == "this-pc" {
		name = "This PC"
	}
	_ = a.Store.UpdateState(func(st *config.State) {
		for _, con := range st.Cockpit.Consoles {
			if con.ID == k {
				return
			}
		}
		if n := strings.ToLower(strings.TrimSpace(name)); n == "" || n == "device" || n == "tablet" || n == "phone" {
			name = fmt.Sprintf("Console %d", len(st.Cockpit.Consoles)+1)
		}
		st.Cockpit.Consoles = append(st.Cockpit.Consoles, config.Console{ID: k, Name: name, Order: len(st.Cockpit.Consoles), Solo: c.form == "phone"})
	})
	a.cockpit.mu.Lock()
	if t := a.cockpit.timers[k]; t != nil {
		t.Stop()
		delete(a.cockpit.timers, k)
	}
	_, wasLost := a.cockpit.lost[k]
	delete(a.cockpit.lost, k)
	delete(a.cockpit.fallback, k)
	a.cockpit.mu.Unlock()
	if wasLost {
		a.broadcastOwners(map[string]any{"t": "cockpit-back", "console": k, "name": a.consoleName(k)})
	}
}

// leftCockpit runs when a deck disconnects: if that was the console's last window, its
// panels move after a short grace (a reload or a Wi-Fi blip doesn't shuffle anything).
func (a *App) leftCockpit(c *client) {
	k := c.consoleKey()
	if k == "" || !a.inCockpit(c) || a.consoleOnline(k) {
		return
	}
	a.cockpit.mu.Lock()
	defer a.cockpit.mu.Unlock()
	if t := a.cockpit.timers[k]; t != nil {
		t.Stop()
	}
	a.cockpit.timers[k] = time.AfterFunc(consoleGrace, func() {
		if a.consoleOnline(k) {
			return
		}
		a.cockpit.mu.Lock()
		a.cockpit.lost[k] = time.Now()
		delete(a.cockpit.timers, k)
		a.cockpit.mu.Unlock()
		plan := a.cockpitPlan(a.cockpitView())
		to := plan.hostOf[k]
		a.Log.Printf("Cockpit: %s dropped off; its panels went to %s.", a.consoleName(k), a.consoleName(to))
		a.broadcastOwners(map[string]any{"t": "cockpit-lost", "console": k, "name": a.consoleName(k), "to": to, "toName": a.consoleName(to)})
		a.pushCockpit()
	})
}

func (a *App) consoleOnline(k string) bool {
	on := false
	a.each(func(c *client) {
		if c.consoleKey() == k {
			on = true
		}
	})
	return on
}

func (a *App) consoleName(k string) string {
	for _, con := range a.Store.State().Cockpit.Consoles {
		if con.ID == k {
			return con.Name
		}
	}
	return k
}

// cockpitView is the view the cockpit shows (Flight to begin with).
func (a *App) cockpitView() config.View {
	if v, ok := a.ViewByID(a.Store.State().Cockpit.View); ok {
		return v
	}
	v, _ := a.ViewByID("flight")
	return v
}

// members are the consoles of the cockpit, left to right.
func (a *App) members() []config.Console {
	var out []config.Console
	for _, con := range a.Store.State().Cockpit.Consoles {
		if !con.Solo {
			out = append(out, con)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out
}

type cockpitPlan struct {
	own    map[string][]string // console -> its panels (saved, or shared out)
	shows  map[string][]string // online console -> what it shows now (its own plus guests)
	hostOf map[string]string   // offline console -> who shows its panels
}

// cockpitPlan shares a view's panels out between the consoles: the saved assignment
// first, panels nobody has go to the console with the least on it, and the panels of
// consoles that are away go to the one picked for them (or the nearest online one).
func (a *App) cockpitPlan(v config.View) cockpitPlan {
	cons := a.members()
	st := a.Store.State().Cockpit
	saved := st.Assign[v.ID]
	inView := map[string]bool{}
	for _, g := range v.Groups {
		inView[g] = true
	}
	p := cockpitPlan{own: map[string][]string{}, shows: map[string][]string{}, hostOf: map[string]string{}}
	if len(cons) == 0 {
		return p
	}
	weight := func(g string) float64 {
		if gr, ok := a.GroupByID(g); ok && gr.Panel != nil && gr.Panel.Size != nil && gr.Panel.Size.Weight > 0 {
			return gr.Panel.Size.Weight
		}
		return 4
	}
	a.cockpit.mu.Lock()
	grace := map[string]bool{}
	for k := range a.cockpit.timers {
		grace[k] = true
	}
	picked := map[string]string{}
	for k, v := range a.cockpit.fallback {
		picked[k] = v
	}
	a.cockpit.mu.Unlock()
	online := map[string]bool{}
	var up []config.Console
	for _, con := range cons {
		online[con.ID] = a.consoleOnline(con.ID)
		if online[con.ID] || grace[con.ID] {
			up = append(up, con)
		}
	}
	if len(up) == 0 {
		up = cons
	}
	placed := map[string]bool{}
	load := map[string]float64{}
	for _, con := range cons {
		for _, g := range saved[con.ID] {
			if inView[g] && !placed[g] {
				placed[g] = true
				p.own[con.ID] = append(p.own[con.ID], g)
				load[con.ID] += weight(g)
			}
		}
	}
	if len(saved) == 0 {
		// Nothing saved for this view: split it in order, left to right, by weight,
		// between the consoles that are here.
		total := 0.0
		for _, g := range v.Groups {
			total += weight(g)
		}
		per, i, acc := total/float64(len(up)), 0, 0.0
		for _, g := range v.Groups {
			w := weight(g)
			if i < len(up)-1 && acc+w/2 > per*float64(i+1) {
				i++
			}
			p.own[up[i].ID] = append(p.own[up[i].ID], g)
			load[up[i].ID] += w
			placed[g] = true
			acc += w
		}
	}
	for _, g := range v.Groups {
		if placed[g] {
			continue
		}
		best := up[0].ID
		for _, con := range up {
			if load[con.ID] < load[best] {
				best = con.ID
			}
		}
		p.own[best] = append(p.own[best], g)
		load[best] += weight(g)
	}

	for i, con := range cons {
		if online[con.ID] {
			p.shows[con.ID] = append(p.shows[con.ID], p.own[con.ID]...)
			continue
		}
		if grace[con.ID] || len(p.own[con.ID]) == 0 {
			continue // just dropped: a reload or a Wi-Fi blip doesn't shuffle anything
		}
		host := picked[con.ID]
		if !online[host] {
			host = ""
			for d := 1; d < len(cons) && host == ""; d++ { // the nearest online console
				for _, j := range []int{i - d, i + d} {
					if j >= 0 && j < len(cons) && online[cons[j].ID] {
						host = cons[j].ID
						break
					}
				}
			}
		}
		if host != "" {
			p.hostOf[con.ID] = host
			p.shows[host] = append(p.shows[host], p.own[con.ID]...)
		}
	}
	return p
}

// cockpitInfo is what each console is told about the others.
func (a *App) cockpitInfo(me string, p cockpitPlan) map[string]any {
	var cons []map[string]any
	for _, con := range a.members() {
		cons = append(cons, map[string]any{"id": con.ID, "name": con.Name, "order": con.Order, "online": len(p.shows[con.ID]) > 0 || a.consoleOnline(con.ID),
			"panels": len(p.own[con.ID]), "hostedBy": p.hostOf[con.ID]})
	}
	return map[string]any{"me": me, "consoles": cons, "theme": a.Store.State().Cockpit.Theme}
}

// pushCockpit sends every console its share of the cockpit's view.
func (a *App) pushCockpit() {
	v := a.cockpitView()
	p := a.cockpitPlan(v)
	a.each(func(c *client) {
		if !a.inCockpit(c) {
			return
		}
		k := c.consoleKey()
		mine := v
		mine.Groups = p.shows[k]
		c.mu.Lock()
		c.view = v.ID
		c.mu.Unlock()
		c.send(map[string]any{"t": "view", "view": v, "groups": a.Resolve(mine), "cockpit": a.cockpitInfo(k, p)})
	})
	a.Bus.Publish("cockpit", map[string]any{"view": v.ID})
}

// SetCockpitView shows a view across the cockpit.
func (a *App) SetCockpitView(id string) bool {
	v, ok := a.ViewByID(id)
	if !ok {
		return false
	}
	_ = a.Store.UpdateState(func(st *config.State) { st.Cockpit.View = v.ID })
	a.pushCockpit()
	return true
}

// MovePanel moves a panel of the cockpit's view to a console ("left" and "right" are
// the from console's neighbors).
func (a *App) MovePanel(group, to, from string) error {
	v := a.cockpitView()
	cons := a.members()
	if to == "left" || to == "right" {
		target := ""
		for i, con := range cons {
			if con.ID == from {
				if to == "left" && i > 0 {
					target = cons[i-1].ID
				}
				if to == "right" && i < len(cons)-1 {
					target = cons[i+1].ID
				}
			}
		}
		if target == "" {
			return fmt.Errorf("no console on that side")
		}
		to = target
	}
	found := false
	for _, con := range cons {
		found = found || con.ID == to || strings.EqualFold(con.Name, to)
		if strings.EqualFold(con.Name, to) {
			to = con.ID
		}
	}
	if !found {
		return fmt.Errorf("no console %q", to)
	}
	p := a.cockpitPlan(v)
	_ = a.Store.UpdateState(func(st *config.State) {
		if st.Cockpit.Assign == nil {
			st.Cockpit.Assign = map[string]map[string][]string{}
		}
		as := map[string][]string{}
		for k, gs := range p.own { // save the whole current split, then move
			for _, g := range gs {
				if g != group {
					as[k] = append(as[k], g)
				}
			}
		}
		as[to] = append(as[to], group)
		st.Cockpit.Assign[v.ID] = as
	})
	a.pushCockpit()
	return nil
}

// UpdateConsole renames, reorders or detaches a console.
func (a *App) UpdateConsole(id string, name *string, order *int, solo *bool) error {
	ok := false
	_ = a.Store.UpdateState(func(st *config.State) {
		cs := st.Cockpit.Consoles
		for i := range cs {
			if cs[i].ID != id {
				continue
			}
			ok = true
			if name != nil && strings.TrimSpace(*name) != "" {
				cs[i].Name = strings.TrimSpace(*name)
				if len(cs[i].Name) > 24 {
					cs[i].Name = cs[i].Name[:24]
				}
			}
			if solo != nil {
				cs[i].Solo = *solo
			}
			if order != nil {
				// Move it to that place and renumber the rest.
				sort.SliceStable(cs, func(x, y int) bool { return cs[x].Order < cs[y].Order })
				var me config.Console
				var rest []config.Console
				for _, c := range cs {
					if c.ID == id {
						me = c
					} else {
						rest = append(rest, c)
					}
				}
				at := max(0, min(*order, len(rest)))
				cs = append(rest[:at], append([]config.Console{me}, rest[at:]...)...)
				for n := range cs {
					cs[n].Order = n
				}
				st.Cockpit.Consoles = cs
			}
			return
		}
	})
	if !ok {
		return fmt.Errorf("no console %q", id)
	}
	a.pushCockpit()
	// A deck that left the cockpit goes back to whole views.
	a.each(func(c *client) {
		if c.consoleKey() == id && !a.inCockpit(c) {
			v := a.cockpitView()
			c.send(map[string]any{"t": "view", "view": v, "groups": a.Resolve(v), "cockpit": nil})
		}
	})
	return nil
}

// ForgetConsole removes a console (a device that was unpaired or is gone for good).
func (a *App) ForgetConsole(id string) {
	_ = a.Store.UpdateState(func(st *config.State) {
		kept := st.Cockpit.Consoles[:0]
		for _, c := range st.Cockpit.Consoles {
			if c.ID != id {
				kept = append(kept, c)
			}
		}
		for n := range kept {
			kept[n].Order = n
		}
		st.Cockpit.Consoles = kept
		for _, as := range st.Cockpit.Assign {
			delete(as, id)
		}
	})
	a.cockpit.mu.Lock()
	delete(a.cockpit.lost, id)
	delete(a.cockpit.fallback, id)
	if t := a.cockpit.timers[id]; t != nil {
		t.Stop()
		delete(a.cockpit.timers, id)
	}
	a.cockpit.mu.Unlock()
	a.pushCockpit()
}

// PickFallback sends a lost console's panels to another console.
func (a *App) PickFallback(lost, to string) {
	a.cockpit.mu.Lock()
	if _, ok := a.cockpit.lost[lost]; ok {
		a.cockpit.fallback[lost] = to
	}
	a.cockpit.mu.Unlock()
	a.pushCockpit()
}

// ResetCockpit forgets the saved split of the current view (it's shared out again).
func (a *App) ResetCockpit() {
	v := a.cockpitView()
	_ = a.Store.UpdateState(func(st *config.State) { delete(st.Cockpit.Assign, v.ID) })
	a.pushCockpit()
}

// SetCockpitTheme puts one theme on every console.
func (a *App) SetCockpitTheme(theme string) {
	_ = a.Store.UpdateState(func(st *config.State) { st.Cockpit.Theme = theme })
	a.each(func(c *client) {
		if a.inCockpit(c) {
			c.send(map[string]any{"t": "theme", "id": theme})
		}
	})
	a.Bus.Publish("cockpit", map[string]any{"theme": theme})
}

// CockpitStatus is the cockpit for the PC's control panel and agents.
func (a *App) CockpitStatus() map[string]any {
	v := a.cockpitView()
	p := a.cockpitPlan(v)
	var cons []map[string]any
	all := a.Store.State().Cockpit.Consoles
	sort.SliceStable(all, func(i, j int) bool { return all[i].Order < all[j].Order })
	for _, con := range all {
		var panels []map[string]any
		for _, g := range p.own[con.ID] {
			if gr, ok := a.GroupByID(g); ok {
				panels = append(panels, map[string]any{"id": gr.ID, "name": gr.Name, "code": gr.Code, "category": gr.Category})
			}
		}
		a.cockpit.mu.Lock()
		_, lost := a.cockpit.lost[con.ID]
		a.cockpit.mu.Unlock()
		cons = append(cons, map[string]any{"id": con.ID, "name": con.Name, "order": con.Order, "solo": con.Solo,
			"online": a.consoleOnline(con.ID), "lost": lost, "hostedBy": p.hostOf[con.ID], "panels": panels})
	}
	return map[string]any{"view": v.ID, "viewName": v.Name, "theme": a.Store.State().Cockpit.Theme, "consoles": cons}
}

// Identify flashes a console's name on its screen.
func (a *App) Identify(id string) {
	a.each(func(c *client) {
		if c.consoleKey() == id {
			c.send(map[string]any{"t": "identify", "name": a.consoleName(id)})
		}
	})
}
