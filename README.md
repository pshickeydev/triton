# triton

A terminal user interface (TUI) for live-coding music with [Tidal Cycles](https://tidalcycles.org/). If you are new here: Tidal Cycles is a Haskell-based language for describing musical patterns; SuperCollider handles the sound. This tool provides a terminal-based editor and controller that connects to both, letting you edit patterns and manage playback without leaving your terminal.

Built with [Charmbracelet](https://charm.land/) tools (Bubble Tea, Lip Gloss, Bubbles).

## What It Does

triton is a TUI that acts as a live-coding controller for Tidal Cycles. It provides a code editor for writing Tidal patterns, transport controls (mute/solo/hush), and live pattern status, all from the terminal.

The TUI does **not** produce sound. It communicates with two external processes:

- **GHCi** (Haskell REPL running BootTidal.hs) — receives pattern code via stdin
- **SuperCollider + SuperDirt** — the audio engine, receiving OSC from Tidal

## Prerequisites

| Dependency | Purpose |
|---|---|
| [Go](https://go.dev/) 1.24+ | Build and run triton |
| [GHC](https://www.haskell.org/ghc/) / GHCi | Haskell runtime for Tidal |
| [Tidal Cycles](https://tidalcycles.org/) | Pattern engine (Haskell library) |
| [SuperCollider](https://supercollider.github.io/) + [SuperDirt](https://github.com/musikinformatik/SuperDirt) | Audio synthesis |
| [sc3-plugins](https://github.com/supercollider/sc3-plugins) (optional) | UGens for SuperDirt's synth sound banks |
| [cabal](https://www.haskell.org/cabal/) | Haskell package manager |

## Setup

### 1. Install Tidal as a local package

From the project directory:

```bash
cabal install --lib tidal --package-env .
```

This creates a `.ghc.environment` file so plain `ghci` can see Tidal when started from this directory.

### 2. Install SuperDirt (first time only)

```bash
sclang install-superdirt.scd
```

### 3. Install sc3-plugins (optional, recommended)

Without [sc3-plugins](https://github.com/supercollider/sc3-plugins) (matching your SuperCollider version), SuperDirt skips its synth sound banks, so patterns using instruments like `supermandolin`, `supergong`, or `superpiano` stay silent. Sample-based sounds (`bd`, `sn`, ...) work either way.

triton watches SuperDirt's boot output and logs a warning in the console pane when sc3-plugins is missing.

## Running

```bash
go run .
```

Pass a `.tidal` file to open it at startup:

```bash
go run . examples/basic.tidal
```

This will:

1. Start SuperCollider/SuperDirt (`sclang start-superdirt.scd`)
2. Wait for SuperDirt to signal ready
3. Start GHCi and load the boot script (the bundled cursor-enabled BootTidal-triton.hs by default)
4. Launch the TUI

## Keybindings

| Key | Action |
|---|---|
| `Ctrl-J` | Evaluate current block (multi-line aware) |
| `Ctrl-L` | Evaluate current line only |
| `Ctrl-\` | Hush (stop all patterns) |
| `Alt-M` | Mute all |
| `Ctrl-U` | Unmute all |
| `F1`-`F8` | Toggle mute on d1-d8 |
| `Alt-1`-`Alt-8` | Toggle solo on d1-d8 |
| `Alt-[` / `Alt-]` | Decrease / increase tempo (cps) |
| `Ctrl-R` | Restart GHCi process |
| `Tab` / `Shift-Tab` | Large (>= 110x34): cycle focus. Small (60x20–109x33): swap editor / status+console. Below 60x20: too-small notice |
| `Ctrl-S` | Save buffer to the current file (unsaved: prompts for a name; existing: asks to overwrite) |
| `Ctrl-O` | Open a `.tidal` file (type a path, `Enter` to load, `Esc` to cancel; unsaved changes ask first) |
| `Ctrl-C` | Quit (press twice to confirm, `Esc` to cancel) |

## Pattern Status

The status panel shows each d-stream's state: playing (highlighted), muted, soloed, or idle.

Streams with unevaluated edits are marked with a `*` suffix. The comparison is per-pattern (`d1 $ ...`), not per evaluation block. `Ctrl-J` evaluates the block under the cursor; `Ctrl-L` evaluates only the current line. The `*` clears once the playing code still appears in the editor. Evaluating only part of a multi-line pattern keeps the marker.

Hush (`Ctrl-\`) and GHCi restart (`Ctrl-R`) stop all patterns. Any code left in the editor then shows as edited until evaluated again.

## Saving and Loading

The editor buffer is a plain `.tidal` file (Tidal/Haskell source).

- `triton path/to/set.tidal` opens a file at startup.
- `Ctrl-S` saves to the current file; a buffer that was never saved opens a prompt prefilled with `untitled.tidal`. Saving over an existing file asks for confirmation first (`Enter` overwrites, `Esc` goes back to the prompt).
- Saves are atomic: an interrupted or failed save leaves the previous file intact.
- `Ctrl-O` opens a file: type a path (or a `~/...` path) and press `Enter`; `Esc` cancels. Opening with unsaved changes asks first (`Enter` discards them and opens, `Esc` goes back to the prompt).
- The help bar shows the current file; `*` after the name marks unsaved changes.
- Quitting with unsaved changes says so in the `Ctrl-C` confirmation.
- Loading a file replaces the editor buffer but leaves playing patterns alone: they keep sounding and show as edited until re-evaluated.

## Configuration

Config file at `~/.config/triton/config.toml`:

```toml
[ghci]
executable = "ghci"
boot_script = "/path/to/BootTidal.hs"   # only needed with cursor.enabled = false
working_dir = "."

[osc]
tidal_control_host = "127.0.0.1"
tidal_control_port = 6010

[superdirt]
executable = "sclang"
boot_script = "start-superdirt.scd"   # must print "SuperDirt ready" when done
auto_start = true

[ui]
theme = "dark"
default_tempo = 0.6
visible_streams = 8   # 1-16

[cursor]
enabled = true
host = "127.0.0.1"
port = 6013
boot_script = ""
```

Environment variables override config values with the `TIDAL_CHARM_` prefix (e.g. `TIDAL_CHARM_SUPERDIRT_AUTO_START=false`).

## Pattern Tracing Cursor

With the cursor enabled (the default), the mini-notation fragment that is currently sounding is highlighted in the editor in the stream's color, advancing with the cycle as the pattern plays.

How it works: when the cursor is enabled, triton boots GHCi with its own boot script (`boot/BootTidal-triton.hs`, bundled with the app) instead of the stock `BootTidal.hs`. That script adds a second OSC target so Tidal reports each event's source positions to the TUI, which listens on `127.0.0.1:6013`. Evaluated code is transparently rewritten so the reported positions point into the live buffer.

Notes:

- Requires Tidal >= 1.9 (the `Sound.Tidal.Boot` / `mkTidalWith` API). On older versions, disable the cursor (`cursor.enabled = false`) so the stock boot script is used.
- `cursor.boot_script` overrides the bundled boot script (e.g. with your own copy of BootTidal.hs that has the same second target).
- Highlights are suppressed once their positions no longer match the editor: streams with unevaluated edits (marked `*`) and patterns deleted from the editor while still sounding. They clear on hush or GHCi restart.
- Port 6013 must be free; change it with `cursor.port` if something else uses it.

## Examples

See `examples/basic.tidal` and `examples/advanced.tidal` for sample Tidal patterns. Paste or type them into the editor and press `Ctrl-J` to evaluate.

## Quick Start

Requires Go 1.24+, GHC, and a working Tidal Cycles / SuperDirt setup (see Prerequisites below).

```bash
go build -o triton .
./triton examples/basic.tidal
```

Local state (SQLite DB, logs) is kept in `.crush/`; it is excluded from version control and safe to delete.

## Project Structure

```
triton/
├── main.go                        # Entry point
├── triton.cabal              # Haskell project (Tidal dependency)
├── Repl.hs                        # Dummy Haskell module for cabal
├── boot/BootTidal-triton.hs         # Cursor-enabled Tidal boot script
├── start-superdirt.scd            # SuperCollider boot script
├── install-superdirt.scd          # SuperDirt installer
├── internal/
│   ├── config/config.go           # TOML config + env overrides
│   ├── doc/doc.go                 # Editor document: .tidal load/save, dirty state
│   ├── ghci/process.go            # GHCi subprocess manager
│   ├── osc/client.go             # OSC client (Tidal control)
│   ├── superdirt/process.go       # SuperCollider/SuperDirt manager
│   ├── model/model.go             # Top-level Bubble Tea model
│   ├── cursor/listener.go         # OSC listener for Tidal highlight messages
│   ├── cursor/spans.go            # Active highlight span state
│   ├── deltamini/deltamini.go     # Go port of Tidal's deltaMini rewrite
│   ├── status/tracker.go          # Manual stream state tracking
│   ├── status/panel.go            # Status panel renderer
│   ├── status/console.go          # Console log renderer
│   └── textarea/                  # Vendored bubbles textarea with
│                                  #   highlight-span rendering
├── ui/
│   ├── theme.go                   # Dark/light themes + color palette
│   ├── styles.go                  # Lip Gloss style definitions
│   └── layout.go                  # Adaptive layout: large (editor | status+console) or small (swappable single view)
└── examples/
    ├── basic.tidal
    └── advanced.tidal
```

## Development

Run `gofmt -w` on modified Go files to keep formatting consistent.

See [docs/DESIGN.md](docs/DESIGN.md) for the full architecture document and [AGENTS.md](AGENTS.md) for code style and commit conventions.
