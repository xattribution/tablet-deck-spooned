# Agent guidelines

Instructions for AI coding agents (and a quick start for human contributors)
working on Tablet Deck. Read this first, then
[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for the details it points to.

## What the project is

Tablet Deck turns a tablet or phone into a cockpit control deck for Star Citizen.
The deck runs in the tablet's browser; every press travels over the home network to
a small Go server on the gaming PC, which presses the real keys with Windows'
`SendInput`. It also has a voice engine, a Comms panel with an AI agent, crew share
links, and an MCP server so agents can drive the deck and build panels. It's a fan
project, not affiliated with Cloud Imperium Games.

| path | what it is |
|---|---|
| `cmd/tabletdeck` | the program: `serve`, `mcp` (stdio), `press`, `game-actions` |
| `internal/app` | the server: deck WebSocket, API, MCP tools, catalog, panels, agent, voice, crew |
| `internal/deck` | the press engine: focus checks, holds, releases |
| `internal/panels` | the panel format ([docs/PANELS.md](docs/PANELS.md)) |
| `internal/sc` | Star Citizen files: Data.p4k reader, keybind profiles, every game action |
| `internal/input`, `internal/keys` | key sending and combo parsing |
| `internal/config` | config.json and state.json |
| `panels/` | built-in panels (embedded) |
| `public/` | the deck (`index.html`, `deck3.*`, `panels.js`), the PC control panel, the editor |
| `layouts/`, `keybinds/` | the classic layouts and the keybind profile |
| `native/`, `installer/`, `integrations/` | the Windows app window, installer, Stream Deck plugin |
| `site/` | the website on GitHub Pages |

## Ground rules

These keep Tablet Deck safe to run next to an online game:

- **The game is never touched.** No reading or writing game memory, no injection,
  no hooks. The only game files used are read-only (keybinds, Data.p4k for the action
  list) and the profiles copied into `Controls\Mappings` when the player asks.
- **Keys reach Star Citizen by default.** The focus check stays on. A control marked
  `global` (a panel action, opted in per action) goes to whatever window is in front:
  this is how Discord, OBS and other apps are driven. Nothing else skips the check.
- **Releases always go through.** A key that went down comes up, after focus loss or
  a disconnect, so no key stays stuck. Global holds are exempt from focus-loss release
  only.
- **One press per request.** The engine presses a control once per tap or hold. The
  agent and MCP press through the same engine, one press per tool call; there are no
  macros or timers that press keys.
- **The agent never presses self-destruct or eject** unless the player asks for it in
  that message. Keep this in its system prompt.
- **Pairing is required.** The WebSocket and every data route check the pairing code
  or a device token; settings, bind-all and agent routes only answer the PC itself.
  New routes follow the same rules.
- **No tracking.** No accounts or analytics. Outgoing traffic is the update check,
  the agent API the player configures, and the optional tunnel.

## Code conventions

- **Match the surrounding code:** naming, comment density, idiom. Comments explain
  why. `gofmt` and `go vet` clean.
- **Few dependencies.** Standard library first; current ones are `coder/websocket`
  and `klauspost/compress`. Vet any new one before adding it.
- **Windows is the target** for what runs on the PC; everything else must build and
  test on Linux. The deck must work in Safari (iPad, iPhone) and Chrome (Android).
- **The deck has no build step.** Plain JS and CSS in `public/`, embedded at build
  time, so rebuild the binary after changing them.
- **Panels are data.** A new built-in panel is a JSON file in `panels/`, not code. A
  new widget needs: the name in `panels.Widgets` (and `ControlRefs` if it places
  controls), a renderer in `public/panels.js`, CSS, and a row in docs/PANELS.md.
- **Design:** follow [docs/DESIGN.md](docs/DESIGN.md): theme tokens, category
  colours, accent colours as variables, no scrolling on the deck.

## Checking your work

1. `go vet ./... && go test ./...`
2. `node --check` every JavaScript file you changed.
3. Run it: `go build -o td ./cmd/tabletdeck && ./td serve --dry-run --data /tmp/td`,
   then open the printed address with `?t=<pairing code>`. `--dry-run` presses no
   keys, only logs them.
4. Look at the deck in a browser (Playwright is fine) at tablet and phone sizes.

Say what you couldn't check. Windows-only parts (key sending, voice, the app window,
the installer) can't run elsewhere; never report them as tested if they weren't.

## Docs and releases

- **Every user-facing change** gets a line in the top section of `CHANGELOG.md`, and
  the README is updated in the same change.
- **The panel format** is documented in docs/PANELS.md, which agents read through
  `panel_guide`. Keep it in step with `internal/panels` and `public/panels.js`.
- Don't tag or release without the owner.

## Git

- Short, imperative commit subjects, with a body that explains what and why.
- Never force-push, rewrite history or delete branches or tags without the owner's OK.

## Privacy

Nothing from a real machine goes into commits, docs, screenshots or the build: no
real IP addresses, pairing codes, API keys, user names or local paths. Use
`http://192.168.1.20:8765` and pairing code `1a2b3c4d` in examples. `config.json`
(pairing code, ports, game paths, the agent's API key) is never committed.
