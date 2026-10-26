// Package config keeps Tablet Deck's settings (config.json) and its saved state
// (state.json: paired devices, share links, crew links) in the data folder.
package config

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// DefaultPort is high and uncommon so it rarely clashes with anything else.
const DefaultPort = 47615

type Link struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Config is config.json. Field names match Tablet Deck 1.x where they existed, so an
// old config.json keeps working.
type Config struct {
	Name        string `json:"name"` // this deck's name, shown to crewmates
	Port        int    `json:"port"`
	GuestPort   int    `json:"guestPort"` // local-only listener a tunnel points at
	Token       string `json:"token"`     // the pairing code
	APIKey      string `json:"apiKey"`    // for Stream Deck and other local tools
	Host        string `json:"host"`      // address for the QR code; "" = automatic
	Layout      string `json:"layout"`
	PhoneLayout string `json:"phoneLayout"`

	// Keys
	FocusMode           string   `json:"focusMode"` // strict, local, always, off
	OnlyWhenGameFocused *bool    `json:"onlyWhenGameFocused,omitempty"`
	TapMs               int      `json:"tapMs"`
	ComboGapMs          int      `json:"comboGapMs"`
	SettleMs            int      `json:"settleMs"`
	MaxHoldMs           int      `json:"maxHoldMs"`
	ReleaseOnFocusLoss  bool     `json:"releaseOnFocusLoss"`
	GameProcesses       []string `json:"gameProcesses"`

	// Keybinds
	FollowGameBinds bool     `json:"followGameBinds"` // read the game's actionmaps.xml
	Actionmaps      []string `json:"actionmaps"`      // extra keybind files, later win
	GameDirs        []string `json:"gameDirs"`        // install folders found by hand

	// Voice
	Voice           bool    `json:"voice"`
	VoiceLanguage   string  `json:"voiceLanguage"`
	VoiceConfidence float64 `json:"voiceConfidence"`
	ShortEmotes     bool    `json:"shortEmotes"`
	PttKey          int     `json:"pttKey"`
	PttKeyName      string  `json:"pttKeyName"`
	PttMode         string  `json:"pttMode"`
	PttSound        *bool   `json:"pttSound,omitempty"`

	// Crew and sharing
	Discovery bool   `json:"discovery"` // find other decks on the network
	Tunnel    string `json:"tunnel"`    // "" or "cloudflare": share links reach the internet

	Links      []Link `json:"links"`
	AutoUpdate bool   `json:"autoUpdate"`
}

func defaults() Config {
	host, _ := os.Hostname()
	return Config{
		Name: host, Port: DefaultPort, Layout: "layouts/hud.json", PhoneLayout: "layouts/hud-phone.json",
		FocusMode: "local", TapMs: 60, ComboGapMs: 12, SettleMs: 45, MaxHoldMs: 120000, ReleaseOnFocusLoss: true,
		GameProcesses: []string{"StarCitizen.exe"}, FollowGameBinds: true,
		Actionmaps: []string{"keybinds/TabletDeck-StarCitizen.xml"},
		Voice: true, VoiceLanguage: "en-US", VoiceConfidence: 0.6, PttMode: "hold",
		Discovery: true, AutoUpdate: true,
		Links: []Link{{"UEX", "https://uexcorp.space/"}, {"Erkul", "https://www.erkul.games/"},
			{"SPViewer", "https://www.spviewer.eu/"}, {"SCMDB", "https://scmdb.net/"}},
	}
}

// DataDir is %APPDATA%\TabletDeck on Windows, ~/.config/tabletdeck elsewhere.
func DataDir() string {
	if d := os.Getenv("TABLETDECK_DATA"); d != "" {
		return d
	}
	if runtime.GOOS == "windows" {
		if a := os.Getenv("APPDATA"); a != "" {
			return filepath.Join(a, "TabletDeck")
		}
	}
	d, err := os.UserConfigDir()
	if err != nil {
		d, _ = os.UserHomeDir()
	}
	return filepath.Join(d, "tabletdeck")
}

// RandomHex returns n random bytes as hex.
func RandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Hash is how tokens are stored: never in the clear.
func Hash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// Store holds the live config and state and saves them.
type Store struct {
	Dir string

	mu    sync.RWMutex
	cfg   Config
	state State
}

// Open reads (or creates) config.json and state.json in dir.
func Open(dir string) (*Store, error) {
	s := &Store{Dir: dir}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	cfg := defaults()
	raw, err := os.ReadFile(s.configPath())
	if err == nil {
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return nil, fmt.Errorf("config.json: %w", err)
		}
	}
	// 1.x settings carried over.
	if cfg.OnlyWhenGameFocused != nil && !*cfg.OnlyWhenGameFocused {
		cfg.FocusMode = "off"
	}
	cfg.OnlyWhenGameFocused = nil
	// A 1.x config keeps its port (8765), since the player's devices know it.
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	if cfg.GuestPort == 0 {
		cfg.GuestPort = cfg.Port + 1
	}
	cfg.Token = strings.ToLower(strings.TrimSpace(cfg.Token))
	if cfg.Token == "" {
		cfg.Token = RandomHex(4)
	}
	if cfg.APIKey == "" {
		cfg.APIKey = RandomHex(16)
	}
	s.cfg = cfg
	if raw, err := os.ReadFile(s.statePath()); err == nil {
		_ = json.Unmarshal(raw, &s.state)
	}
	if s.state.InstanceID == "" {
		s.state.InstanceID = RandomHex(8)
	}
	return s, s.save()
}

func (s *Store) configPath() string { return filepath.Join(s.Dir, "config.json") }
func (s *Store) statePath() string  { return filepath.Join(s.Dir, "state.json") }

// ConfigPath is shown to the player.
func (s *Store) ConfigPath() string { return s.configPath() }

func (s *Store) save() error {
	c, _ := json.MarshalIndent(s.cfg, "", "  ")
	if err := writeAtomic(s.configPath(), append(c, '\n')); err != nil {
		return err
	}
	st, _ := json.MarshalIndent(s.state, "", "  ")
	return writeAtomic(s.statePath(), append(st, '\n'))
}

// writeAtomic writes through a temp file, so a crash never leaves half a config.
func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Config returns a copy of the settings.
func (s *Store) Config() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := s.cfg
	c.Links = append([]Link(nil), s.cfg.Links...)
	return c
}

// Update changes settings and saves them.
func (s *Store) Update(fn func(*Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.cfg)
	return s.save()
}

var (
	reToken = regexp.MustCompile(`^[a-z0-9]{4,32}$`)
	reLang  = regexp.MustCompile(`^[a-z]{2}(-[A-Za-z]{2,4})?$`)
	reHost  = regexp.MustCompile(`^$|^(\d{1,3}\.){3}\d{1,3}$|^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$`)
)

// Validate checks settings sent from the control panel.
func (c *Config) Validate() error {
	switch {
	case c.Port < 1024 || c.Port > 65535:
		return fmt.Errorf("port must be 1024-65535")
	case c.GuestPort < 1024 || c.GuestPort > 65535 || c.GuestPort == c.Port:
		return fmt.Errorf("guest port must be 1024-65535 and not the deck's port")
	case !reToken.MatchString(c.Token):
		return fmt.Errorf("pairing code: 4-32 lowercase letters and digits")
	case c.TapMs < 20 || c.TapMs > 1000:
		return fmt.Errorf("tap length must be 20-1000 ms")
	case c.ComboGapMs < 0 || c.ComboGapMs > 200:
		return fmt.Errorf("combo gap must be 0-200 ms")
	case c.SettleMs < 0 || c.SettleMs > 1000:
		return fmt.Errorf("settle time must be 0-1000 ms")
	case !reLang.MatchString(c.VoiceLanguage):
		return fmt.Errorf("voice language like en-US")
	case c.VoiceConfidence < 0 || c.VoiceConfidence > 1:
		return fmt.Errorf("voice strictness must be 0-1")
	case !reHost.MatchString(c.Host):
		return fmt.Errorf("host must be an IP address or host name")
	case len(c.Links) > 12:
		return fmt.Errorf("at most 12 links")
	case c.Tunnel != "" && c.Tunnel != "cloudflare":
		return fmt.Errorf("tunnel must be empty or cloudflare")
	}
	switch c.FocusMode {
	case "strict", "local", "always", "off":
	default:
		return fmt.Errorf("focus mode must be strict, local, always or off")
	}
	for _, l := range c.Links {
		if l.Name == "" || len(l.Name) > 24 || !strings.HasPrefix(l.URL, "https://") || len(l.URL) > 300 {
			return fmt.Errorf("links need a name and an https:// address")
		}
	}
	return nil
}

// ---------- state.json ----------

// Device is a paired tablet or phone. Its token is only kept hashed.
type Device struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	TokenHash string    `json:"tokenHash"`
	Paired    time.Time `json:"paired"`
	LastSeen  time.Time `json:"lastSeen"`
}

// Grant is a share link: someone else's deck may press some of this deck's buttons.
type Grant struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`  // who it's for: "Mike"
	Kind      string    `json:"kind"`   // share (a link for a friend) or crew (another PC linked to this one)
	Token     string    `json:"token"`  // in the link; random and long
	Layout    string    `json:"layout"` // whose buttons the guest sees
	Buttons   []string  `json:"buttons,omitempty"`
	Panels    []string  `json:"panels,omitempty"`
	All       bool      `json:"all"`
	AfkOnly   bool      `json:"afkOnly"` // only works while the owner is away
	NoHold    bool      `json:"noHold"`  // taps only
	PerMinute int       `json:"perMinute"`
	Created   time.Time `json:"created"`
	Expires   time.Time `json:"expires,omitempty"` // zero = until revoked
	Presses   int       `json:"presses"`
}

// Live reports whether the grant can be used now.
func (g *Grant) Live(now time.Time) bool { return g.Expires.IsZero() || now.Before(g.Expires) }

// CrewLink is another deck this one is linked to, through a share link it gave us.
type CrewLink struct {
	ID    string    `json:"id"`
	Name  string    `json:"name"`
	URL   string    `json:"url"` // their share link
	Added time.Time `json:"added"`
}

// State is state.json.
type State struct {
	InstanceID string     `json:"instanceId"`
	Devices    []Device   `json:"devices"`
	Grants     []Grant    `json:"grants"`
	Crew       []CrewLink `json:"crew"`
}

// State returns a copy of the saved state.
func (s *Store) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := s.state
	st.Devices = append([]Device(nil), s.state.Devices...)
	st.Grants = append([]Grant(nil), s.state.Grants...)
	st.Crew = append([]CrewLink(nil), s.state.Crew...)
	return st
}

// UpdateState changes the saved state and saves it.
func (s *Store) UpdateState(fn func(*State)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.state)
	return s.save()
}

// PruneGrants drops share links that ran out over a day ago.
func (s *Store) PruneGrants() {
	now := time.Now()
	_ = s.UpdateState(func(st *State) {
		kept := st.Grants[:0]
		for _, g := range st.Grants {
			if g.Expires.IsZero() || now.Sub(g.Expires) < 24*time.Hour {
				kept = append(kept, g)
			}
		}
		st.Grants = kept
	})
}
