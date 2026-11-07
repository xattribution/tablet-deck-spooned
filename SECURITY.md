# Security policy

SpoolUp presses keys on your PC and runs a small server on your home
network, so security problems matter. Thanks for helping keep it safe.

## Reporting a problem

Please **don't open a public issue** for a security problem. Report it
privately instead:

- **[Report a vulnerability](https://github.com/xattribution/SpoolUp/security/advisories/new)**
  on GitHub (the Security tab → "Report a vulnerability").

Include the SpoolUp version (shown in the window's title bar), what someone
could do, and the steps to make it happen. Leave out your own pairing code and
IP addresses. You'll get a reply as soon as possible. Once it's fixed, the fix
ships in a new release, and you're credited in the advisory unless you'd rather
not be.

## Supported versions

Only the latest release gets security fixes. SpoolUp checks for updates
itself and installs them when you click **Update now**.

## What counts as a security problem

SpoolUp is meant to be safe on your own home network. These are in scope:

- Anything that lets someone **without the pairing code** press keys, change
  settings, open links or play sounds on the PC, or read the pairing code.
- Keys reaching **another window than Star Citizen** while the "only while Star
  Citizen is in front" check is on.
- The layout editor's features working from **another device** (they're meant
  for the PC itself only).
- SpoolUp being reachable or doing anything on networks **Windows marks as
  public**.
- The updater installing anything that **isn't the release's installer**, or
  installing it when its checksum doesn't match.
- SpoolUp reading or changing the **game's memory or program files**, or
  any data beyond what the README describes.

## Known limits (not vulnerabilities)

These are by design and documented in the README's
[Pairing and network](README.md#pairing-and-network) section:

- The tablet talks to the PC over plain `http` on your home network, so others
  on the **same Wi-Fi** could see the pairing code and taps. Don't use SpoolUp
  on shared networks (LAN parties, dorms, hotels).
- Anyone with your **pairing code** can press keys, open the websites set up
  on the PC and play the deck's sounds on its outputs. Treat it like a password,
  and change it in **Settings** if it leaks. Wrong codes are limited (5 a
  minute per device, then that device waits a minute), which slows guessing
  down but doesn't make a short code safe on a shared network.
- Someone who already has **access to your PC** can change SpoolUp's
  settings or files.
- Releases are **code-signed** from 1.8.0 (earlier ones aren't, so Windows
  shows an "unknown publisher" warning for them). Each release lists the
  installer's SHA-256 checksum, and releases are built by GitHub Actions from
  this repository (see [CODE_SIGNING.md](CODE_SIGNING.md)).
