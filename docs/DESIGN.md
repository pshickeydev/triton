# triton — Design Document

A terminal user interface for live-coding music with Tidal Cycles, built with Charmbracelet tools (Bubble Tea, Lip Gloss, Bubbles).

## 1. Overview

triton is a TUI that acts as a live-coding controller for Tidal Cycles. It provides a code editor for writing Tidal patterns, transport controls (mute/solo/hush), and live pattern visualization — all from the terminal.

The TUI does **not** produce sound. It communicates with two external processes:

- **GHCi** (Haskell REPL running BootTidal.hs) — receives pattern code via stdin
- **SuperCollider + SuperDirt** — the audio engine, receiving OSC from Tidal

triton is a thin control layer: it sends code to GHCi for evaluation and OSC messages to Tidal's control port for transport commands.

## 2. Architecture

```mermaid
flowchart TD
    subgraph TC [triton (Go)]
        Editor[Code Editor<br/>Bubbles textarea]
        Status[Pattern Status Panel<br/>- d1..d8 state<br/>- tempo (cps)<br/>- GHCi status]
        Console[Console Log<br/>GHCi stdout/stderr]
        OSCClient[OSC Client<br/>/mute /unmute /solo<br/>/hush /muteAll /setcps]
        Editor --> Status
        Status --> Console
    end

    Editor -->|stdin pipe| GHCi[GHCi subprocess<br/>BootTidal.hs]
    GHCi -->|stdout/stderr| Console
    GHCi -->|OSC UDP :57120| SC[SuperCollider<br/>+ SuperDirt]

    OSCClient -->|OSC :6010 control| GHCi
```

### 2.1 Data Flow

1. **Code evaluation**: User edits Tidal code in the textarea, presses `Ctrl-J`. The TUI extracts the block of contiguous non-empty lines around the cursor and writes it to GHCi's stdin pipe. GHCi evaluates it, updating the pattern in Tidal's `Stream`. Tidal's frame-based scheduler queries the pattern and emits OSC bundles to SuperDirt on port 57120. Multi-line blocks are sent using GHCi's `:{` ... `:}` delimiters.

2. **Transport control**: User presses a key (e.g., `Ctrl-\` to hush, `F1` to toggle mute on d1). The TUI sends an OSC message to Tidal's control port (127.0.0.1:6010). Tidal acts on it immediately.

3. **Status feedback**: The TUI tracks its own transport commands (which streams it muted/soloed) and displays that state. GHCi stdout/stderr is forwarded to the console log pane for error display.

4. **SuperDirt lifecycle**: On startup, the TUI spawns `sclang` with the SuperDirt boot script, waits for the ready signal, then starts GHCi. On exit, both subprocesses are cleaned up.

5. **File persistence**: The editor buffer loads from and saves to `.tidal` files through `internal/doc` (see 3.10) — no external processes involved.

### 2.2 Process Management

The TUI manages three subprocess lifecycles:

- **SuperCollider/SuperDirt** (`internal/superdirt/process.go`): Started first on launch. Scans stdout for the `SuperDirt` ready marker. Has a 30-second boot timeout. Cleaned up on exit.
- **GHCi** (`internal/ghci/process.go`): Started after SuperDirt is ready. Sends `:script BootTidal.hs` through stdin on startup. Supports restart via `Ctrl-R` using an epoch counter to invalidate stale goroutines; the close-and-respawn runs in a Bubble Tea command so a slow shutdown does not freeze the UI, and a quit requested during a restart completes once the restart settles. Graceful shutdown with a 3-second timeout before kill.
- **Cleanup order**: the model closes the current GHCi process before quitting (restarts replace the process, so the model owns the live handle), then `main`'s deferred closes act as a safety net for GHCi and SuperDirt.

## 3. Components

### 3.1 GHCi Process Manager

Manages a GHCi subprocess running BootTidal.hs.

**Responsibilities:**
- Spawn GHCi with the correct working directory
- Send `:script BootTidal.hs` through stdin after GHCi starts (not as a CLI flag)
- Write evaluated code lines to GHCi's stdin
- Read GHCi's stdout/stderr for errors and display them
- Handle process lifecycle (restart on crash, cleanup on exit)

**Key design decisions:**
- Use `os/exec` with `cmd.StdinPipe()` and `cmd.StdoutPipe()`
- Run stdout/stderr reading in goroutines, forwarding to a channel
- Output channel is closed after both reader goroutines finish (via `sync.WaitGroup`) to prevent goroutine leaks
- Multi-line blocks sent using GHCi's `:{` ... `:}` delimiters
- Single-line evaluation sends the code followed by a newline
- `writeLocked()` is a private method shared by `Evaluate` and `EvaluateMulti` to avoid mutex re-entrancy deadlock
- BootTidal.hs path is configurable; default points to the cabal store location
- GHCi process is started from the project directory so the `.ghc.environment` file (created by `cabal install --lib tidal --package-env .`) makes Tidal visible without global registration
- Graceful shutdown: sends `:quit`, waits up to 3 seconds, then kills

### 3.2 SuperDirt Process Manager

Manages the SuperCollider/SuperDirt subprocess.

**Responsibilities:**
- Spawn `sclang` with the SuperDirt boot script
- Detect readiness by scanning stdout for the `SuperDirt` marker
- Wait for ready signal before GHCi starts (30-second timeout)
- Clean up on exit (5-second graceful timeout before kill)

**Key design decisions:**
- `markReady()` closes the `readyCh` channel inside the mutex to prevent double-close panics from concurrent stdout/stderr goroutines
- Output channel closed after both reader goroutines finish via `sync.WaitGroup`
- Auto-start can be disabled via config (`superdirt.auto_start = false`)

### 3.3 OSC Client

Sends control messages to Tidal's inbound OSC port (127.0.0.1:6010).

**Responsibilities:**
- Send `/hush` — stop all patterns
- Send `/muteAll` / `/unmuteAll`
- Send `/mute <n>` / `/unmute <n>` — mute/unmute pattern stream dn (n is 1-indexed, matching Tidal's pattern IDs where `d1 = p 1`)
- Send `/solo <n>` / `/unsolo <n>`
- Send `/setcps <float>` — set the tempo (cycles per second)

**Implementation:**
- Library: `github.com/hypebeast/go-osc` (mature Go OSC library)
- Tidal's OSC address format: `/mute`/`/unmute`/`/solo`/`/unsolo` take an `Int32` pattern ID that Tidal stringifies to match the pattern name ("1".."16"), so n is 1-indexed; `/setcps` takes a single `Float`
- All messages sent over UDP to 127.0.0.1:6010 (configurable)
- OSC addresses, max stream number, and error messages are extracted to constants
- `validateStream()` helper centralizes the 1-16 range check

### 3.4 Code Editor

An embedded code editor for writing Tidal/Haskell pattern code.

**Responsibilities:**
- Multi-line text editing (Bubbles `textarea` component)
- Evaluate on `Ctrl-J` — extract and send the contiguous block of non-empty lines around the cursor
- Evaluate on `Ctrl-L` — send only the current line
- Block extraction via `extractBlock()` which finds the surrounding non-empty lines

**Implementation notes:**
- Bubbles `textarea` provides basic multi-line editing but lacks syntax highlighting
- Syntax highlighting is deferred to Phase 3
- `extractBlock()` clamps the cursor line to bounds, returns empty string for blank lines, expands outward to find contiguous non-empty lines
- Multi-line blocks are sent to GHCi using `:{` ... `:}` delimiters via `EvaluateMulti()`

### 3.5 Status Panel

Displays live information about the Tidal session.

**Information shown:**
- Active streams: d1–d8 (configurable up to 16), with visual indication of muted/active/soloed state
- Edited markers: streams whose editor code differs from the code last evaluated for them are shown with a `*` suffix in the edited color, distinguishing patterns that are playing from patterns with unevaluated edits
- Tempo (cps — cycles per second)
- GHCi connection status (running/stopped)

**Challenge — obtaining status:**
Tidal does not expose rich state over OSC. Options for getting state:

1. **Parse GHCi stdout**: GHCi prints type information and error messages. Not reliable for pattern state.
2. **Custom Haskell hooks**: Add custom logging in BootTidal that sends OSC status messages to a local port the TUI listens on. This requires modifying BootTidal.hs.
3. **OSC snooping**: Listen on a promiscuous OSC sniffer to observe outbound messages to SuperDirt (port 57120). Can infer active streams from which `/dirt/play` messages are flowing. This is a read-only tap that doesn't interfere.
4. **Manual tracking**: The TUI tracks its own commands (which streams it muted/soloed) and displays that state. Doesn't reflect changes made outside the TUI (e.g., from another editor), but is simplest to implement.

**Current approach:** Option 4 (manual tracking) is implemented. The `status.Tracker` maintains stream state (idle/active/muted/soloed) with mutex-protected access. Mute and solo toggles revert to the previous state when the OSC send fails, so the panel never claims a state Tidal did not apply. The `status.Panel` renders the tracker state using the stream palette derived from the active theme.

**Playing vs edited:** Pattern code is compared per stream, not per block. Both the editor and the evaluated code are split into segments: a segment starts at a `dn $` assignment line and ends at the next assignment or blank line, so continuation lines belong to the pattern above them. Successful evaluation records each stream's segment of the evaluated code (`Tracker.SetPlaying`, keeping muted/soloed transport state intact); when a block assigns the same stream more than once, the last assignment is the one playing. Whenever the editor content changes, the model recomputes for each stream whether the code last evaluated for it still appears in the editor as one of its segments (modulo whitespace) and flags mismatches as edited: the pattern currently playing is not what the editor shows. This makes `Ctrl-J` (block) and `Ctrl-L` (line) behave consistently: evaluating a stream's full pattern clears its flag, evaluating only part of a multi-line pattern keeps it, and editing the editor afterwards restores it. Hush and GHCi restart clear all recorded playing code, since no pattern keeps playing after either. Unmuting (`Ctrl-U`, or the second press of a mute toggle) restores a muted stream to active only when a pattern is recorded as playing on it; otherwise it returns to idle, so "active" always means a pattern is playing.

Caveats: evaluation is recorded optimistically — a Haskell error reported asynchronously by GHCi does not restore the marker — and transport expressions evaluated as editor text (e.g. a bare `hush` line) are not tracked; the marker reflects only what the TUI evaluated itself. Assignment detection matches `dn $` at the start of a line.

### 3.6 Console Log

A dedicated pane below the status panel on the right side, showing the GHCi session log.

**Information shown:**
- GHCi stdout/stderr output (evaluation results, errors)
- TUI status messages (restarts, transport command errors)

**Implementation:**
- `status.Log` wraps a Bubbles `viewport`, so the log scrolls (PgUp/PgDn/arrows when focused) and wraps long lines at the pane width instead of overflowing the frame
- Incoming GHCi output is split into lines and auto-scrolls to the bottom; the buffer keeps 500 lines, trimming to 250
- The right column is split vertically: status panel on top, console log below

### 3.7 Adaptive Layout

The layout adapts to the terminal size with three modes (`ui.Mode`):

| Mode | Threshold | Behavior |
|---|---|---|
| Large | >= 110x34 | Side-by-side: editor on the left, status + console stacked on the right |
| Small | >= 60x20 | Single view: either the editor or the status + console stack fills the window; `Tab` swaps between them |
| Too small | < 60x20 | A notice stating the 60x20 minimum is shown instead of the panes |

**Rationale for thresholds:**
- **60x20** is the user-facing minimum. The status/help line below the panes takes 2 rows, so the pane area threshold is 60x18: 60 columns leave usable editor and status content widths after pane frames, and 18 rows fit the status panel content (title, streams, cps, ghci) above a minimum console viewport. The too-small notice tells the user the 60x20 requirement.
- **110x34** derives from the side-by-side layout's own minimums: the editor needs at least its `minEditorWidth` (30) of content plus a 2/3 share that keeps the ratio meaningful, and the status column needs its `minStatusWidth` (20) plus frames. Vertically, the status panel content (title, streams, cps, ghci rows) plus the minimum console height (3) plus pane frames and the help bar fit within 34 rows. Below this, the split would crush either the editor or the right column into their minimum clamps and waste space.

**Implementation:**
- `ui.Layout.SetSize` classifies the terminal into a mode on every `WindowSizeMsg`.
- In small mode, the model tracks an active `viewArea` (editor vs status+console). `Tab`/`Shift-Tab` swaps views and moves focus: console gets focus when the status+console view is active (so PgUp/PgDn scroll immediately), editor gets focus otherwise.
- In large mode, `Tab`/`Shift-Tab` cycles focus across the three panes as before; the view field is irrelevant.
- Pane content sizes (`textarea`, `panel`, `console`) are computed from the same width/height accessors regardless of mode, so resize between modes only re-renders.

### 3.8 Visualization Panel

Renders a visual representation of the current pattern cycle.

**Concept:**
- Show a horizontal timeline of one cycle (or a configurable number of cycles)
- Render events as blocks positioned by their time span within the cycle
- Color-code by orbit/stream (d1 = red, d2 = blue, etc.)
- Animate in real-time, advancing with the cycle clock

**Implementation:**
- Lip Gloss for styled rendering, layout, and color
- ntcharts (terminal charting library) for potential timeline rendering
- Harmonica for smooth scrolling/animation of the playhead
- The visualization needs pattern data — either by snooping outbound OSC (seeing what events are being sent) or by querying the pattern engine directly

**Complexity:** High. This is the most ambitious component and is deferred to Phase 3.

### 3.9 Pattern Tracing Cursor

Highlights the mini-notation fragment that is currently sounding, directly in the editor, as the pattern plays. Researched against the installed Tidal 1.10.3 source (`tidal` + `tidal-core` packages).

**Mechanism (all verified in Tidal source):** Tidal events already carry source positions. `Pattern.hs` defines `Context {contextPosition :: [((Int, Int), (Int, Int))]}`, populated by the mini-notation parser (`ParseBP.hs`, `wrapPos`) as 1-based `(column, line)` start/end pairs relative to the string literal. After the `deltaMini` rewrite, `deltaContext` adds the 0-based buffer column of the string literal's opening quote to the parser's 1-based in-string column, so the emitted **buffer columns are 0-based `[start, end)` indices while lines stay 1-based**; the TUI shifts columns to its 1-based convention at decode time. `Stream/Process.hs` (`toOSC` for `OSCContext`) can send these positions to a second OSC target: an `OSCContext "/editor/highlights"` shape emits, for each fragment position of each event, a message `[ident : string, delta : float, cycle : float, x, y, x', y' : int32]`, where `ident` is the pattern id injected by the stream (`"1"`..`"16"` for `d1`–`d16`; other pattern names such as `p "foo"` are ignored by the TUI). This is the same mechanism the official Pulsar plugin and the nvim Tidal plugins use for pattern highlighting (listener on `127.0.0.1:6013`).

**Message timing (verified):** the `OSCContext` branch of `toOSC` has **no onset guard** — unlike the SuperDirt branch, which only fires for events whose onset falls in the current tick arc (`peHasOnset`). Context messages are therefore re-sent on *every* clock tick for every event still intersecting the tick arc, arriving up to one `cProcessAhead` (~50 ms) before onset. Two consequences: (1) highlight lifetime should be refresh-based — a span stays while its messages keep arriving and is dropped after ~2 ticks of silence (at `cFrameTimespan` 1/50 that is ~80 ms) — this is exactly how Pulsar drives removal, and it keeps sustained events highlighted for their whole duration; (2) the `delta` argument is **not** usable as seconds: `peDelta` is raw `Link.Micros` (microseconds, Int64), and the SuperDirt path converts it with `Clock.addMicrosToOsc` while the `OSCContext` path passes it through unconverted.

**Buffer-absolute positions:** ParseBP positions are relative to the mini-notation string, not the editor buffer. Tidal ships the fix: `Pattern.hs` `deltaMini` rewrites source code by wrapping each string literal in `(deltaContext col line "...")`, shifting the contained event positions to buffer coordinates; it relies on `OverloadedStrings`, which BootTidal.hs already sets. Merged events accumulate every fragment's position (`combineContexts` concatenates positions), so `d1 $ s "bd sn" # n "0 1"` reports spans in both strings, one message per fragment.

**Implementation plan:**

1. **Custom boot script** (`boot/BootTidal-triton.hs`, used when the cursor is enabled): a copy of the upstream BootTidal.hs that uses `mkTidalWith` with two targets — SuperDirt unchanged, plus `editorTarget {oName = "charm-cursor", oAddress = "127.0.0.1", oPort = 6013, oLatency = 0.05, oWindow = Nothing, oSchedule = Pre BundleStamp, oHandshake = False, oBusPort = Nothing}` with `[OSCContext "/editor/highlights"]` — and a lowered `cFrameTimespan` (1/50) for responsive updates. The orphan `Tidally` instance is preserved so the boot aliases work. No extra imports are needed: `Sound.Tidal.Boot` re-exports `Sound.Tidal.Context`, which re-exports `Sound.Tidal.Stream` and with it `Target`, `OSC`, `OSCContext`, `superdirtTarget`, `superdirtShape`, and `defaultConfig`. Requires Tidal >= 1.9 (the `Sound.Tidal.Boot` / `mkTidalWith` API).
2. **Code rewriting at evaluation**: a Go port of `deltaMini` applied to each evaluated block/line (Ctrl-J / Ctrl-L) before writing it to GHCi, using the block's buffer coordinates as offsets. Received spans then point at the live buffer. Unit tests mirror cases from the Haskell implementation. Two wiring details: `extractBlock()` must additionally return the block's starting line (it currently returns only the code text), and the rewrite applies **only** to the text written to GHCi — `markEvaluated` and the edited-marker comparison must keep recording the original text, or the playing-vs-edited logic breaks.
3. **OSC listener** (`internal/cursor`): a go-osc `Server` on `127.0.0.1:6013` (configurable) with an `/editor/highlights` handler; decoded highlights are forwarded into the Bubble Tea program over a channel, the same way GHCi output is. Note go-osc decodes OSC floats as Go `float32`.
4. **Highlight state in the model**: a set of active spans `{stream, line, colStart, colEnd, lastSeenAt}` with refresh-based expiry (see message timing above): a periodic tick drops spans silent for ~2 clock frames. Spans are unioned when events overlap (polymeters). Spans whose positions no longer match the editor are suppressed: streams flagged *edited* (positions go stale once the buffer changes; the existing Tracker segment logic tracks this) and streams whose playing code is not in the editor at all (the pattern text was deleted while it keeps sounding). Highlights clear on hush and GHCi restart alongside the existing playing-state clears, and render only when the editor pane is visible (`editorVisible()`).
5. **Editor rendering**: vendor the bubbles textarea into `internal/textarea` (MIT; `textarea.go` + the `memoization` subpackage, keeping license headers) with two additions: an exported `YOffset` accessor and a highlight-span list rendered inside the existing `View()` wrap loop (so soft-wrapped spans style correctly), styled by a new theme style (reverse video in the stream's palette color). Post-processing `textarea.View()` was rejected: the scroll offset and wrap grid are unreachable through the public API.
6. **Configuration and theme**: a `[cursor]` config section (enabled, host, port) with env overrides; a boot-script override; a playing-highlight theme entry.

**Rejected alternatives:** snooping `/dirt/play` to SuperDirt (carries no source information), polling GHCi with `queryArc` (printed events contain no positions), and a Go mini-notation parser to recompute spans (duplicates Tidal semantics and cannot track combinators like `$`, `fast`). Tidal's `Context`/`OSCContext` is the purpose-built mechanism for exactly this feature.

**Risks:** port 6013 conflicts (configurable); transport expressions without string literals simply never highlight; stale spans after buffer edits (edited-flag suppression); packets arrive slightly early (cosmetic; matches Pulsar/nvim behavior); Tidal < 1.9 lacks `mkTidalWith` (config boot-script override plus a documented version check).

**Note:** README's config sample gains the `[cursor]` section when the feature ships.

### 3.10 File Document

Buffer persistence for `.tidal` files (plain Haskell text, normalized to match the editor buffer).

**Design:**
- `internal/doc.Doc` binds the buffer to a file: `Load` replaces the editor content and rebinds, `Save` writes the buffer (empty path = bound path) and rebinds only on success, `Dirty` compares the buffer with the saved copy. Paths expand `~` and bind absolute, so save-as is independent of later working-directory changes.
- Saves are atomic: content is staged in a temp file renamed over the target, so a failed save never truncates the previous file. Existing permissions are preserved and symlinks are followed; read-only targets still fail to save. `Load` normalizes line endings, tabs, and other control characters exactly like the editor's input sanitizer, so a freshly opened file never reads as unsaved.
- `Ctrl-S` saves; an unbound buffer opens a filename prompt prefilled with `untitled.tidal`. `Ctrl-O` opens the same prompt for a path to load. The prompt is a bordered overlay (`ui.Styles.Prompt` via `ui.Layout.Overlay`) with a bubbles `textinput`; while open it swallows all keys (`Enter` confirms, `Esc` cancels, `Ctrl-C` cancels into the quit confirmation) and the help bar switches to prompt keys. Save-as to an existing file asks before overwriting (warning overlay: `Enter` overwrites, `Esc` returns to the prompt, `Ctrl-C` cancels into the quit confirmation); opening over a dirty buffer asks before discarding the unsaved edits (same warning overlay: `Enter` opens, `Esc` returns to the prompt).
- The help bar shows the bound file's base name (or `untitled`), `*` marks unsaved changes, reusing the edited-pattern marker's visual language.
- The quit confirmation mentions unsaved changes when the buffer is dirty.
- Loading replaces the buffer without touching transport: playing patterns absent from the file clear their marker (like deleting them from the editor), and patterns whose text differs show as edited; highlight spans drop when positions no longer match. File errors surface in the console pane and leave the buffer unchanged.
- A file passed on the command line (`triton file.tidal`) loads before the TUI starts; failure exits with an error before any subprocess boots.

## 4. Keybindings

| Key | Action |
|---|---|
| `Ctrl-J` | Evaluate current block, send to GHCi |
| `Ctrl-L` | Evaluate current line, send to GHCi |
| `Ctrl-\` | Hush (stop all patterns) |
| `Alt-M` | Mute all |
| `Ctrl-U` | Unmute all |
| `F1`–`F8` | Toggle mute on d1–d8 |
| `Alt-1`–`Alt-8` | Toggle solo on d1–d8 |
| `Alt-[` / `Alt-]` | Decrease / increase tempo (cps) |
| `Tab` / `Shift-Tab` | Large: cycle focus between editor, status panel, and console. Small: swap between editor view and status+console view |
| `Ctrl-R` | Restart GHCi process |
| `Ctrl-S` | Save buffer to the current file (unsaved: prompts for a name; existing: asks to overwrite) |
| `Ctrl-O` | Open a `.tidal` file (type a path, `Enter` to load, `Esc` to cancel; unsaved changes ask first) |
| `Ctrl-C` | Quit: first press shows a warning, second press quits (cleanup all subprocesses), `Esc` cancels |

**Notes on keybinding choices:**

- `Ctrl-Enter` and `Shift-Enter` from the original design are not available as distinct keys in most terminals. `Ctrl-J` (line feed) and `Ctrl-L` are used instead.
- `Ctrl-M` is not a distinct key either: it is carriage return, which bubbletea reports as `enter`. Mute-all uses `Alt-M` instead.
- `Ctrl-.` is not a distinct key in bubbletea. `Ctrl-\` is used for hush.
- `[` / `]` alone conflict with text editing. `Alt-[` / `Alt-]` are used for tempo control.
- `Shift-F1`–`Shift-F8` are not delivered reliably by terminals (bubbletea defines no shift-F key types), so solo toggles use `Alt-1`–`Alt-8`.
- `Ctrl-S` (save) and `Ctrl-O` (open) are implemented (see 3.10). Up/down history navigation remains planned for Phase 2.

**Quit confirmation:** `Ctrl-C` opens a centered modal (`ui.Layout.Overlay` + `ui.Styles.Warning`) instead of quitting immediately. A second `Ctrl-C` (or a SIGINT/`InterruptMsg` when input is not a TTY) quits; `Esc` dismisses the modal. While the modal is open, all other key input is swallowed. The overlay composites the modal over the rendered panes at exact cell boundaries, preserving line widths and the underlying border/text colors. The warning mentions unsaved changes when the buffer differs from its file (see 3.10).

## 5. Configuration

Configuration via a TOML file (`~/.config/triton/config.toml`):

```toml
[ghci]
executable = "ghci"              # Path to GHCi
boot_script = "/path/to/BootTidal.hs"  # Path to BootTidal.hs
working_dir = "."                # Working directory for GHCi

[osc]
tidal_control_host = "127.0.0.1"
tidal_control_port = 6010        # Tidal's inbound OSC port

[superdirt]
executable = "sclang"            # Path to sclang
boot_script = "start-superdirt.scd"  # SuperDirt boot script
auto_start = true                # Start SuperDirt automatically on launch

[ui]
theme = "dark"                   # dark | light
default_tempo = 0.6              # Default cps
visible_streams = 8              # How many d-streams to show (1-16)

[cursor]
enabled = true                   # Pattern tracing cursor in the editor
host = "127.0.0.1"               # Listener for Tidal's /editor/highlights
port = 6013
```

Environment variables override config file values with prefix `TIDAL_CHARM_`.

## 6. Project Structure

```
triton/
├── DESIGN.md
├── README.md
├── AGENTS.md
├── go.mod / go.sum
├── main.go                          # Entry point: config → superdirt → ghci → model → tea.Program
├── triton.cabal                # Haskell project (Tidal dependency)
├── Repl.hs                          # Dummy Haskell module for cabal
├── start-superdirt.scd              # SuperCollider boot script
├── install-superdirt.scd            # SuperDirt installer
├── internal/
│   ├── config/
│   │   └── config.go                # Configuration loading (TOML + env overrides)
│   ├── doc/
│   │   └── doc.go                   # Editor document: .tidal load/save, dirty state
│   ├── ghci/
│   │   ├── process.go               # GHCi subprocess management
│   │   └── process_test.go
│   ├── osc/
│   │   ├── client.go                # OSC client for Tidal control messages
│   │   └── client_test.go
│   ├── superdirt/
│   │   └── process.go               # SuperCollider/SuperDirt subprocess management
│   ├── model/
│   │   ├── model.go                 # Top-level Bubble Tea model + keybindings
│   │   └── model_test.go            # Tests for extractBlock()
│   ├── status/
│   │   ├── panel.go                 # Status panel renderer
│   │   ├── tracker.go               # Manual state tracking
│   │   └── tracker_test.go
│   └── (viz/                         # Phase 2/3: timeline, snooper)
├── ui/
│   ├── layout.go                    # Layout management (split panes, modal overlay)
│   ├── theme.go                     # Color scheme, palette, themes
│   └── styles.go                    # Lip Gloss style definitions (incl. warning modal)
└── examples/
    ├── basic.tidal                  # Example Tidal patterns
    └── advanced.tidal
```

## 7. Implementation Phases

### Phase 1: Minimum Viable Product (COMPLETE)

Goal: Write and evaluate Tidal code from the terminal, with transport controls.

- [x] GHCi process manager (spawn, pipe stdin, read stdout/stderr for errors)
- [x] OSC client (hush, mute/unmute, muteAll/unmuteAll)
- [x] Code editor (Bubbles textarea, Ctrl-J to evaluate block, Ctrl-L for single line)
- [x] Basic layout (editor + status panel with output log)
- [x] Configuration file loading (TOML + env overrides)
- [x] GHCi error display (stderr forwarded to output panel, stdout vs stderr distinguished)
- [x] Process lifecycle management (restart via Ctrl-R with epoch invalidation, cleanup on exit)
- [x] SuperDirt auto-start (sclang subprocess, ready detection, cleanup on exit)
- [x] Block evaluation (extractBlock finds contiguous non-empty lines around cursor)
- [x] Multi-line GHCi evaluation (`:{` / `:}` delimiters)
- [x] Status panel (stream states, cps, ghci status)
- [x] Tempo control (Alt-[ / Alt-] to adjust cps via OSC /setcps)

**Deliverable:** A functional TUI where you can write `d1 $ sound "bd sn"` and hear it play, press a key to hush, and see GHCi errors inline.

### Phase 2: Status and Transport

Goal: Visual feedback on active patterns and richer transport control.

- [x] Solo toggles (Alt-1–Alt-8)
- [ ] Evaluation history (up/down to recall previously evaluated blocks)
- [x] File open/save (.tidal files, see 3.10)
- [ ] OSC snooping (read-only tap on port 57120 for real-time event data)
- [ ] Cycle count display (derived from snooped OSC messages)
- [x] Shift-Tab to reverse-focus cycle
- [x] Playing vs edited distinction (`*` marker on streams with unevaluated edits)
- [x] Pattern tracing cursor (OSCContext listener, `deltaMini` code rewriting, editor span highlighting — see 3.9)

**Deliverable:** A TUI that shows which patterns are playing, lets you mute/solo individual streams, and recalls evaluation history.

### Phase 3: Visualization

Goal: Real-time pattern visualization.

- [ ] Timeline renderer (one cycle, events as colored blocks, playhead animation)
- [ ] Multi-stream overlay (show d1–d8 events on a shared timeline)
- [ ] Harmonica-based smooth animation for playhead
- [ ] Optional: custom BootTidal.hs hook for richer status OSC messages
- [ ] Syntax highlighting for Tidal mini-notation and Haskell

**Deliverable:** A TUI with a live animated view of pattern events as they cycle.

## 8. Dependencies

### Go libraries

| Library | Purpose | Version |
|---|---|---|
| `github.com/charmbracelet/bubbletea` | TUI framework | latest |
| `github.com/charmbracelet/lipgloss` | Styling and layout | latest |
| `github.com/charmbracelet/bubbles` | textarea component | latest |
| `github.com/charmbracelet/harmonica` | Spring animation (phase 3) | latest |
| `github.com/hypebeast/go-osc` | OSC client for Tidal control | latest |
| `github.com/BurntSushi/toml` | Configuration parsing | latest |

### External dependencies (user's system)

| Dependency | Purpose | Required? |
|---|---|---|
| GHC / GHCi | Haskell runtime for Tidal | Yes |
| cabal | Haskell package manager (for Tidal install) | Yes |
| Tidal Cycles | Pattern engine (Haskell library) | Yes |
| SuperCollider / sclang | Audio synthesis | Yes |
| SuperDirt | SuperCollider quark for Tidal | Yes |
| BootTidal.hs | Tidal boot script | Yes (bundled with Tidal) |

## 9. Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| GHCi stdout parsing is unreliable | Cannot display meaningful errors | Display raw GHCi stderr; don't attempt to parse beyond "error" detection |
| Tidal's OSC control API is limited | No rich state feedback | Start with manual tracking; add OSC snooping; offer custom BootTidal hook as opt-in |
| Bubbles textarea lacks syntax highlighting | Poor editing experience | Custom highlighter overlay; or accept plain editing for phase 1 |
| GHCi process crashes | TUI becomes non-functional | Auto-detect process exit, display error, offer restart key (Ctrl-R) with epoch counter to invalidate stale goroutines |
| OSC snooping requires raw socket access | May need elevated permissions | Make snooping optional; default to manual tracking |
| Multi-line Tidal code evaluation is tricky | Blocks not evaluated correctly | Use `:{` ... `:}` delimiters; extractBlock() finds contiguous non-empty lines around cursor |
| SuperDirt startup is slow | User waits on launch | 30-second timeout with warning if not ready; continue anyway |
| Concurrent channel access in subprocess managers | Double-close panics, goroutine leaks | Mutex-protected close, WaitGroup for reader goroutines, epoch counter for restart invalidation |

## 10. Non-Goals

- **Not a replacement for the Strudel/Tidal web REPL.** This is a controller for an existing Tidal setup, not a self-contained environment.
- **Not a sound engine.** All audio is handled by SuperCollider/SuperDirt.
- **Not a Haskell IDE.** The editor is optimized for Tidal pattern expressions, not general Haskell development.
- **Not a pattern sequencer GUI.** This is a code-first live-coding tool, not a grid-based step sequencer.
- **No Strudel support in v1.** Strudel's browser-based architecture makes it a poor fit for terminal control. Could be revisited if Strudel adds an inbound control API.

## 11. Open Questions

1. **OSC snooping legality**: Can we reliably snoop on UDP port 57120 without interfering with SuperDirt? We'd need a separate socket bound to the same port (not possible on UDP) or use a packet capture library. Alternative: configure Tidal to send to a local proxy that forwards to SuperDirt and copies to the TUI. This needs investigation.

2. **Custom BootTidal hook**: What minimal Haskell additions would give us rich status reporting? Would need to add OSC send calls in the pattern update hooks. This modifies the user's Tidal setup, so it should be opt-in and non-destructive.

3. **Syntax highlighting scope**: Should we highlight just the mini-notation (`"bd sn ~ cp"`) or also Haskell operators (`$`, `#`, `|+|`, `<$>`)? A full Haskell highlighter in Go doesn't exist; we'd need a custom lightweight tokenizer.

4. **Local vs global Tidal install**: The current setup uses `cabal install --lib tidal --package-env .` to create a `.ghc.environment` file in the project directory. This means GHCi must be started from that directory. Should we support a global install fallback for users who prefer `ghc-pkg register`?
