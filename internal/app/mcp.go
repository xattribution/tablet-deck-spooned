package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	tabletdeck "github.com/xattribution/tablet-deck-spooned"
	"github.com/xattribution/tablet-deck-spooned/internal/config"
	"github.com/xattribution/tablet-deck-spooned/internal/deck"
	"github.com/xattribution/tablet-deck-spooned/internal/keys"
	"github.com/xattribution/tablet-deck-spooned/internal/panels"
)

// The MCP server lets an AI agent drive the deck: find and press controls, put views
// on devices ("bring up my flight controls on the iPad"), build and save views,
// share controls, talk to the crew and speak through the PC. It speaks MCP's
// Streamable HTTP transport at /mcp (one JSON response per request); `tabletdeck mcp`
// bridges it to stdio for agents that launch a command.

const mcpProtocol = "2025-06-18"

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type tool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	run         func(a *App, args map[string]any) (any, error)
}

func obj(props map[string]any, required ...string) map[string]any {
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func num(desc string) map[string]any { return map[string]any{"type": "number", "description": desc} }
func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
func strs(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}
func enum(desc string, vals ...string) map[string]any {
	return map[string]any{"type": "string", "enum": vals, "description": desc}
}

func argStr(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return strings.TrimSpace(s)
}
func argNum(m map[string]any, k string) int {
	f, _ := m[k].(float64)
	return int(f)
}
func argBool(m map[string]any, k string) bool {
	b, _ := m[k].(bool)
	return b
}
func argStrs(m map[string]any, k string) []string {
	var out []string
	if list, ok := m[k].([]any); ok {
		for _, v := range list {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
	}
	return out
}

func controlView(c *Control, states map[string]bool) map[string]any {
	m := map[string]any{"id": c.ID, "label": c.Label, "keys": c.Keys, "group": c.Group, "tags": c.Tags}
	if c.Mode != "" {
		m["mode"] = c.Mode
	}
	if c.State != nil {
		m["on"] = states[c.State.Group]
		m["stateGroup"] = c.State.Group
	}
	if c.Danger {
		m["danger"] = true
	}
	if c.Icon != "" {
		m["icon"] = c.Icon
	}
	if c.ScAction != "" {
		m["gameAction"] = c.ScAction
	}
	if c.Unbound {
		m["unbound"] = true
	}
	if c.Global {
		m["global"] = true
	}
	return m
}

var mcpTools = []tool{
	{Name: "panel_guide", Title: "How to build a panel",
		Description: "The panel format: every widget, how controls are referenced, and examples. Read this before save_panel.",
		InputSchema: obj(map[string]any{}),
		run: func(a *App, _ map[string]any) (any, error) {
			b, err := tabletdeck.Files.ReadFile("docs/PANELS.md")
			return string(b), err
		}},
	{Name: "list_panels", Title: "List panels",
		Description: "Every panel: built-in (the ship systems and Comms), from the panels folder, and saved ones. Each is a group a view can show.",
		InputSchema: obj(map[string]any{}),
		run: func(a *App, _ map[string]any) (any, error) {
			var out []map[string]any
			for _, p := range a.Panels() {
				out = append(out, map[string]any{"id": p.ID, "name": p.Name, "code": p.Code, "category": p.Category, "source": p.Source,
					"actions": len(p.Actions), "controls": len(p.Controls), "designed": len(p.Layout) > 0})
			}
			return out, nil
		}},
	{Name: "get_panel", Title: "Get a panel",
		Description: "One panel's full definition (actions, controls and layout), to copy or change and save again.",
		InputSchema: obj(map[string]any{"id": str("Panel id")}, "id"),
		run: func(a *App, args map[string]any) (any, error) {
			for _, p := range a.Panels() {
				if p.ID == argStr(args, "id") {
					return p, nil
				}
			}
			return nil, fmt.Errorf("no panel %q (list_panels shows them)", argStr(args, "id"))
		}},
	{Name: "save_panel", Title: "Save a panel",
		Description: "Create or replace (by id) a panel: a named collection of controls and how they're laid out. The simplest panel is a name and a list of controls (ids from find_controls, or \"sc:<game action>\"); add a layout for sliders, toggles, levers and big keys (panel_guide has the format). Add actions for keys that aren't controls yet, such as another app's hotkey (global: true). Then show it with show_view groups: [id] or add it to a view.",
		InputSchema: obj(map[string]any{"panel": map[string]any{"type": "object", "description": "The panel: {id?, name, code?, category?, size?, actions?, controls?, layout?}"}}, "panel"),
		run: func(a *App, args map[string]any) (any, error) {
			b, _ := json.Marshal(args["panel"])
			var p panels.Panel
			if err := json.Unmarshal(b, &p); err != nil {
				return nil, fmt.Errorf("panel: %v", err)
			}
			saved, err := a.SavePanel(p)
			if err != nil {
				return nil, err
			}
			g, _ := a.GroupByID(saved.ID)
			all, byID := a.Catalog()
			cs := a.controlsOf(g, all, byID)
			var unbound []string
			for _, c := range cs {
				if c.Unbound {
					unbound = append(unbound, c.Label)
				}
			}
			res := map[string]any{"ok": true, "id": saved.ID, "controls": len(cs)}
			if missing := a.missingRefs(saved); len(missing) > 0 {
				res["missing"] = missing
				res["note_missing"] = "no control has these ids; use find_controls for real ids, or add them as actions"
			}
			if len(unbound) > 0 {
				res["unbound"] = unbound
				res["note"] = "these have no key in the game yet; bind_all_actions gives them one"
			}
			return res, nil
		}},
	{Name: "delete_panel", Title: "Delete a panel",
		Description: "Delete a saved panel. A built-in panel with the same id comes back.",
		InputSchema: obj(map[string]any{"id": str("Panel id")}, "id"),
		run: func(a *App, args map[string]any) (any, error) {
			return map[string]any{"ok": a.DeletePanel(argStr(args, "id"))}, nil
		}},
	{Name: "export_layout", Title: "Export panels and views",
		Description: "The player's own panels and views as one JSON file (format tabletdeck/1) to share or back up; just one view and its panels when view is given.",
		InputSchema: obj(map[string]any{"view": str("Only this view (id or name)")}),
		run:         func(a *App, args map[string]any) (any, error) { return a.Export(argStr(args, "view")), nil }},
	{Name: "import_layout", Title: "Import panels and views",
		Description: "Save the panels and views from an exported file (or a single panel).",
		InputSchema: obj(map[string]any{"data": map[string]any{"type": "object", "description": "The exported JSON"}}, "data"),
		run: func(a *App, args map[string]any) (any, error) {
			b, _ := json.Marshal(args["data"])
			return a.Import(b)
		}},
	{Name: "set_control_keys", Title: "Change a control's keys",
		Description: "Change the keys a control presses on this deck (for example Discord's mute hotkey, or a panel action). Empty keys go back to the default. Game actions normally follow the game's own binds; use this only to override them.",
		InputSchema: obj(map[string]any{"control": str("Control id"), "keys": str("Keys like \"f13\" or \"ralt+k\"; empty for the default")}, "control"),
		run: func(a *App, args map[string]any) (any, error) {
			c := a.FindControl(argStr(args, "control"))
			if c == nil {
				return nil, fmt.Errorf("no control %q", argStr(args, "control"))
			}
			k := strings.TrimSpace(argStr(args, "keys"))
			if k != "" {
				if _, err := keys.ParseCombo(k); err != nil {
					return nil, err
				}
			}
			err := a.Store.Update(func(cfg *config.Config) {
				if cfg.Rebinds == nil {
					cfg.Rebinds = map[string]string{}
				}
				if k == "" {
					delete(cfg.Rebinds, c.ID)
				} else {
					cfg.Rebinds[c.ID] = k
				}
			})
			a.broadcastOwners(map[string]any{"t": "reload"})
			return map[string]any{"ok": err == nil, "control": c.ID, "keys": k}, err
		}},
	{Name: "bind_all_actions", Title: "Give every game action a key",
		Description: "Write a keybind profile that keeps all the player's binds (keyboard, joystick, gamepad, mouse) and gives every game action without a key a free one, so the deck can press anything. The player then loads it in the game (Options > Keybindings > Control Profiles).",
		InputSchema: obj(map[string]any{}),
		run:         func(a *App, _ map[string]any) (any, error) { return a.BindAll() }},

	{Name: "cockpit_status", Title: "The cockpit",
		Description: "The cockpit: every console (tablet or screen) left to right, online or not, and the panels each shows in the cockpit's view.",
		InputSchema: obj(map[string]any{}),
		run:         func(a *App, _ map[string]any) (any, error) { return a.CockpitStatus(), nil }},
	{Name: "move_panel", Title: "Move a panel to another console",
		Description: "Move a panel (group id) of the cockpit's view to another console, by console id or name (\"move shields to the left screen\": cockpit_status first).",
		InputSchema: obj(map[string]any{"panel": str("Panel (group) id"), "console": str("Console id or name")}, "panel", "console"),
		run: func(a *App, args map[string]any) (any, error) {
			if err := a.MovePanel(argStr(args, "panel"), argStr(args, "console"), ""); err != nil {
				return nil, err
			}
			return a.CockpitStatus(), nil
		}},
	{Name: "deck_status", Title: "Deck status",
		Description: "What's going on: whether Star Citizen is running and in front, connected decks (devices) and the view each shows, crew, away mode, voice, open requests.",
		InputSchema: obj(map[string]any{}),
		run: func(a *App, _ map[string]any) (any, error) {
			decks := []map[string]any{}
			a.each(func(c *client) {
				c.mu.Lock()
				view := c.view
				c.mu.Unlock()
				kind := "deck"
				if c.p.grant != nil {
					kind = map[bool]string{true: "crew-link", false: "guest"}[c.peer]
				}
				decks = append(decks, map[string]any{"device": c.name(), "id": c.p.deviceID, "kind": kind, "view": view, "local": c.p.local})
			})
			presses, blocked := a.Engine.Counts()
			return map[string]any{"game": a.Engine.Driver().Game(), "decks": decks, "afk": a.Afk(), "crew": a.crew.summary(),
				"voice": a.voice.status(), "requests": a.reqs.open(), "lit": a.Engine.States(), "presses": presses, "blocked": blocked,
				"focusMode": a.Store.Config().FocusMode}, nil
		}},
	{Name: "list_groups", Title: "List control groups",
		Description: "Every group of controls, with how many each has: the ship systems (power, landing, flight, quantum, targeting, mining…), panels (comms, the player's own), and the game's own menus (ids starting game-, holding every game action). Views are built from these. A group with a panel has a designed layout.",
		InputSchema: obj(map[string]any{}),
		run: func(a *App, _ map[string]any) (any, error) {
			all, _ := a.Catalog()
			n, unbound := map[string]int{}, map[string]int{}
			for _, c := range all {
				n[c.Group]++
				if c.Unbound {
					unbound[c.Group]++
				}
			}
			out := []map[string]any{}
			for _, g := range a.Groups() {
				m := map[string]any{"id": g.ID, "code": g.Code, "name": g.Name, "category": g.Category, "controls": n[g.ID]}
				if unbound[g.ID] > 0 {
					m["unbound"] = unbound[g.ID]
				}
				if g.Panel != nil {
					m["panel"] = g.Panel.Source
				}
				if g.Game {
					m["game"] = true
				}
				out = append(out, m)
			}
			return out, nil
		}},
	{Name: "find_controls", Title: "Find controls",
		Description: "Search every control on the deck by words (label, tags, group, voice phrases, Star Citizen action), optionally within one group. Returns ids to press or to put in views. An empty query lists a whole group.",
		InputSchema: obj(map[string]any{"query": str("Words to match, e.g. 'landing gear', 'quantum', 'shields up'"),
			"group": str("Only this group id (see list_groups)"), "limit": num("Most results (default 40)")}),
		run: func(a *App, args map[string]any) (any, error) {
			limit := argNum(args, "limit")
			if limit <= 0 {
				limit = 40
			}
			states := a.Engine.States()
			out := []map[string]any{}
			for _, c := range a.SearchControls(argStr(args, "query"), argStr(args, "group"), limit) {
				out = append(out, controlView(c, states))
			}
			return out, nil
		}},
	{Name: "press", Title: "Press a control",
		Description: "Press a control in Star Citizen. mode tap (default) presses and lets go; hold_start holds it down until hold_end. Only presses while the game is in front (or brings it to front, per the focus setting). One press per call.",
		InputSchema: obj(map[string]any{"control": str("Control id, label or Star Citizen action name"),
			"mode": enum("tap, hold_start or hold_end", "tap", "hold_start", "hold_end"), "ms": num("For a tap, how long to hold (ms)")}, "control"),
		run: func(a *App, args map[string]any) (any, error) {
			c := a.FindControl(argStr(args, "control"))
			if c == nil {
				return nil, fmt.Errorf("no control %q; use find_controls", argStr(args, "control"))
			}
			mode := map[string]string{"hold_start": "down", "hold_end": "up"}[argStr(args, "mode")]
			ms := argNum(args, "ms")
			if said, comms := args["__player"].(string); comms {
				// The Comms agent: dangerous controls only when the player asked in this
				// message, never held, and no long presses.
				if c.Danger && (!reAskedDanger.MatchString(said) || mode == "down") {
					return nil, fmt.Errorf("%s needs the player to ask for it by name; ask them to confirm", c.Label)
				}
				if ms > 1000 {
					ms = 1000
				}
			}
			res, _ := a.agentPress(c, mode, ms)
			return res, nil
		}},
	{Name: "release_all", Title: "Release all keys", Description: "Let go of every key the deck is holding down.",
		InputSchema: obj(map[string]any{}),
		run: func(a *App, _ map[string]any) (any, error) {
			a.Engine.ReleaseAll(deck.ReasonReleaseAll)
			apiHoldsMu.Lock()
			apiHolds = map[string]*deck.Hold{}
			apiHoldsMu.Unlock()
			return map[string]any{"ok": true}, nil
		}},
	{Name: "set_light", Title: "Set a button light",
		Description: "Set whether a toggle (doors, gear, lights…) shows as on. The deck can't read the game, so its lights follow presses; use this to correct one. Give a control or a state group.",
		InputSchema: obj(map[string]any{"control": str("Control id or label"), "group": str("State group, instead of a control"), "on": boolean("On or off")}, "on"),
		run: func(a *App, args map[string]any) (any, error) {
			g := argStr(args, "group")
			if c := a.FindControl(argStr(args, "control")); c != nil && c.State != nil {
				g = c.State.Group
			}
			if g == "" {
				return nil, fmt.Errorf("that control has no light; give a toggle or a group")
			}
			a.Engine.SetState(g, argBool(args, "on"))
			return map[string]any{"ok": true, "group": g}, nil
		}},
	{Name: "list_views", Title: "List views",
		Description: "The views decks can show: presets (flight, combat, mining, salvage, crew, on foot) and the player's own, each a list of groups and sections of controls.",
		InputSchema: obj(map[string]any{}),
		run:         func(a *App, _ map[string]any) (any, error) { return a.Views(), nil }},
	{Name: "show_view", Title: "Show a view on a deck",
		Description: "Put controls on a deck's screen right now: a saved view by id or name, or a one-off view made of groups and/or controls. 'Bring up my flight controls' is view 'flight'. device is a deck's name or part of it (\"ipad\"), or omit for every deck.",
		InputSchema: obj(map[string]any{"view": str("View id or name"), "groups": strs("Group ids for a one-off view"),
			"controls": strs("Control ids or labels for a one-off view"), "title": str("Name for a one-off view"),
			"device": str("Which deck; omit for all")}),
		run: func(a *App, args map[string]any) (any, error) {
			var v config.View
			if ref := argStr(args, "view"); ref != "" {
				var ok bool
				if v, ok = a.ViewByID(ref); !ok {
					return nil, fmt.Errorf("no view %q; use list_views", ref)
				}
			} else {
				v = config.View{ID: "agent-" + config.RandomHex(3), Name: argStr(args, "title"), Groups: argStrs(args, "groups")}
				if v.Name == "" {
					v.Name = "Agent view"
				}
				if cs := argStrs(args, "controls"); len(cs) > 0 {
					v.Sections = []config.Section{{Name: v.Name, Controls: cs}}
				}
				if len(v.Groups) == 0 && len(v.Sections) == 0 {
					return nil, fmt.Errorf("give a view, groups or controls")
				}
			}
			n := a.ShowView(v, argStr(args, "device"))
			if n == 0 {
				return nil, fmt.Errorf("no deck matches %q; see deck_status", argStr(args, "device"))
			}
			return map[string]any{"ok": true, "view": v.Name, "decks": n}, nil
		}},
	{Name: "save_view", Title: "Save a view",
		Description: "Save (or replace, by id) a view: whole groups plus named sections of single controls. Saved views appear in every deck's view bar.",
		InputSchema: obj(map[string]any{"id": str("Id to replace; omit for a new view"), "name": str("Name shown on the deck"),
			"groups": strs("Group ids, in order"),
			"sections": map[string]any{"type": "array", "description": "Named sections of controls", "items": obj(map[string]any{
				"name": str("Section name"), "controls": strs("Control ids or labels")}, "name", "controls")}}, "name"),
		run: func(a *App, args map[string]any) (any, error) {
			v := config.View{ID: argStr(args, "id"), Name: argStr(args, "name"), Groups: argStrs(args, "groups")}
			if secs, ok := args["sections"].([]any); ok {
				for _, s := range secs {
					sm, _ := s.(map[string]any)
					sec := config.Section{Name: argStr(sm, "name")}
					for _, ref := range argStrs(sm, "controls") {
						c := a.FindControl(ref)
						if c == nil {
							return nil, fmt.Errorf("no control %q", ref)
						}
						sec.Controls = append(sec.Controls, c.ID)
					}
					v.Sections = append(v.Sections, sec)
				}
			}
			return a.SaveView(v)
		}},
	{Name: "delete_view", Title: "Delete a view", Description: "Delete one of the player's saved views (presets can't be deleted).",
		InputSchema: obj(map[string]any{"id": str("View id")}, "id"),
		run: func(a *App, args map[string]any) (any, error) {
			return map[string]any{"ok": a.DeleteView(argStr(args, "id"))}, nil
		}},
	{Name: "highlight", Title: "Point at controls",
		Description: "Flash controls on a deck so the player can find them (\"which one is quantum?\"). Shows their group if it isn't on screen.",
		InputSchema: obj(map[string]any{"controls": strs("Control ids or labels"), "device": str("Which deck; omit for all")}, "controls"),
		run: func(a *App, args map[string]any) (any, error) {
			var ids []string
			for _, ref := range argStrs(args, "controls") {
				if c := a.FindControl(ref); c != nil {
					ids = append(ids, c.ID)
				}
			}
			if len(ids) == 0 {
				return nil, fmt.Errorf("no such controls")
			}
			n := 0
			a.each(func(c *client) {
				if c.p.grant == nil && c.matches(argStr(args, "device")) {
					c.send(map[string]any{"t": "highlight", "ids": ids})
					n++
				}
			})
			return map[string]any{"ok": n > 0, "decks": n, "controls": ids}, nil
		}},
	{Name: "set_theme", Title: "Set a deck's theme",
		Description: "Change a deck's manufacturer theme: rsi, drake, aegis, anvil, origin, misc, crusader, banu, aopoa, argo, cnou, esperia.",
		InputSchema: obj(map[string]any{"theme": enum("Theme", "rsi", "drake", "aegis", "anvil", "origin", "misc", "crusader", "banu", "aopoa", "argo", "cnou", "esperia"),
			"device": str("Which deck; omit for all")}, "theme"),
		run: func(a *App, args map[string]any) (any, error) {
			n := 0
			a.each(func(c *client) {
				if c.p.grant == nil && c.matches(argStr(args, "device")) {
					c.send(map[string]any{"t": "theme", "id": argStr(args, "theme")})
					n++
				}
			})
			return map[string]any{"ok": n > 0, "decks": n}, nil
		}},
	{Name: "set_afk", Title: "Away mode", Description: "Turn away (AFK) mode on or off. While on, AFK-only share links let crewmates fly for the player.",
		InputSchema: obj(map[string]any{"on": boolean("Away or back")}, "on"),
		run: func(a *App, args map[string]any) (any, error) {
			a.SetAfk(argBool(args, "on"))
			return map[string]any{"afk": a.Afk()}, nil
		}},
	{Name: "share_controls", Title: "Share controls with someone",
		Description: "Make a share link a friend opens in their browser to press some of the player's controls: whole groups, single controls, or everything. Returns the link(s).",
		InputSchema: obj(map[string]any{"for": str("Who it's for"), "groups": strs("Group ids to share"), "controls": strs("Single controls to share"),
			"all": boolean("Share everything"), "minutes": num("How long; 0 = until revoked"), "afk_only": boolean("Only works while the player is AFK"),
			"taps_only": boolean("No holding keys down")}, "for"),
		run: func(a *App, args map[string]any) (any, error) {
			l, err := a.Layout("tablet", "")
			if err != nil {
				return nil, err
			}
			spec := GrantSpec{Label: argStr(args, "for"), Minutes: argNum(args, "minutes"), All: argBool(args, "all"),
				AfkOnly: argBool(args, "afk_only"), NoHold: argBool(args, "taps_only")}
			// Share links work on the tablet layout's buttons: map groups and controls onto them.
			spec.Buttons = a.ButtonsFor(l, argStrs(args, "groups"), argStrs(args, "controls"))
			if !spec.All && len(spec.Buttons) == 0 {
				return nil, fmt.Errorf("none of those are on the tablet layout; share groups, controls or all")
			}
			g, err := a.CreateGrant(spec)
			if err != nil {
				return nil, err
			}
			return a.grantView(g), nil
		}},
	{Name: "list_share_links", Title: "List share links", Description: "Share links that exist, who they're for, what they cover and who's connected.",
		InputSchema: obj(map[string]any{}),
		run: func(a *App, _ map[string]any) (any, error) {
			out := []map[string]any{}
			for _, g := range a.Store.State().Grants {
				out = append(out, a.grantView(g))
			}
			return out, nil
		}},
	{Name: "revoke_share_link", Title: "Revoke a share link", Description: "End a share link and disconnect whoever uses it.",
		InputSchema: obj(map[string]any{"id": str("Share link id (list_share_links)")}, "id"),
		run: func(a *App, args map[string]any) (any, error) {
			return map[string]any{"ok": a.RevokeGrant(argStr(args, "id"))}, nil
		}},
	{Name: "message_crew", Title: "Message a crewmate",
		Description: "Send a crewmate a request or message (\"brb, keep us in orbit\"). It shows on their deck and is read aloud if they use voice.",
		InputSchema: obj(map[string]any{"to": str("Crewmate's name (see deck_status crew)"), "text": str("The message, up to 140 characters")}, "to", "text"),
		run: func(a *App, args map[string]any) (any, error) {
			to := strings.ToLower(argStr(args, "to"))
			for _, c := range a.crew.summary() {
				name, _ := c["name"].(string)
				if strings.Contains(strings.ToLower(name), to) {
					sink := &client{a: a, out: make(chan []byte, 4)}
					a.crew.request(c["id"].(string), argStr(args, "text"), "", sink)
					return map[string]any{"ok": true, "to": name}, nil
				}
			}
			return nil, fmt.Errorf("no crewmate %q is connected", argStr(args, "to"))
		}},
	{Name: "answer_request", Title: "Answer a request",
		Description: "Answer a crewmate's request (deck_status lists them): do=true presses what they asked for (or acknowledges a message), false declines.",
		InputSchema: obj(map[string]any{"id": str("Request id"), "do": boolean("Do it, or decline")}, "id", "do"),
		run: func(a *App, args map[string]any) (any, error) {
			sink := &client{a: a, out: make(chan []byte, 4), p: principal{owner: true, name: "Agent"}}
			a.answerRequest(sink, argStr(args, "id"), argBool(args, "do"))
			return map[string]any{"ok": true}, nil
		}},
	{Name: "say", Title: "Speak through the PC",
		Description: "Say something out loud on the PC's speakers with the deck's voice (Pocket TTS). Spell numbers out as words.",
		InputSchema: obj(map[string]any{"text": str("What to say")}, "text"),
		run: func(a *App, args map[string]any) (any, error) {
			n := a.voice.Native()
			if n == nil || n.sp == nil {
				return nil, fmt.Errorf("the native voice isn't running (no Pocket TTS model found)")
			}
			n.announce(argStr(args, "text"))
			return map[string]any{"ok": true}, nil
		}},
}

// agentPress presses a catalog control for an agent or the API, tracking holds.
func (a *App) agentPress(c *Control, mode string, ms int) (map[string]any, int) {
	src := deck.Source{Kind: "agent", Who: "Agent", Local: true}
	b := c.Button
	switch mode {
	case "down":
		apiHoldsMu.Lock()
		_, held := apiHolds[b.ID]
		apiHoldsMu.Unlock()
		if held {
			return map[string]any{"ok": true, "control": c.ID, "held": true, "note": "already held"}, 200
		}
		var h *deck.Hold
		var r deck.Result
		h, r = a.Engine.Down(b, src, func(string) {
			apiHoldsMu.Lock()
			if apiHolds[b.ID] == h {
				delete(apiHolds, b.ID)
				delete(agentHolds, b.ID)
			}
			apiHoldsMu.Unlock()
		})
		if h != nil {
			apiHoldsMu.Lock()
			apiHolds[b.ID] = h
			agentHolds[b.ID] = true
			apiHoldsMu.Unlock()
		}
		return map[string]any{"ok": r.OK, "reason": r.Reason, "control": c.ID, "held": h != nil}, 200
	case "up":
		apiHoldsMu.Lock()
		h := apiHolds[b.ID]
		delete(apiHolds, b.ID)
		apiHoldsMu.Unlock()
		a.Engine.Up(h)
		return map[string]any{"ok": true, "control": c.ID}, 200
	}
	r := a.Engine.Tap(b, src, ms, "agent")
	out := map[string]any{"ok": r.OK, "control": c.ID, "label": c.Label, "keys": c.Keys}
	if !r.OK {
		out["reason"] = r.Reason
	}
	if c.State != nil {
		out["on"] = a.Engine.States()[c.State.Group]
	}
	return out, 200
}

// agentHolds are the apiHolds an agent started (by button id).
var agentHolds = map[string]bool{}

// releaseAgentHolds lets go of everything the Comms agent is holding when its turn ends.
func (a *App) releaseAgentHolds() {
	apiHoldsMu.Lock()
	var hs []*deck.Hold
	for id := range agentHolds {
		if h := apiHolds[id]; h != nil {
			hs = append(hs, h)
			delete(apiHolds, id)
		}
		delete(agentHolds, id)
	}
	apiHoldsMu.Unlock()
	for _, h := range hs {
		a.Engine.Up(h)
	}
}

// handleMCP answers MCP requests (JSON-RPC 2.0 over HTTP POST).
func (a *App) handleMCP(w http.ResponseWriter, r *http.Request, pr principal) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "MCP takes POST", http.StatusMethodNotAllowed)
		return
	}
	var raw json.RawMessage
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&raw); err != nil {
		writeJSON(w, 400, map[string]any{"jsonrpc": "2.0", "id": nil, "error": rpcErr{-32700, "parse error"}})
		return
	}
	// A batch (older clients) or one message.
	if len(raw) > 0 && raw[0] == '[' {
		var reqs []rpcReq
		if json.Unmarshal(raw, &reqs) != nil {
			writeJSON(w, 400, map[string]any{"jsonrpc": "2.0", "id": nil, "error": rpcErr{-32600, "invalid request"}})
			return
		}
		var out []any
		for _, q := range reqs {
			if resp := a.mcpCall(q, pr.local); resp != nil {
				out = append(out, resp)
			}
		}
		if len(out) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeJSON(w, 200, out)
		return
	}
	var q rpcReq
	if json.Unmarshal(raw, &q) != nil {
		writeJSON(w, 400, map[string]any{"jsonrpc": "2.0", "id": nil, "error": rpcErr{-32600, "invalid request"}})
		return
	}
	resp := a.mcpCall(q, pr.local)
	if resp == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeJSON(w, 200, resp)
}

// mcpCall runs one JSON-RPC message; nil for notifications.
// localOnlyTools change the game's files or the PC's settings: only from the PC itself.
var localOnlyTools = map[string]bool{"bind_all_actions": true, "set_control_keys": true}

// mcpCall runs one JSON-RPC message; nil for notifications. local is a caller on this PC.
func (a *App) mcpCall(q rpcReq, local bool) map[string]any {
	if len(q.ID) == 0 { // notification
		return nil
	}
	ok := func(result any) map[string]any { return map[string]any{"jsonrpc": "2.0", "id": q.ID, "result": result} }
	fail := func(code int, msg string) map[string]any {
		return map[string]any{"jsonrpc": "2.0", "id": q.ID, "error": rpcErr{code, msg}}
	}
	switch q.Method {
	case "initialize":
		return ok(map[string]any{
			"protocolVersion": mcpProtocol,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "tabletdeck", "title": "Tablet Deck", "version": tabletdeck.Version},
			"instructions": "Tablet Deck is the player's Star Citizen control deck. Controls are grouped by ship system; " +
				"use find_controls to get ids (every game action is a control, id sc.<action>), press to press them, show_view to change what a deck shows, save_panel to build a panel (read panel_guide first) " +
				"(\"bring up my flight controls\" = show_view view=flight). Presses only reach the game while it's in front. " +
				"Never press danger controls (eject, self-destruct, jettison) unless the player clearly asked for that exact thing.",
		})
	case "ping":
		return ok(map[string]any{})
	case "tools/list":
		return ok(map[string]any{"tools": mcpTools})
	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if json.Unmarshal(q.Params, &p) != nil {
			return fail(-32602, "invalid params")
		}
		if p.Arguments == nil {
			p.Arguments = map[string]any{}
		}
		for k := range p.Arguments {
			if strings.HasPrefix(k, "__") {
				delete(p.Arguments, k) // internal fields aren't the caller's to set
			}
		}
		for _, t := range mcpTools {
			if t.Name != p.Name {
				continue
			}
			if localOnlyTools[t.Name] && !local {
				return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": t.Name + " only works from the PC itself"}}, "isError": true})
			}
			if req, _ := t.InputSchema["required"].([]string); len(req) > 0 {
				for _, k := range req {
					if v, has := p.Arguments[k]; !has || v == nil || v == "" {
						return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": "missing argument " + k}}, "isError": true})
					}
				}
			}
			t0 := time.Now()
			res, err := t.run(a, p.Arguments)
			a.Bus.Publish("agent", map[string]any{"tool": p.Name, "args": p.Arguments, "ok": err == nil, "ms": time.Since(t0).Milliseconds()})
			if err != nil {
				return ok(map[string]any{"content": []any{map[string]any{"type": "text", "text": err.Error()}}, "isError": true})
			}
			b, _ := json.Marshal(res)
			out := map[string]any{"content": []any{map[string]any{"type": "text", "text": string(b)}}}
			if m, isObj := res.(map[string]any); isObj {
				out["structuredContent"] = m
			}
			return ok(out)
		}
		return fail(-32602, "unknown tool "+p.Name)
	case "resources/list":
		return ok(map[string]any{"resources": []any{}})
	case "prompts/list":
		return ok(map[string]any{"prompts": []any{}})
	}
	return fail(-32601, "method not found: "+q.Method)
}
