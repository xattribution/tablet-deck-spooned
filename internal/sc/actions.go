package sc

// Every action the game has, read from the installed game itself: the default
// keyboard profile (Data\Libs\Config\defaultProfile.xml) and the English names
// (Data\Localization\english\global.ini), both inside Data.p4k. Data.p4k is a zip64
// archive with zstd entries; only those two entries are read, never the rest of the
// 150 GB. The result is cached by the archive's size and time, so the 450 MB central
// directory is read once per game patch. A copy from game 4.10 is built in for when
// no install is reachable.

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// Action is one game action.
type Action struct {
	Map      string `json:"map"`      // action map, e.g. spaceship_weapons
	MapLabel string `json:"mapLabel"` // its name in the game's menu
	Name     string `json:"name"`     // e.g. v_toggle_landing_system
	Label    string `json:"label"`    // English name in the keybind menu
	Desc     string `json:"desc,omitempty"`
	Key      string `json:"kb,omitempty"`   // default keyboard bind, e.g. "lalt+n"
	Mode     string `json:"mode,omitempty"` // activation mode: tap, hold, double_tap...
}

//go:embed actions-4.10.json
var builtinActions []byte

// BuiltinActions is the action list from game 4.10, for when no install is found.
func BuiltinActions() []Action {
	var out []Action
	_ = json.Unmarshal(builtinActions, &out)
	return out
}

type actionCache struct {
	Stamp   string   `json:"stamp"`
	Version string   `json:"version"`
	Actions []Action `json:"actions"`
}

// LoadActions reads the actions of the install in dir (a channel folder such as
// ...\StarCitizen\LIVE), using cacheFile when the archive hasn't changed.
func LoadActions(dir, cacheFile string) ([]Action, string, error) {
	p4k := filepath.Join(dir, "Data.p4k")
	fi, err := os.Stat(p4k)
	if err != nil {
		return nil, "", err
	}
	stamp := fmt.Sprintf("%d|%d", fi.Size(), fi.ModTime().UnixNano())
	version := readVersion(dir)
	if cacheFile != "" {
		if b, err := os.ReadFile(cacheFile); err == nil {
			var c actionCache
			if json.Unmarshal(b, &c) == nil && c.Stamp == stamp && len(c.Actions) > 0 {
				return c.Actions, c.Version, nil
			}
		}
	}
	files, err := ReadP4K(p4k, "data/libs/config/defaultprofile.xml", "data/localization/english/global.ini")
	if err != nil {
		return nil, "", err
	}
	prof, ok := files["data/libs/config/defaultprofile.xml"]
	if !ok {
		return nil, "", errors.New("defaultProfile.xml isn't in Data.p4k")
	}
	acts, err := ParseProfile(prof, ParseLocalization(files["data/localization/english/global.ini"]))
	if err != nil {
		return nil, "", err
	}
	// A patch that moved the labels gives a short list; the built-in one is better then.
	if len(acts) < len(BuiltinActions())/2 {
		return nil, "", fmt.Errorf("only %d labeled actions found (the game's files changed?)", len(acts))
	}
	if cacheFile != "" {
		if b, err := json.Marshal(actionCache{Stamp: stamp, Version: version, Actions: acts}); err == nil {
			_ = os.WriteFile(cacheFile, b, 0o600)
		}
	}
	return acts, version, nil
}

func readVersion(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "build_manifest.id"))
	if err != nil {
		return ""
	}
	var m struct {
		Data struct{ Branch, Version string }
	}
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	return strings.TrimSpace(m.Data.Branch + " " + m.Data.Version)
}

// ---------- Data.p4k ----------

// ReadP4K returns the named entries (lower case, forward slashes) from a p4k archive.
func ReadP4K(path string, names ...string) (map[string][]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	tailLen := int64(1 << 16)
	if size < tailLen {
		tailLen = size
	}
	tail := make([]byte, tailLen)
	if _, err := f.ReadAt(tail, size-tailLen); err != nil {
		return nil, err
	}
	i := bytes.LastIndex(tail, []byte("PK\x06\x06"))
	if i < 0 || i+56 > len(tail) {
		return nil, errors.New("not a zip64 archive")
	}
	cdSize := int64(binary.LittleEndian.Uint64(tail[i+40:]))
	cdOff := int64(binary.LittleEndian.Uint64(tail[i+48:]))
	want := map[string]bool{}
	for _, n := range names {
		want[strings.ToLower(n)] = true
	}
	type entry struct {
		method     uint16
		csize, lho int64
	}
	found := map[string]entry{}
	r := bufio.NewReaderSize(io.NewSectionReader(f, cdOff, cdSize), 1<<20)
	hdr := make([]byte, 46)
	for len(found) < len(want) {
		if _, err := io.ReadFull(r, hdr); err != nil {
			break
		}
		if !bytes.Equal(hdr[:4], []byte("PK\x01\x02")) {
			break
		}
		method := binary.LittleEndian.Uint16(hdr[10:])
		csize := int64(binary.LittleEndian.Uint32(hdr[20:]))
		usize := int64(binary.LittleEndian.Uint32(hdr[24:]))
		nl, el, cl := int(binary.LittleEndian.Uint16(hdr[28:])), int(binary.LittleEndian.Uint16(hdr[30:])), int(binary.LittleEndian.Uint16(hdr[32:]))
		lho := int64(binary.LittleEndian.Uint32(hdr[42:]))
		rest := make([]byte, nl+el+cl)
		if _, err := io.ReadFull(r, rest); err != nil {
			break
		}
		name := strings.ToLower(strings.ReplaceAll(string(rest[:nl]), "\\", "/"))
		if !want[name] {
			continue
		}
		// Zip64 extra field: the 32-bit fields set to 0xFFFFFFFF are stored here.
		extra := rest[nl : nl+el]
		for q := 0; q+4 <= len(extra); {
			id, l := binary.LittleEndian.Uint16(extra[q:]), int(binary.LittleEndian.Uint16(extra[q+2:]))
			body := extra[q+4 : min(len(extra), q+4+l)]
			if id == 1 {
				k := 0
				next := func() int64 {
					if k+8 > len(body) {
						return 0
					}
					v := int64(binary.LittleEndian.Uint64(body[k:]))
					k += 8
					return v
				}
				if usize == 0xFFFFFFFF {
					next()
				}
				if csize == 0xFFFFFFFF {
					csize = next()
				}
				if lho == 0xFFFFFFFF {
					lho = next()
				}
			}
			q += 4 + l
		}
		found[name] = entry{method, csize, lho}
	}
	out := map[string][]byte{}
	var dec *zstd.Decoder
	for name, e := range found {
		lh := make([]byte, 30)
		if _, err := f.ReadAt(lh, e.lho); err != nil {
			return nil, err
		}
		start := e.lho + 30 + int64(binary.LittleEndian.Uint16(lh[26:])) + int64(binary.LittleEndian.Uint16(lh[28:]))
		if e.csize < 0 || e.csize > 256<<20 || e.lho < 0 { // these files are a few hundred KB
			return nil, fmt.Errorf("%s: unexpected size %d in Data.p4k", name, e.csize)
		}
		data := make([]byte, e.csize)
		if _, err := f.ReadAt(data, start); err != nil {
			return nil, err
		}
		switch e.method {
		case 0:
			out[name] = data
		case 100: // zstd
			if dec == nil {
				if dec, err = zstd.NewReader(nil); err != nil {
					return nil, err
				}
				defer dec.Close()
			}
			b, err := dec.DecodeAll(data, nil)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			out[name] = b
		default:
			return nil, fmt.Errorf("%s: compression method %d isn't supported", name, e.method)
		}
	}
	return out, nil
}

// ---------- the profile ----------

// ParseLocalization reads global.ini into key (lower case) -> text.
func ParseLocalization(b []byte) map[string]string {
	out := map[string]string{}
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(strings.TrimRight(line, "\r"), "=")
		if !ok || k == "" {
			continue
		}
		k, _, _ = strings.Cut(k, ",")
		out[strings.ToLower(k)] = v
	}
	return out
}

// Maps that are menus, debug or cameras: nothing a player presses from a deck.
var skipMaps = map[string]bool{"debug": true, "flycam": true, "spectator": true, "server_renderer": true, "view_director_mode": true,
	"RemoteRigidEntityController": true, "character_customizer": true, "ui_textfield": true, "default": true, "IFCS_controls": true}

// ParseProfile turns defaultProfile.xml (binary CryXmlB or text) into actions with
// English names.
func ParseProfile(b []byte, loc map[string]string) ([]Action, error) {
	if bytes.HasPrefix(b, []byte("CryXmlB")) {
		t, err := CryXMLToText(b)
		if err != nil {
			return nil, err
		}
		b = t
	}
	name := func(s string) string {
		if strings.HasPrefix(s, "@") {
			if v, ok := loc[strings.ToLower(s[1:])]; ok {
				return strings.TrimSpace(strings.ReplaceAll(v, `\n`, " "))
			}
			return ""
		}
		return strings.TrimSpace(s)
	}
	d := xml.NewDecoder(bytes.NewReader(b))
	var out []Action
	var cur, curLabel string
	var act *Action
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch e := tok.(type) {
		case xml.StartElement:
			switch e.Name.Local {
			case "actionmap":
				cur, curLabel = attr(e, "name"), name(attr(e, "UILabel"))
			case "action":
				if cur == "" {
					continue
				}
				act = &Action{Map: cur, MapLabel: curLabel, Name: attr(e, "name"), Label: name(attr(e, "UILabel")),
					Desc: name(attr(e, "UIDescription")), Key: strings.TrimSpace(attr(e, "keyboard")), Mode: attr(e, "activationMode")}
			case "keyboard":
				if act != nil && act.Key == "" {
					act.Key = strings.TrimSpace(attr(e, "input"))
				}
			case "inputdata":
				if act != nil && act.Key == "" && attr(e, "input") != "" {
					act.Key = strings.TrimSpace(attr(e, "input"))
				}
			}
		case xml.EndElement:
			switch e.Name.Local {
			case "action":
				if act != nil && act.Label != "" && !skipMaps[act.Map] {
					out = append(out, *act)
				}
				act = nil
			case "actionmap":
				cur = ""
			}
		}
	}
	return out, nil
}

// CryXMLToText converts CryEngine's binary XML to ordinary XML text.
func CryXMLToText(b []byte) ([]byte, error) {
	if len(b) < 44 || !bytes.HasPrefix(b, []byte("CryXmlB")) {
		return nil, errors.New("not CryXmlB")
	}
	u := func(o int) int { return int(binary.LittleEndian.Uint32(b[o:])) }
	nodeOff, nodeN := u(12), u(16)
	attrOff := u(20)
	childOff := u(28)
	strOff := u(36)
	str := func(o int) string {
		p := strOff + o
		if p < 0 || p >= len(b) {
			return ""
		}
		e := bytes.IndexByte(b[p:], 0)
		if e < 0 {
			return string(b[p:])
		}
		return string(b[p : p+e])
	}
	if nodeOff+nodeN*28 > len(b) {
		return nil, errors.New("CryXmlB is truncated")
	}
	type node struct {
		name     string
		attrs    [][2]string
		children []int
	}
	nodes := make([]node, nodeN)
	for i := 0; i < nodeN; i++ {
		o := nodeOff + i*28
		n := node{name: str(u(o))}
		ac, cc := int(binary.LittleEndian.Uint16(b[o+8:])), int(binary.LittleEndian.Uint16(b[o+10:]))
		fa, fc := int(int32(binary.LittleEndian.Uint32(b[o+16:]))), int(int32(binary.LittleEndian.Uint32(b[o+20:])))
		for k := 0; k < ac; k++ {
			p := attrOff + (fa+k)*8
			if p+8 > len(b) {
				break
			}
			n.attrs = append(n.attrs, [2]string{str(u(p)), str(u(p + 4))})
		}
		for k := 0; k < cc; k++ {
			p := childOff + (fc+k)*4
			if p+4 > len(b) {
				break
			}
			n.children = append(n.children, int(int32(binary.LittleEndian.Uint32(b[p:]))))
		}
		nodes[i] = n
	}
	var w bytes.Buffer
	var write func(i, depth int)
	write = func(i, depth int) {
		if i < 0 || i >= len(nodes) || depth > 64 {
			return
		}
		n := nodes[i]
		w.WriteString("<" + n.name)
		for _, a := range n.attrs {
			w.WriteString(" " + a[0] + `="`)
			_ = xml.EscapeText(&w, []byte(a[1]))
			w.WriteString(`"`)
		}
		if len(n.children) == 0 {
			w.WriteString("/>")
			return
		}
		w.WriteString(">")
		for _, c := range n.children {
			write(c, depth+1)
		}
		w.WriteString("</" + n.name + ">")
	}
	write(0, 0)
	return w.Bytes(), nil
}
