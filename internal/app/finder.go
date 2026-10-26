package app

import (
	"context"
	"encoding/json"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	tabletdeck "github.com/xattribution/tablet-deck-spooned"
)

// DiscoveryPort carries a small UDP broadcast every few seconds, so decks on the same
// network find each other without typing addresses. It says only a name, a port and a
// random id: no pairing code, nothing to press with.
const DiscoveryPort = 47619

type beacon struct {
	App  string `json:"app"` // "tabletdeck"
	ID   string `json:"id"`
	Name string `json:"name"`
	Port int    `json:"port"`
	Ver  string `json:"ver"`
}

// Nearby is another deck on the network.
type Nearby struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Address  string    `json:"address"`
	Version  string    `json:"version"`
	LastSeen time.Time `json:"lastSeen"`
}

type finder struct {
	a    *App
	mu   sync.Mutex
	seen map[string]*Nearby
	conn *net.UDPConn
	kick chan struct{}
}

func newFinder(a *App) *finder {
	return &finder{a: a, seen: map[string]*Nearby{}, kick: make(chan struct{}, 1)}
}

func (f *finder) start(ctx context.Context) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: DiscoveryPort})
	if err != nil {
		f.a.Log.Printf("Finding other decks is off: %v", err)
		return
	}
	f.conn = conn
	go func() { <-ctx.Done(); conn.Close() }()
	go f.listen()
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			f.send()
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			case <-f.kick:
			}
		}
	}()
}

func (f *finder) announce() {
	select {
	case f.kick <- struct{}{}:
	default:
	}
}

func (f *finder) send() {
	cfg := f.a.Store.Config()
	if f.conn == nil || !cfg.Discovery {
		return
	}
	b, _ := json.Marshal(beacon{App: "tabletdeck", ID: f.a.Store.State().InstanceID, Name: cfg.Name, Port: cfg.Port, Ver: tabletdeck.Version})
	for _, bc := range broadcastAddrs() {
		_, _ = f.conn.WriteToUDP(b, &net.UDPAddr{IP: bc, Port: DiscoveryPort})
	}
}

func (f *finder) listen() {
	buf := make([]byte, 1024)
	self := f.a.Store.State().InstanceID
	for {
		n, from, err := f.conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		var b beacon
		if json.Unmarshal(buf[:n], &b) != nil || b.App != "tabletdeck" || b.ID == self || b.Port <= 0 || b.Port > 65535 {
			continue
		}
		if len(b.Name) > 40 {
			b.Name = b.Name[:40]
		}
		addr := (&net.TCPAddr{IP: from.IP, Port: b.Port}).String()
		f.mu.Lock()
		_, known := f.seen[b.ID]
		f.seen[b.ID] = &Nearby{ID: b.ID, Name: b.Name, Address: addr, Version: b.Ver, LastSeen: time.Now()}
		f.mu.Unlock()
		if !known {
			f.a.Bus.Publish("nearby", nil)
		}
	}
}

// List returns decks heard from in the last 20 seconds.
func (f *finder) List() []Nearby {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []Nearby{}
	for id, n := range f.seen {
		if time.Since(n.LastSeen) > 20*time.Second {
			delete(f.seen, id)
			continue
		}
		out = append(out, *n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// broadcastAddrs are the directed broadcast addresses of this PC's real networks.
func broadcastAddrs() []net.IP {
	var out []net.IP
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagBroadcast == 0 || ifc.Flags&net.FlagLoopback != 0 || virtualAdapter(ifc.Name) {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, ad := range addrs {
			ipn, ok := ad.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil {
				continue
			}
			ip, mask := ipn.IP.To4(), ipn.Mask
			if len(mask) == 16 {
				mask = mask[12:]
			}
			bc := make(net.IP, 4)
			for i := range bc {
				bc[i] = ip[i] | ^mask[i]
			}
			out = append(out, bc)
		}
	}
	if len(out) == 0 {
		out = append(out, net.IPv4bcast)
	}
	return out
}

// ---------- addresses ----------

var virtualWords = []string{"vethernet", "wsl", "hyper-v", "virtualbox", "vmware", "vmnet", "docker", "loopback", "bluetooth",
	"tailscale", "zerotier", "hamachi", "nordlynx", "nord", "proton", "wireguard", "openvpn", "tap-", "wintun", "mullvad",
	"expressvpn", "surfshark", "pia", "cyberghost", "windscribe", "cisco", "anyconnect", "fortinet", "forticlient",
	"globalprotect", "pangp", "juniper", "pulse", "sonicwall", "radmin", "vpn", "utun", "veth", "br-", "virbr"}

// virtualAdapter: VPNs, virtual machines, WSL and Docker, which a tablet usually can't reach.
func virtualAdapter(name string) bool {
	n := strings.ToLower(name)
	for _, w := range virtualWords {
		if strings.Contains(n, w) {
			return true
		}
	}
	return false
}

// Address is one of this PC's network addresses.
type Address struct {
	Adapter string `json:"adapter"`
	IP      string `json:"ip"`
	Virtual bool   `json:"virtual"`
	Kind    string `json:"kind"` // lan, tailscale, other
}

// Addresses lists this PC's IPv4 addresses, the likeliest tablet network first.
func Addresses() []Address {
	var out []Address
	ifaces, _ := net.Interfaces()
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, ad := range addrs {
			ipn, ok := ad.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || ipn.IP.IsLinkLocalUnicast() {
				continue
			}
			ip := ipn.IP.String()
			kind := "other"
			switch {
			case strings.HasPrefix(ip, "100.") && (strings.Contains(strings.ToLower(ifc.Name), "tailscale") || isCGNAT(ipn.IP)):
				kind = "tailscale"
			case ipn.IP.IsPrivate():
				kind = "lan"
			}
			out = append(out, Address{Adapter: ifc.Name, IP: ip, Virtual: virtualAdapter(ifc.Name), Kind: kind})
		}
	}
	rank := func(a Address) int {
		r := 0
		if a.Virtual {
			r += 10
		}
		switch {
		case strings.HasPrefix(a.IP, "192.168."):
		case strings.HasPrefix(a.IP, "10."):
			r++
		case a.Kind == "lan":
			r += 2
		default:
			r += 3
		}
		return r
	}
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i]) < rank(out[j]) })
	return out
}

func isCGNAT(ip net.IP) bool {
	ip4 := ip.To4()
	return ip4 != nil && ip4[0] == 100 && ip4[1]&0xc0 == 64
}

// LanAddress is the address for the QR code: the host setting, else the best guess.
func (a *App) LanAddress() string {
	if h := a.Store.Config().Host; h != "" {
		return h
	}
	if addrs := Addresses(); len(addrs) > 0 {
		return addrs[0].IP
	}
	return ""
}
