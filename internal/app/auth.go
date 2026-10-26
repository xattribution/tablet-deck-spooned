package app

import (
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/xattribution/tablet-deck-spooned/internal/config"
)

// Who is asking, once their credentials checked out.
type principal struct {
	owner    bool          // the deck's owner: a paired device, the pairing code, or this PC with the API key
	local    bool          // the request came from this PC, on the deck port
	byCode   bool          // used the pairing code (a device token is minted for it)
	deviceID string        // a paired device
	name     string        // device or guest name
	grant    *config.Grant // a guest's share link
}

// manager reports whether the caller may change settings, keybind files, the agent
// and tunnels: this PC, or a paired owner device. Never a guest on a share link.
func (p principal) manager() bool {
	return p.grant == nil && (p.local || p.owner)
}

// isLocal: from this PC, on the deck port. The guest port is also on this PC but is
// where tunnels connect, so nothing there counts as local.
func isLocal(r *http.Request, guestListener bool) bool {
	if guestListener {
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func remoteIP(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func eq(a, b string) bool {
	return a != "" && len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// credential is the token a request carries: ?t= (pairing code or device token), ?k=
// (API key), or an Authorization: Bearer header.
func credential(r *http.Request) string {
	if t := r.URL.Query().Get("t"); t != "" {
		return strings.ToLower(strings.TrimSpace(t))
	}
	if k := r.URL.Query().Get("k"); k != "" {
		return k
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return r.Header.Get("X-Deck-Key")
}

// authOwner checks owner credentials. It returns the principal and 0, or the HTTP
// status to refuse with (403 wrong, 429 too many wrong tries).
func (a *App) authOwner(r *http.Request, guestListener bool) (principal, int) {
	if guestListener {
		return principal{}, http.StatusForbidden
	}
	local := isLocal(r, false)
	cred := credential(r)
	ip := remoteIP(r)
	if !local && a.pair.locked(ip) {
		return principal{}, http.StatusTooManyRequests
	}
	cfg := a.Store.Config()
	switch {
	case eq(cred, cfg.Token):
		a.pair.ok(ip)
		return principal{owner: true, local: local, byCode: true, name: deviceName(r)}, 0
	case local && eq(cred, cfg.APIKey):
		return principal{owner: true, local: true, name: "This PC"}, 0
	}
	if cred != "" {
		h := config.Hash(cred)
		for _, d := range a.Store.State().Devices {
			if eq(h, d.TokenHash) {
				a.pair.ok(ip)
				return principal{owner: true, local: local, deviceID: d.ID, name: d.Name}, 0
			}
		}
	}
	if local {
		return principal{}, http.StatusForbidden
	}
	if a.pair.fail(ip) {
		a.Log.Printf("Too many wrong pairing codes from %s; ignoring it for a minute.", ip)
		return principal{}, http.StatusTooManyRequests
	}
	return principal{}, http.StatusForbidden
}

// authGuest checks a share link token (?g=). It works on both ports.
func (a *App) authGuest(r *http.Request) (principal, int) {
	tok := r.URL.Query().Get("g")
	ip := remoteIP(r)
	if a.pair.locked(ip) {
		return principal{}, http.StatusTooManyRequests
	}
	now := time.Now()
	for _, g := range a.Store.State().Grants {
		if eq(tok, g.Token) {
			if !g.Live(now) {
				return principal{}, http.StatusGone
			}
			g := g
			return principal{grant: &g, name: g.Label}, 0
		}
	}
	if a.pair.fail(ip) {
		return principal{}, http.StatusTooManyRequests
	}
	return principal{}, http.StatusForbidden
}

// deviceName guesses a friendly name from the browser ("iPad", "Android phone").
func deviceName(r *http.Request) string {
	if n := strings.TrimSpace(r.URL.Query().Get("name")); n != "" && len(n) <= 40 {
		return n
	}
	ua := r.UserAgent()
	switch {
	case strings.Contains(ua, "iPad"):
		return "iPad"
	case strings.Contains(ua, "iPhone"):
		return "iPhone"
	case strings.Contains(ua, "Android") && strings.Contains(ua, "Mobile"):
		return "Android phone"
	case strings.Contains(ua, "Android"):
		return "Android tablet"
	case strings.Contains(ua, "Macintosh"):
		return "Mac or iPad" // iPadOS asks for the desktop site
	case strings.Contains(ua, "Windows"):
		return "Windows PC"
	}
	return "Device"
}

// mintDevice pairs a new device and returns its token, which only the device keeps.
func (a *App) mintDevice(name string) (config.Device, string) {
	tok := config.RandomHex(24)
	d := config.Device{ID: config.RandomHex(6), Name: name, TokenHash: config.Hash(tok), Paired: time.Now(), LastSeen: time.Now()}
	_ = a.Store.UpdateState(func(s *config.State) { s.Devices = append(s.Devices, d) })
	a.Log.Printf("Paired a new device: %s.", name)
	a.Bus.Publish("devices", nil)
	return d, tok
}

func (a *App) touchDevice(id string) {
	if id == "" {
		return
	}
	_ = a.Store.UpdateState(func(s *config.State) {
		for i := range s.Devices {
			if s.Devices[i].ID == id {
				s.Devices[i].LastSeen = time.Now()
			}
		}
	})
}

// ---------- wrong code limiter ----------

// After 5 wrong codes from one address within a minute, that address is refused for a
// minute, even with the right code, so nothing on the network can keep guessing.
type limiter struct {
	mu    sync.Mutex
	fails map[string]*failRec
}

type failRec struct {
	count       int
	first, lock time.Time
}

func newLimiter() *limiter { return &limiter{fails: map[string]*failRec{}} }

func (l *limiter) locked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.fails[ip]
	return f != nil && time.Now().Before(f.lock)
}

func (l *limiter) ok(ip string) {
	l.mu.Lock()
	delete(l.fails, ip)
	l.mu.Unlock()
}

// fail records a wrong try and reports whether the address is now locked out.
func (l *limiter) fail(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	f := l.fails[ip]
	if f == nil || now.Sub(f.first) > time.Minute {
		f = &failRec{first: now}
		l.fails[ip] = f
	}
	f.count++
	if f.count >= 5 {
		f.lock = now.Add(time.Minute)
	}
	if len(l.fails) > 1000 {
		for k, v := range l.fails {
			if now.After(v.lock) && now.Sub(v.first) > time.Minute {
				delete(l.fails, k)
			}
		}
	}
	return f.count >= 5
}
