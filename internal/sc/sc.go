// Package sc finds Star Citizen installs and reads the player's keyboard binds.
// Nothing here touches the running game: it reads actionmaps.xml (read only) and,
// when the player asks, copies the Tablet Deck profile into Controls\Mappings.
package sc

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Channels are the game's release channels, each its own folder under StarCitizen\.
var Channels = []string{"LIVE", "PTU", "EPTU", "TECH-PREVIEW", "HOTFIX"}

// Install is one channel folder, e.g. C:\Program Files\Roberts Space Industries\StarCitizen\LIVE.
type Install struct {
	Channel    string `json:"channel"`
	Dir        string `json:"dir"`
	Actionmaps string `json:"actionmaps,omitempty"` // empty until the game has saved binds
	Running    bool   `json:"running"`
}

// Mappings is where the game looks for keybind profiles to load in its Options.
func (i Install) Mappings() string {
	return filepath.Join(i.Dir, "user", "client", "0", "Controls", "Mappings")
}

func actionmapsIn(dir string) string {
	p := filepath.Join(dir, "user", "client", "0", "Profiles", "default", "actionmaps.xml")
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return p
	}
	return ""
}

// ChannelDirFromExe turns ...\StarCitizen\PTU\Bin64\StarCitizen.exe into ...\StarCitizen\PTU.
func ChannelDirFromExe(exe string) string {
	if exe == "" {
		return ""
	}
	bin := filepath.Dir(exe)
	if !strings.EqualFold(filepath.Base(bin), "Bin64") {
		return ""
	}
	return filepath.Dir(bin)
}

// Discover lists every channel it can find. runningExe is the full path of a running
// StarCitizen.exe ("" when the game isn't running); it finds installs in any folder,
// so the drive scan below is only the fallback for when the game is closed. extra are
// folders the player set by hand (a StarCitizen folder or a channel folder).
func Discover(runningExe string, extra []string) []Install {
	seen := map[string]bool{}
	var out []Install
	add := func(dir string, running bool) {
		dir = filepath.Clean(dir)
		key := strings.ToLower(dir)
		if seen[key] {
			if running {
				for i := range out {
					if strings.EqualFold(out[i].Dir, dir) {
						out[i].Running = true
					}
				}
			}
			return
		}
		fi, err := os.Stat(dir)
		if err != nil || !fi.IsDir() {
			return
		}
		seen[key] = true
		out = append(out, Install{Channel: strings.ToUpper(filepath.Base(dir)), Dir: dir, Actionmaps: actionmapsIn(dir), Running: running})
	}
	addRoot := func(root string) { // a StarCitizen folder: try every channel under it
		for _, ch := range Channels {
			add(filepath.Join(root, ch), false)
		}
	}
	if d := ChannelDirFromExe(runningExe); d != "" {
		add(d, true)
		addRoot(filepath.Dir(d))
	}
	for _, e := range extra {
		if e == "" {
			continue
		}
		if isChannel(filepath.Base(e)) {
			add(e, false)
		} else {
			addRoot(e)
			addRoot(filepath.Join(e, "StarCitizen"))
		}
	}
	for _, root := range candidateRoots() {
		addRoot(root)
	}
	// Running first, then LIVE, then the channel order above.
	rank := func(i Install) int {
		if i.Running {
			return -1
		}
		for n, ch := range Channels {
			if ch == i.Channel {
				return n
			}
		}
		return len(Channels)
	}
	sort.SliceStable(out, func(a, b int) bool { return rank(out[a]) < rank(out[b]) })
	return out
}

func isChannel(name string) bool {
	for _, ch := range Channels {
		if strings.EqualFold(ch, name) {
			return true
		}
	}
	return false
}

// candidateRoots are the usual StarCitizen folders on every drive letter.
func candidateRoots() []string {
	if os.PathSeparator != '\\' {
		return nil
	}
	var roots []string
	for d := 'C'; d <= 'Z'; d++ {
		drive := string(d) + `:\`
		if _, err := os.Stat(drive); err != nil {
			continue
		}
		for _, sub := range []string{
			`Program Files\Roberts Space Industries\StarCitizen`,
			`Roberts Space Industries\StarCitizen`,
			`RSI\StarCitizen`,
			`Games\Roberts Space Industries\StarCitizen`,
			`Games\StarCitizen`,
			`StarCitizen`,
		} {
			roots = append(roots, drive+sub)
		}
	}
	return roots
}

// ReadBinds returns action name -> keyboard combo ("lalt+n") for every keyboard rebind
// in an actionmaps.xml or a Controls\Mappings profile. Later files win, like a profile
// loaded in-game on top of earlier changes.
func ReadBinds(files ...string) map[string]string {
	out := map[string]string{}
	for _, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			continue
		}
		parseBinds(fh, out)
		fh.Close()
	}
	return out
}

// ParseBinds is ReadBinds for XML already in hand.
func ParseBinds(r io.Reader) map[string]string {
	out := map[string]string{}
	parseBinds(r, out)
	return out
}

func parseBinds(r io.Reader, out map[string]string) {
	dec := xml.NewDecoder(r)
	dec.Strict = false
	var action string
	for {
		tok, err := dec.Token()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "action":
				action = attr(t, "name")
			case "rebind":
				in := attr(t, "input")
				if action == "" || !strings.HasPrefix(in, "kb") {
					continue
				}
				// "kb1_lalt+n"; an unbound key is written "kb1_ " and is skipped, so the
				// layout's own key stays.
				if i := strings.IndexByte(in, '_'); i > -1 {
					if combo := strings.TrimSpace(in[i+1:]); combo != "" {
						out[action] = combo
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "action" {
				action = ""
			}
		}
	}
}

func attr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// InstallProfile copies the Tablet Deck keybind profile into each install's
// Controls\Mappings folder, where the game's Options > Keybindings > Control Profiles
// lists it. Returns the files written.
func InstallProfile(profile []byte, name string, installs []Install) ([]string, error) {
	var written []string
	for _, in := range installs {
		dir := in.Mappings()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return written, err
		}
		dst := filepath.Join(dir, name)
		if err := os.WriteFile(dst, profile, 0o644); err != nil {
			return written, err
		}
		written = append(written, dst)
	}
	return written, nil
}
