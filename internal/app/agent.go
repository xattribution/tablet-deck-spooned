package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// The agent: a language model the player talks to from the deck (by voice or text),
// through any OpenAI-compatible API (xAI, OpenAI, OpenRouter, a local Ollama or LM
// Studio). Two styles:
//   fast: short answers and a few tools (press, find, views, release), one round;
//   crew: every deck tool (the same ones MCP agents get), several rounds, for
//         "set up my mining view and tell Mike I'm on my way".
// Its replies are spoken by the deck's voice and shown on the Comms panel.

type chatLine struct {
	Who  string    `json:"who"` // you, agent, system
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

type agentState struct {
	mu      sync.Mutex
	log     []chatLine
	history []map[string]any // the conversation sent to the model (user and assistant turns)
	busy    bool
}

var fastTools = map[string]bool{"press": true, "find_controls": true, "show_view": true, "release_all": true, "set_afk": true, "deck_status": true, "message_crew": true}

func (a *App) agentReady() bool {
	cfg := a.Store.Config()
	return cfg.AgentURL != "" && cfg.AgentModel != ""
}

func (a *App) agentInfo() map[string]any {
	cfg := a.Store.Config()
	return map[string]any{"ready": a.agentReady(), "name": cfg.AgentName, "style": cfg.AgentStyle, "route": cfg.VoiceRoute}
}

func (a *App) chatLog() []chatLine {
	a.agent.mu.Lock()
	defer a.agent.mu.Unlock()
	return append([]chatLine(nil), a.agent.log...)
}

func (a *App) addChat(who, text string) {
	l := chatLine{Who: who, Text: text, At: time.Now()}
	a.agent.mu.Lock()
	a.agent.log = append(a.agent.log, l)
	if len(a.agent.log) > 40 {
		a.agent.log = a.agent.log[len(a.agent.log)-40:]
	}
	a.agent.mu.Unlock()
	a.broadcastOwners(map[string]any{"t": "chat", "line": l})
	a.Bus.Publish("chat", l)
}

// ClearChat starts a new conversation.
func (a *App) ClearChat() {
	a.agent.mu.Lock()
	a.agent.history, a.agent.log = nil, nil
	a.agent.mu.Unlock()
	a.broadcastOwners(map[string]any{"t": "chat-clear"})
}

// AskAgent sends what the player said or typed to the agent and speaks its answer.
// It returns at once; the answer arrives as a chat line.
func (a *App) AskAgent(text, via string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if !a.agentReady() {
		a.addChat("system", "No agent set up. Add one on the PC: Comms > Agent.")
		return
	}
	a.agent.mu.Lock()
	if a.agent.busy {
		a.agent.mu.Unlock()
		a.addChat("system", "Still answering the last one.")
		return
	}
	a.agent.busy = true
	a.agent.mu.Unlock()
	a.addChat("you", text)
	a.broadcastOwners(map[string]any{"t": "agent-busy", "on": true})
	go func() {
		defer func() {
			a.agent.mu.Lock()
			a.agent.busy = false
			a.agent.mu.Unlock()
			a.broadcastOwners(map[string]any{"t": "agent-busy", "on": false})
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		reply, err := a.runAgent(ctx, text)
		if err != nil {
			a.Log.Printf("Agent: %v", err)
			a.addChat("system", "The agent didn't answer: "+err.Error())
			return
		}
		if reply == "" {
			reply = "Done."
		}
		a.addChat("agent", reply)
		if n := a.voice.Native(); n != nil {
			n.announce(reply)
		} else {
			a.voice.say(reply)
		}
	}()
}

func (a *App) agentTools(style string) []map[string]any {
	var out []map[string]any
	for _, t := range mcpTools {
		if t.Name == "say" || (style != "crew" && !fastTools[t.Name]) {
			continue
		}
		out = append(out, map[string]any{"type": "function", "function": map[string]any{
			"name": t.Name, "description": t.Description, "parameters": t.InputSchema}})
	}
	return out
}

func (a *App) runAgent(ctx context.Context, text string) (string, error) {
	cfg := a.Store.Config()
	name := cfg.AgentName
	if name == "" {
		name = "Crew"
	}
	sys := fmt.Sprintf("You are %s, the player's crew AI in Star Citizen, working through Tablet Deck, their control deck. "+
		"Your replies are spoken aloud: one or two short sentences, no lists, no markdown. "+
		"To act, use the tools: find_controls to get a control's id, then press it; show_view to change what a deck shows. "+
		"Never press self-destruct or eject unless the player clearly asks for it in this message. "+
		"Right now: %s", name, a.spokenStatus())
	if cfg.AgentPrompt != "" {
		sys += "\n\nThe player's instructions: " + cfg.AgentPrompt
	}
	a.agent.mu.Lock()
	hist := append([]map[string]any(nil), a.agent.history...)
	a.agent.mu.Unlock()
	msgs := append([]map[string]any{{"role": "system", "content": sys}}, hist...)
	msgs = append(msgs, map[string]any{"role": "user", "content": text})
	tools := a.agentTools(cfg.AgentStyle)
	rounds := 1
	if cfg.AgentStyle == "crew" {
		rounds = 6
	}
	var reply string
	for round := 0; ; round++ {
		withTools := round < rounds
		msg, err := a.chatCompletion(ctx, cfg.AgentURL, cfg.AgentKey, cfg.AgentModel, msgs, tools, withTools)
		if err != nil {
			return "", err
		}
		calls, _ := msg["tool_calls"].([]any)
		if len(calls) == 0 || !withTools {
			reply, _ = msg["content"].(string)
			break
		}
		msgs = append(msgs, msg)
		for _, c := range calls {
			cm, _ := c.(map[string]any)
			fn, _ := cm["function"].(map[string]any)
			tname, _ := fn["name"].(string)
			argText, _ := fn["arguments"].(string)
			args := map[string]any{}
			_ = json.Unmarshal([]byte(argText), &args)
			result := a.runTool(tname, args)
			id, _ := cm["id"].(string)
			msgs = append(msgs, map[string]any{"role": "tool", "tool_call_id": id, "content": result})
		}
	}
	reply = strings.TrimSpace(reply)
	a.agent.mu.Lock()
	a.agent.history = append(a.agent.history, map[string]any{"role": "user", "content": text}, map[string]any{"role": "assistant", "content": reply})
	if len(a.agent.history) > 16 {
		a.agent.history = a.agent.history[len(a.agent.history)-16:]
	}
	a.agent.mu.Unlock()
	return reply, nil
}

// runTool runs one deck tool for the agent, the same way MCP does.
func (a *App) runTool(name string, args map[string]any) string {
	for _, t := range mcpTools {
		if t.Name != name {
			continue
		}
		a.Bus.Publish("agent", map[string]any{"tool": name, "args": args, "via": "comms"})
		a.broadcastOwners(map[string]any{"t": "agent", "tool": name})
		res, err := t.run(a, args)
		if err != nil {
			return "error: " + err.Error()
		}
		b, _ := json.Marshal(res)
		if len(b) > 6000 {
			b = append(b[:6000], []byte("…")...)
		}
		return string(b)
	}
	return "error: no tool " + name
}

func (a *App) chatCompletion(ctx context.Context, base, key, model string, msgs []map[string]any, tools []map[string]any, withTools bool) (map[string]any, error) {
	body := map[string]any{"model": model, "messages": msgs, "max_tokens": 400}
	if withTools && len(tools) > 0 {
		body["tools"] = tools
	}
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(base, "/")+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(raw[:min(len(raw), 300)])))
	}
	var out struct {
		Choices []struct {
			Message map[string]any `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &out); err != nil || len(out.Choices) == 0 {
		return nil, fmt.Errorf("unexpected answer from the agent's API")
	}
	m := out.Choices[0].Message
	if m["role"] == nil {
		m["role"] = "assistant"
	}
	return m, nil
}
