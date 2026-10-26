# Installs or updates Tablet Deck from the latest GitHub release, quietly.
#   irm https://github.com/xattribution/tablet-deck-spooned/releases/latest/download/install.ps1 | iex
# Uninstall: Settings > Apps > Tablet Deck, or run "unins000.exe" in its folder.
$ErrorActionPreference = "Stop"
$repo = "xattribution/tablet-deck-spooned"
$rel = Invoke-RestMethod "https://api.github.com/repos/$repo/releases/latest" -Headers @{ "User-Agent" = "tabletdeck-install" }
$asset = $rel.assets | Where-Object { $_.name -like "TabletDeck-Setup-*.exe" } | Select-Object -First 1
if (-not $asset) { throw "No installer in the latest release." }
$file = Join-Path $env:TEMP $asset.name
Write-Host "Downloading $($asset.name)..."
Invoke-WebRequest $asset.browser_download_url -OutFile $file -UseBasicParsing
$sum = $rel.assets | Where-Object { $_.name -eq "$($asset.name).sha256" } | Select-Object -First 1
if ($sum) {
  $want = ((Invoke-RestMethod $sum.browser_download_url) -split "\s+")[0].ToLower()
  $got = (Get-FileHash $file -Algorithm SHA256).Hash.ToLower()
  if ($want -ne $got) { Remove-Item $file; throw "Checksum mismatch: the download is damaged." }
}
Write-Host "Installing (Windows asks for admin rights once, for Program Files and the firewall rule)..."
Start-Process $file -ArgumentList "/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/TASKS=startup,scprofile" -Verb RunAs -Wait
Remove-Item $file -ErrorAction SilentlyContinue
Write-Host "Tablet Deck $($rel.tag_name) is installed and running in the tray."
