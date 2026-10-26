package app

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Battle mode reads the game's own log (Game.log, which the game writes for support
// and which tools may read; nothing in the game is touched). What it can tell:
//   - your ship is being hit, and by whom;
//   - your ship was disabled or destroyed;
//   - you died, and who you killed.
// It can't tell shield or hull levels, component damage or oxygen: the game doesn't
// write those anywhere a program may read.

var (
	reNick     = regexp.MustCompile(`nickname="([^"]+)"`)
	reDriver   = regexp.MustCompile(`<Vehicle Control Flow>.*Local client node \[\d+\] (requesting|granted|releasing) control token for '([^']+)'`)
	reHit      = regexp.MustCompile(`<Debug Hostility Events> \[OnHandleHit\] (Fake hit|Hit) FROM (\S+) TO (\S+)\. Being sent to child (\S+)`)
	reDestroy  = regexp.MustCompile(`<Vehicle Destruction> .*Vehicle '([^']+)' .*advanced from destroy level (\d) to (\d) caused by '([^']*)'`)
	reDeath    = regexp.MustCompile(`<Actor Death> CActor::Kill: '([^']+)' .*killed by '([^']*)'.*damage type '([^']*)'`)
	reEntityNo = regexp.MustCompile(`_\d{6,}$`)
)

// battleState is what the decks show in battle mode.
type battleState struct {
	Ship    string    `json:"ship"`  // the ship you fly ("ORIG m80")
	Fire    bool      `json:"fire"`  // hit in the last few seconds
	From    string    `json:"from"`  // who hit you last
	Hits    int       `json:"hits"`  // hits this fight
	Level   int       `json:"level"` // your ship: 0 fine, 1 disabled, 2 destroyed
	Dead    bool      `json:"dead"`  // you died
	Kills   int       `json:"kills"` // this session
	LastHit time.Time `json:"-"`
}

type battleRT struct {
	mu    sync.Mutex
	nick  string
	ship  string // entity name, with its id
	st    battleState
	timer *time.Timer
}

// short turns "PU_Pilots-Human-Criminal-Pilot_Light_6929783507558" into something a
// person reads: "Criminal pilot", and "ORIG_m80_200000777639" into "ORIG m80".
func shortEntity(entity string) string {
	e := reEntityNo.ReplaceAllString(entity, "")
	low := strings.ToLower(e)
	switch {
	case strings.Contains(low, "pirate"):
		return "Pirate"
	case strings.Contains(low, "criminal"):
		return "Criminal pilot"
	case strings.Contains(low, "security") || strings.Contains(low, "advocacy"):
		return "Security"
	case strings.HasPrefix(low, "pu_"):
		return "NPC"
	}
	return strings.ReplaceAll(e, "_", " ")
}

// watchGameLog follows Game.log of the running (or LIVE) install while the app runs.
func (a *App) watchGameLog(stop <-chan struct{}) {
	var path string
	var off int64
	var f *os.File
	defer func() {
		if f != nil {
			f.Close()
		}
	}()
	t := time.NewTicker(250 * time.Millisecond)
	defer t.Stop()
	find := time.Time{}
	for {
		select {
		case <-stop:
			return
		case <-t.C:
		}
		if f == nil && (path == "" || time.Since(find) > 30*time.Second) {
			find = time.Now()
			for _, in := range a.Binds().Installs {
				p := filepath.Join(in.Dir, "Game.log")
				if _, err := os.Stat(p); err == nil && (path == "" || in.Running) {
					path = p
					if in.Running {
						break
					}
				}
			}
			if path == "" {
				continue
			}
		}
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		if f == nil || fi.Size() < off { // a new game session starts a new log
			if f != nil {
				f.Close()
			}
			if f, err = os.Open(path); err != nil {
				f = nil
				continue
			}
			off = 0
			a.battleReset()
			if fi.Size() > 4<<20 { // joining a long session: read its start for the name, then the end
				a.gameLogLines(io.LimitReader(f, 1<<20))
				off = fi.Size() - 256<<10
			}
		}
		if fi.Size() == off {
			continue
		}
		if _, err := f.Seek(off, io.SeekStart); err != nil {
			continue
		}
		n := a.gameLogLines(io.LimitReader(f, fi.Size()-off))
		off += n
	}
}

// gameLogLines reads whole lines and returns how many bytes it used.
func (a *App) gameLogLines(r io.Reader) int64 {
	br := bufio.NewReaderSize(r, 64<<10)
	var used int64
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return used // a partial line waits for the rest
		}
		used += int64(len(line))
		a.gameLogLine(line)
	}
}

func (a *App) gameLogLine(line string) {
	b := &a.battle
	switch {
	case strings.Contains(line, "nickname="):
		if m := reNick.FindStringSubmatch(line); m != nil {
			b.mu.Lock()
			b.nick = m[1]
			b.mu.Unlock()
		}
	case strings.Contains(line, "<Vehicle Control Flow>"):
		if m := reDriver.FindStringSubmatch(line); m != nil {
			b.mu.Lock()
			if m[1] == "releasing" {
				if b.ship == m[2] {
					b.ship, b.st.Ship = "", ""
				}
			} else if b.ship != m[2] {
				b.ship, b.st.Ship = m[2], shortEntity(m[2])
				b.st.Level, b.st.Dead = 0, false
			}
			b.mu.Unlock()
			a.pushBattle("ship")
		}
	case strings.Contains(line, "<Debug Hostility Events>"):
		m := reHit.FindStringSubmatch(line)
		if m == nil {
			return
		}
		b.mu.Lock()
		mine := b.nick != "" && m[4] == b.nick
		if mine {
			if b.ship == "" || m[3] != b.ship { // a hit on a ship we're in tells us which ship
				b.ship, b.st.Ship = m[3], shortEntity(m[3])
			}
			if !b.st.Fire {
				b.st.Hits = 0
			}
			b.st.Fire, b.st.From, b.st.LastHit = true, shortEntity(m[2]), time.Now()
			b.st.Hits++
			if b.timer != nil {
				b.timer.Stop()
			}
			b.timer = time.AfterFunc(8*time.Second, func() { // quiet for a while: the fight is over
				b.mu.Lock()
				over := time.Since(b.st.LastHit) >= 8*time.Second
				if over {
					b.st.Fire = false
				}
				b.mu.Unlock()
				if over {
					a.pushBattle("clear")
				}
			})
		}
		b.mu.Unlock()
		if mine {
			a.pushBattle("hit")
		}
	case strings.Contains(line, "<Vehicle Destruction>"):
		m := reDestroy.FindStringSubmatch(line)
		if m == nil {
			return
		}
		b.mu.Lock()
		mine := b.ship != "" && m[1] == b.ship
		if mine {
			b.st.Level = int(m[3][0] - '0')
		}
		b.mu.Unlock()
		if mine {
			a.pushBattle("ship-" + m[3])
		}
	case strings.Contains(line, "<Actor Death>"):
		m := reDeath.FindStringSubmatch(line)
		if m == nil {
			return
		}
		b.mu.Lock()
		nick := b.nick
		kind := ""
		if nick != "" && m[1] == nick {
			b.st.Dead, b.st.Fire, kind = true, false, "dead"
		} else if nick != "" && m[2] == nick {
			b.st.Kills++
			kind = "kill"
		}
		b.mu.Unlock()
		if kind != "" {
			a.pushBattle(kind)
		}
	}
}

func (a *App) battleReset() {
	a.battle.mu.Lock()
	a.battle.ship = ""
	a.battle.st = battleState{}
	a.battle.mu.Unlock()
	a.pushBattle("reset")
}

func (a *App) battleNow() battleState {
	a.battle.mu.Lock()
	defer a.battle.mu.Unlock()
	return a.battle.st
}

// pushBattle tells the decks what happened (hit, ship-1, ship-2, dead, kill, clear).
func (a *App) pushBattle(event string) {
	st := a.battleNow()
	a.broadcastOwners(map[string]any{"t": "battle", "event": event, "state": st})
	if event != "hit" || st.Hits == 1 {
		a.Bus.Publish("battle", map[string]any{"event": event, "from": st.From, "ship": st.Ship})
	}
}
