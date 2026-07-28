# 7D2D Mod Update Tracker

A cross-platform Go desktop application, built with Fyne, that scans a
**7 Days to Die** `Mods` directory and compares installed versions with
configured mod pages.

## Development

Install [mise](https://mise.jdx.dev/), a C compiler, and the graphics
development packages required by Fyne. The committed `mise.toml` installs the
project's pinned Go version:

```bash
mise install
make run
```

The included `FyneApp.toml` opts into Fyne's single-event-thread `fyne.Do`
migration. Network requests run in a background goroutine, and their UI
updates are queued through `fyne.Do`.

The app checks the common Steam and per-user mod locations on Linux, macOS,
and Windows, including Proton's Windows `%APPDATA%` prefix on Linux. Detection
prefers locations containing real community mods and favors a populated
per-user folder over the Steam installation on ties. You can rerun detection
or choose any folder manually from Settings.

Each immediate subfolder is treated as a mod. Metadata is read from the
standard nested `ModInfo.xml` structure (`xml > ModInfo`), including `Name`,
`Description`, `Author`, `Version`, and `Website`. Flat legacy files are also
accepted. Versions may be simple increasing numbers (`2`) or dotted identifiers
(`6.5.5`). When no manually configured source exists, a `Website` value on
`nexusmods.com` or `7daystodiemods.com` is used as that mod's initial update
source. Other websites remain metadata only. Update sources are deliberately
restricted to these two supported domains.

The game-managed Harmony wrapper (`0_TFP_Harmony`, including the
`0_TPF_Harmony` spelling variant) is excluded from scans and never displayed.

If community mods are detected in the Steam game installation, the app offers
to move them to the platform's per-user Mods folder. On Proton it uses the
Windows `%APPDATA%` directory inside Steam's app-251570 prefix. The
game-managed `0_TFP_Harmony` folder is always left in the installation.

Configured sources are stored by the stable `ModInfo.xml` name, not by the
install folder name. Only each site's identifier is persisted:

```json
"sources": {
  "AGF - V3 - HUDPlus": {
    "nexus": "870",
    "7d2dmods": "agf-v3-hudplus-1main"
  }
}
```

If a mod is not available from one of the two providers, that provider is
stored as `null`.

1. Use the settings cog to choose the Mods folder and optionally add a Nexus
   API key.
2. Choose **Edit…** on a mod row to configure its sources.
3. Choose **Check updates**.
4. Click a configured Nexus or 7D2D Mods button to open that source directly.

Use **Open Mods** in the main toolbar to open the currently configured Mods
directory in the system file manager.

The source editor opens as a resizable window at 75% of the main window's
width. It has one field per provider and accepts either a short Nexus mod
ID / 7D2D Mods slug or a full mod-page URL.

The main view uses one card per mod. Each card shows installed/latest versions,
status, direct Nexus and 7D2D Mods buttons, and its own source-edit action.
Provider buttons show only the provider name and are hidden when that source is
not configured; IDs and slugs remain in configuration.
Cards with an available update use a prominent green border and update label.
They are also sorted to the top of the card list automatically.
The card grid responds to window width, adding more columns as enough
horizontal space becomes available.

Update checks run automatically after a successful scan for every mod with at
least one valid source. The manual **Check updates** button is disabled for the
duration of a check to prevent overlapping requests.

Daily append-only logs are stored under the operating system's user cache
directory in `7d2d-mod-tracker/logs`. Use **Settings → Open logs** to open that
folder. Logs include scans, update results, and operational errors; the Nexus
API key is never written to them.

For the example `AGF-HUDPlus-1Main-v6.5.5` folder, the two supplied source
URLs are suggested automatically when setting sources.

## Nexus Mods

Public pages are checked without credentials where possible. Nexus can change
or restrict its HTML, so an API key is the reliable option. Add one through
the settings cog; it is stored only in the app's user configuration file.

The application only reports updates. It intentionally does not download,
overwrite, or remove mods.

## Test and package

```bash
make test
make build
```

Install the packaging tools once:

```bash
make tools
```

Create a package for the current OS:

```bash
make package-native
```

With Docker running, build all supported desktop OS/architecture combinations:

```bash
make cross-all
```

Individual `cross-linux`, `cross-windows`, `cross-darwin`, and
`cross-freebsd` targets are also available. Mobile targets are intentionally
excluded: this application manages desktop game files and has no useful mobile
sandbox workflow.

The Linux target builds a small derivative of the official fyne-cross image
that adds the EGL development headers required by current GLFW/Fyne releases.
This workaround is defined in `packaging/fyne-cross-linux.Dockerfile`.

Tagged GitHub releases include a portable Linux amd64 AppImage in addition to
the generic Fyne archive. macOS is packaged as separate Intel and Apple Silicon
DMG downloads with the usual drag-to-Applications layout. These macOS packages
are currently unsigned and are therefore subject to Gatekeeper warnings.

### Distribution notes

- Add a square `Icon.png` before public packaging. Fyne uses
  `FyneApp.toml` for the application name, ID, version, build number, desktop
  metadata, and default icon.
- Cross-built packages are suitable for testing, but public macOS and Windows
  releases should be produced and signed on their native OS. macOS distribution
  also normally requires Apple notarization.
- Unsigned Windows binaries can trigger SmartScreen warnings. Code signing
  improves reputation and verifies the publisher.
- Linux users may prefer an AppImage, Flatpak, distro package, or the tarball
  produced by Fyne. Test both X11 and Wayland environments.
- Publish SHA-256 checksums with releases and test each artifact on a clean
  machine or VM.
- Choose and include a project license before public distribution. Review the
  licenses and notices of Fyne and other bundled dependencies.
- Document that the app makes outbound requests to Nexus Mods and
  7daystodiemods.com, and that an optional Nexus API key is stored locally.
