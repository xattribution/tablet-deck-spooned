# Publishing on the Elgato Marketplace and in winget / the Microsoft Store

Checked against the vendors' own pages on 2026-11-06. Legal, trademark and Cloud Imperium questions are left out, since you're reviewing those yourself.

## TL;DR

- **Elgato Marketplace:** SpoolUp for Stream Deck (the plugin, icon pack and profiles) is a paid product built and published from the private repository `xattribution/SpoolUp-StreamDeck`, which holds the Marketplace steps and listings. Elgato's price-match rule is why this repository no longer ships Stream Deck downloads (from 2.1).
- **winget / Microsoft Store:** these are two separate routes:
  - a manifest pull request to `microsoft/winget-pkgs`;
  - a Microsoft Store "EXE or MSI app" listing, which winget can also install from its `msstore` source.
  
  **Recommendation:** use **winget-pkgs now**, starting with **1.8.0**, the first signed release, and update it automatically from `release.yml`. Hold off on the Store until Partner Center support answers two questions: whether it accepts Artifact Signing, and whether the app counts as a "game product".
- **Before either one:** publish the v1.8.0 release. GitHub has no v1.8.0 release yet, and every existing 1.7.0 build is unsigned.

---

## 1. Elgato Marketplace

SpoolUp for Stream Deck is sold on the [Elgato Marketplace](https://marketplace.elgato.com) and built in the private repository `xattribution/SpoolUp-StreamDeck`. Its `docs/PUBLISHING.md` section 1 has the Marketplace rules and steps, and `docs/publishing/marketplace-listings.md` the listing text and the checklist before uploading.

What matters here:

- **Price match.** Elgato's rule means the Stream Deck products can't be free elsewhere, so SpoolUp's releases and website carry no Stream Deck downloads from 2.1. Don't add any back.
- **The server's side** of the plugin connection stays in this repository: [STREAMDECK-API.md](STREAMDECK-API.md). A change there, or to the button and panel ids in `layouts/hud.json`, the themes or the icons, needs the Stream Deck repository updated too.
- **Links.** Once the product page is live, replace the placeholder `https://marketplace.elgato.com` links in the README's "Stream Deck" section and in `site/index.html` (each has a comment marking it).

---

## 2. winget and the Microsoft Store

### The two routes

| | winget community repo (`microsoft/winget-pkgs`) | Microsoft Store "EXE or MSI app" |
|---|---|---|
| Install command | `winget install xattribution.SpoolUp` | `winget install <XP…StoreId> -s msstore`, or the Store app |
| What you submit | A pull request with 3 YAML files per version | A Partner Center listing, with a versioned HTTPS installer URL |
| Signing | Not required | Required: the certificate must chain to a CA in the Microsoft Trusted Root Program (policy 10.2.9) |
| Review | 10 automated checks plus a moderator. New packages waited **3–61 days** (two PRs merged 2026-11-06) | Up to **3 business days** of review (certification) |
| Updates | Automatic PR from `release.yml` via wingetcreate | A new submission per release (by hand, or through the API). The Store never updates existing users; the app's own updater does |
| GitHub release URL | Accepted | Likely rejected: it redirects (302). Q&A threads from 2023–24 show rejections, but no Learn page states the rule |
| Account | GitHub account and Microsoft CLA | Free Partner Center account at storedeveloper.microsoft.com (ID plus selfie) |

**Recommendation:** go with winget-pkgs. It has no signing or policy blockers and can be automated.

Do the Store later, and only once Partner Center support has answered both of these:
- **Artifact Signing.** Microsoft's Artifact Signing page (2026-10-01) says its CAs are "no longer" in the Trusted Root Program. The CCADB list still shows the root as Included, so whether the Store accepts these signatures is unknown. The fallback is an OV certificate.
- **Game product.** Policy 10.1 says products that mainly offer "control functionality of games" must be listed as games, and games can't use the EXE route. There is a precedent: Elgato Stream Deck is listed under Productivity as an MSI.

### What the project must change or settle first

1. **Publish v1.8.0, and skip 1.7.0.**
   - Both 1.7.0 installers are unsigned.
   - Two *different* files are named `TabletDeck-Setup-1.7.0.exe`:
     - nodieCode/tablet-deck, SHA-256 `C6419E21…8940`;
     - xattribution/SpoolUp, SHA-256 `80081EEB…7891`.
   - The 1.7.0 build's update check points at `nodieCode/tablet-deck`. Today's tree (launcher.cs, the .iss `RepoUrl`, the site) points at `xattribution/SpoolUp`.
   - Listing 1.7.0 from nodieCode while 1.8.0 lives in xattribution would mix two publishers and two repos under one package.
2. **Pick the final PackageIdentifier now.** For example `xattribution.SpoolUp`, matching `AppName` "SpoolUp" and `AppPublisher` "xattribution". Changing it after the first merge means the two-identifier migration described in the winget-pkgs FAQ.
3. **Silent install copies the keybind profile.**
   - The `scprofile` task (`installer/TabletDeck.iss` line 64) has no `unchecked` flag. So every silent install, winget and Store included, copies the profile into the game's Mappings folder without the player asking. That breaks the ground rule that the profile is copied only when the player asks.
   - The manifests below add `/MERGETASKS=!scprofile` to fix it.
   - The cleaner long-term fix is in the installer: skip the copy on silent first installs.
4. **Silent upgrade over a running app.**
   - winget's silent mode lets Inno close the app through Restart Manager.
   - `CloseRunningApp()`, which releases held keys, only runs with `/RELAUNCH=1`.
   - Test on Windows that held keys still come up after a winget upgrade, or call `CloseRunningApp()` for every silent install.
5. **Privacy policy URL.**
   - winget: optional. The drafts point at the README's "Pairing and network" section.
   - Store: **mandatory** for Win32 apps (policy 10.5.1). Adapt the Privacy section of `CODE_SIGNING.md`.
6. **Store only:** host a byte-identical copy of each installer at a versioned URL that doesn't redirect. GitHub Pages under the new repo currently returns 404, and whether Partner Center accepts a github.io URL is untested.

### Steps for winget (recommended)

1. Publish v1.8.0 with the Release workflow. **Never replace a release asset afterwards**: that breaks the hash.
2. Fill in the 1.8.0 manifests:
   - Start from the drafts in [`docs/publishing/winget/`](publishing/winget/) (the SpoolUp 1.8.0 manifests; the installer hash is filled in once the release exists).
   - Fill in `InstallerSha256` (upper case, from the release's `.sha256` asset) and `ReleaseDate`.
   - The changes from the 1.7.0 set below are listed after it.
3. Test on Windows:
   - Run `winget settings --enable LocalManifestFiles` as admin.
   - Run `winget validate --manifest <folder>`.
   - From a **non-elevated** terminal, run `winget install --manifest <folder>`. Expect one UAC prompt, no setup wizard, no profile copied into the game, and the firewall rule present.
   - Then run `winget list`, an upgrade over a running copy, and `winget uninstall`.
   - Repeat in Windows Sandbox with `Tools\SandboxTest.ps1` from winget-pkgs.
   - Scan the installer with Defender. Any antivirus or unwanted-software flag (Binary-Validation-Error) can't be waived.
4. Fork `microsoft/winget-pkgs` and add the three files under `manifests/x/xattribution/SpoolUp/1.8.0/` (one version, manifest files only). You can also run `wingetcreate submit <folder>` or `komac submit`.
   - PR title: `New package: xattribution.SpoolUp version 1.8.0`.
   - Sign the Microsoft CLA when the bot asks.
5. In the PR description, say:
   - the installer is machine-wide and admin-only;
   - it adds a Windows Firewall rule for Private networks only, removed on uninstall;
   - the app serves a local web page to the user's own devices;
   - `scprofile` is deselected on purpose;
   - the app works on its own, so it doesn't read as hardware-bound. A "Hardware" label closes the PR.
6. Watch the PR's Checks tab. If **Needs-Author-Feedback** appears, reply within 5 days: No-Recent-Activity is added after 5 days, and the PR is closed 3 days later.
7. After the merge, the package goes live within about an hour.
8. Turn on automatic updates:
   - Create a **classic** token with the `public_repo` scope. Fine-grained tokens don't work.
   - Use a separate bot account that owns the fork and has signed the CLA, because the token can write to all of that account's public repos.
   - Store it as the `WINGET_TOKEN` secret, and set the repository variable `WINGET_PACKAGE_ID=xattribution.SpoolUp`.
   - Add the job below.
9. For any later rename or change of publisher, repo or asset name, submit that version by hand.

### For reference: the manifests for Tablet Deck 1.7.0

Use the SpoolUp 1.8.0 set in [`docs/publishing/winget/`](publishing/winget/) instead; these show the format against the old release.

These are complete, with the hash filled in for the **nodieCode/tablet-deck** v1.7.0 asset. They were checked against the 1.28.0 JSON schemas. `winget validate` wasn't run, because it needs Windows. **I recommend submitting 1.8.0 instead; see the field changes below.** Files: `docs/publishing/winget/`.

`manifests/n/nodieCode/TabletDeck/1.7.0/nodieCode.TabletDeck.yaml`
```yaml
# yaml-language-server: $schema=https://aka.ms/winget-manifest.version.1.28.0.schema.json

PackageIdentifier: nodieCode.TabletDeck
PackageVersion: 1.7.0
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.28.0
```

`nodieCode.TabletDeck.installer.yaml`
```yaml
# yaml-language-server: $schema=https://aka.ms/winget-manifest.installer.1.28.0.schema.json

PackageIdentifier: nodieCode.TabletDeck
PackageVersion: 1.7.0
InstallerLocale: en-US
Platform:
- Windows.Desktop
MinimumOSVersion: 10.0.0.0
InstallerType: inno
Scope: machine
InstallModes:
- interactive
- silent
- silentWithProgress
InstallerSwitches:
  Custom: /MERGETASKS=!scprofile
UpgradeBehavior: install
ProductCode: '{A7A9440D-F358-4D02-81DE-04193AAA6A52}_is1'
ReleaseDate: 2026-10-26
ElevationRequirement: elevationRequired
InstallationMetadata:
  DefaultInstallLocation: '%ProgramFiles%\Tablet Deck'
Installers:
- Architecture: x64
  InstallerUrl: https://github.com/nodieCode/tablet-deck/releases/download/v1.7.0/TabletDeck-Setup-1.7.0.exe
  InstallerSha256: C6419E211D6E78A4CC0E69A4E95256B5C67D6F59768E3201CDBB5248041C8940
ManifestType: installer
ManifestVersion: 1.28.0
```

`nodieCode.TabletDeck.locale.en-US.yaml`
```yaml
# yaml-language-server: $schema=https://aka.ms/winget-manifest.defaultLocale.1.28.0.schema.json

PackageIdentifier: nodieCode.TabletDeck
PackageVersion: 1.7.0
PackageLocale: en-US
Publisher: nodieCode
PublisherUrl: https://github.com/nodieCode
PublisherSupportUrl: https://github.com/nodieCode/tablet-deck/issues
PrivacyUrl: https://github.com/nodieCode/tablet-deck#pairing-and-network
Author: nodieCode
PackageName: Tablet Deck
PackageUrl: https://github.com/nodieCode/tablet-deck
License: MIT
LicenseUrl: https://github.com/nodieCode/tablet-deck/blob/v1.7.0/LICENSE
Copyright: Copyright (c) 2026 nodieCode
ShortDescription: Turns a tablet or phone on your home network into a touch control panel for Star Citizen.
Description: |-
  Tablet Deck is a HUD-style button deck that runs in the web browser of a tablet or phone on the same network as the gaming PC. Each button press is sent to the PC, which presses that button's key once, and only while Star Citizen is the window in front.
  The PC app serves the deck, pairs devices with a QR code and pairing code, follows the player's own keybinds and offers hold-to-talk voice commands, a layout editor and 12 manufacturer themes. Tablets use their browser or the SpoolUp app.
  Setup adds a Windows Firewall rule so tablets can connect on private networks.
  Fan project. Not affiliated with or endorsed by Cloud Imperium Games.
Moniker: tabletdeck
Tags:
- button-box
- control-panel
- hotkeys
- hud
- keybinds
- phone
- star-citizen
- tablet
- touchscreen
- voice-commands
ReleaseNotesUrl: https://github.com/nodieCode/tablet-deck/releases/tag/v1.7.0
InstallationNotes: |-
  Open Tablet Deck from the Start menu: its window shows the QR code, address and pairing code for your tablet.
  WinGet doesn't copy the Star Citizen keybind profile into the game; add it with Settings > Install keybind profile... in the Tablet Deck window, then load "Tablet Deck Keyboard" in the game.
Documentations:
- DocumentLabel: Get started
  DocumentUrl: https://github.com/nodieCode/tablet-deck#get-started
ManifestType: defaultLocale
ManifestVersion: 1.28.0
```

**For 1.8.0 (recommended), change these fields.** Everything else stays the same.
- `PackageIdentifier: xattribution.SpoolUp`, in the folder `manifests/x/xattribution/SpoolUp/1.8.0/`.
- `Publisher: xattribution` and `PackageName: SpoolUp`.
- Every `nodieCode/tablet-deck` URL becomes `xattribution/SpoolUp`, and `v1.7.0` becomes `v1.8.0`.
- `InstallerUrl: https://github.com/xattribution/SpoolUp/releases/download/v1.8.0/SpoolUp-Setup-1.8.0.exe`
- `InstallerSha256: <SHA256 of SpoolUp-Setup-1.8.0.exe, upper case>` and `ReleaseDate: <YYYY-MM-DD>`. Neither can be known until the release exists.
- `Moniker: spoolup`. The InstallationNotes name the "SpoolUp Keyboard" profile.

### Release-workflow step that keeps it updated

The job has to live **inside** `.github/workflows/release.yml`. The release is created with `GITHUB_TOKEN`, and a release made that way doesn't start a separate `on: release` workflow. The job only runs once the first version has been merged by hand.

First, in the existing `build` job, expose the tag:
```yaml
  build:
    outputs:
      release-tag: ${{ steps.release.outputs.tag }}
    # in the "Work out the release" step (id: release), also write:
    #   "tag=$tag" >> $env:GITHUB_OUTPUT
```
Then add this job:
```yaml
  winget:
    needs: build
    if: needs.build.outputs.release-tag != '' && vars.WINGET_PACKAGE_ID != ''
    runs-on: windows-latest
    permissions:
      contents: read
    steps:
      - name: Submit the release to WinGet
        shell: pwsh
        env:
          GH_TOKEN: ${{ github.token }}
          TAG: ${{ needs.build.outputs.release-tag }}
          PACKAGE_ID: ${{ vars.WINGET_PACKAGE_ID }}
          WINGET_CREATE_GITHUB_TOKEN: ${{ secrets.WINGET_TOKEN }}
        run: |
          if (-not $env:WINGET_CREATE_GITHUB_TOKEN) { throw "Set the WINGET_TOKEN secret." }
          $version = $env:TAG.TrimStart("v")
          $setup = gh release view $env:TAG --repo "${{ github.repository }}" --json assets --jq '.assets[].name' |
            Where-Object { $_ -match '-Setup-.+\.exe$' } | Select-Object -First 1
          if (-not $setup) { throw "Release $env:TAG has no *-Setup-*.exe." }
          $url = "https://github.com/${{ github.repository }}/releases/download/$env:TAG/$setup"
          # wingetcreate pinned by version and SHA-256, like jsign
          $exe = Join-Path $env:RUNNER_TEMP "wingetcreate.exe"
          Invoke-WebRequest "https://github.com/microsoft/winget-create/releases/download/v1.12.13.0/wingetcreate.exe" -OutFile $exe
          if ((Get-FileHash $exe -Algorithm SHA256).Hash -ne "24042BD37915805615E6CF969AC57C6439124C3FE85823327F5F3FB24BD9FFEA") { throw "wingetcreate.exe doesn't match its SHA-256" }
          & $exe update $env:PACKAGE_ID --version $version --urls $url `
            --release-notes-url "https://github.com/${{ github.repository }}/releases/tag/$env:TAG" `
            --submit --no-open
          if ($LASTEXITCODE -ne 0) { throw "wingetcreate update failed with exit code $LASTEXITCODE" }
```
Notes:
- wingetcreate 1.12.13.0 writes schema 1.12.0, which is still accepted; 1.28.0 is recommended. It also has telemetry on by default.
- Komac 2.17.2 writes 1.28.0 and has no telemetry. It's available through WinGet Releaser; pin that action to a commit SHA.
- The full draft, with comments, is in [`docs/publishing/winget/release-workflow-step.yml`](publishing/winget/release-workflow-step.yml).

### Microsoft Store: the short version

1. Get answers from Partner Center support on Artifact Signing (policy 10.2.9) and on the game-product question (policy 10.1).
2. Open a free account at https://storedeveloper.microsoft.com. Other entry points show the old flow, which has fees.
   - Individual accounts publish "under your own name" and can't become Company accounts later.
   - Policy 10.14 requires a Company account if the app or publisher name reads like a business.
3. Reserve the name "SpoolUp": Partner Center (https://aka.ms/submitwindowsapp) > New product > EXE or MSI app. An unused reservation lapses after 3 months.
4. Fill in the submission:
   - Free; category **Utilities + tools**; the IARC age-rating questionnaire; a privacy policy; support = GitHub issues.
   - Package: the versioned, non-redirecting HTTPS URL, x64, en-us, EXE.
   - Installer parameters: `/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /SP- /MERGETASKS=!scprofile`. Use `/VERYSILENT`, because `/SILENT` still shows a progress window.
   - Return codes: 2 and 5 = canceled; 8 = reboot required. That's the closest match: Inno's 8 means setup stopped *before* installing and needs a restart. Codes 1, 3, 4 and 7 go under miscellaneous, with the Inno exit-code page as the documentation URL.
5. Notes for certification:
   - the deck needs a second device's browser, or a browser on the same PC;
   - presses only reach Star Citizen while it's the focused window;
   - the Private-network firewall rule;
   - the optional updater;
   - a silent uninstall keeps `%APPDATA%\TabletDeck`, which the clean-uninstall test checks.
6. Certification takes up to 3 business days, and the listing is live about 15 minutes after it passes. Then `winget install <XP-ID> -s msstore` works. That it works without the Store app is backed only by community reports.
7. Each release needs an Update submission with a new versioned URL. It can be automated through the Store submission API or `msstore`, which need a Microsoft Entra tenant and work only for free apps that are already live.
   - Keep the in-app updater: the Store never updates existing EXE users.
   - If the app is in both sources, `winget install SpoolUp` asks users to choose a source.

---

## 3. Things to decide or check

**Decide**
- The final name and winget PackageIdentifier, before the first upload anywhere.
- Whether to fix `scprofile` in the installer, rather than relying on `/MERGETASKS=!scprofile` in every manifest.
- Whether to list 1.7.0 at all. I recommend not; if you do, decide which repo's binary is canonical.

**Check (needs Windows or real hardware)**
- `winget validate`, the non-elevated install, the upgrade over a running copy (held keys released), and the uninstall.
- Defender and other antivirus engines on the signed installer, `keysender.exe` and the server executable.

**Couldn't be confirmed**
- Whether the Store accepts Artifact Signing–signed EXEs. Microsoft's two sources conflict.
- Whether a Store reviewer treats SpoolUp as a game product.
- Whether Partner Center accepts a GitHub or github.io Package URL.
- Whether an Individual Store account can show "xattribution" as publisher, or link an Entra tenant for automation.
