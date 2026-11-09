// Package e2e drives the built triton binary in steep's emulated terminal.
//
// steep's in-process harnesses require Bubble Tea v2, so these tests use
// the exec harness, which runs any command on a pseudo-terminal. Each
// test starts triton against a stub GHCi: a shell loop that echoes what
// it receives, standing in for a real session so the suite needs neither
// GHC, SuperDirt, nor audio.
package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/lrstanley/steep"
)

const (
	screenWidth  = 120
	screenHeight = 36
	smallWidth   = 84
	smallHeight  = 24
	tinyWidth    = 48
	tinyHeight   = 14

	panelTitleText    = " triton "
	consoleTitleText  = " console "
	placeholderOnView = `d1 $ sound "bd sn"`
	markerHintOnView  = "* marks a pattern"
	quitWarningView   = "Press Ctrl-C again to quit"
	tooSmallView      = "Terminal too small (minimum 60x20)"
	echoPrefix        = "stub>"
	evalProbe         = `d1 $ sound "z9"`

	// exitTimeout covers the stub quitting after the final Ctrl-C.
	exitTimeout = 20 * time.Second
)

// stubGhci mimics the GHCi session triton drives: every line it reads is
// echoed back with a marker so the console shows what was sent, and it
// exits when triton sends the quit command. GHCi meta-commands are
// swallowed: their echo would carry temp-directory paths into snapshots.
const stubGhci = `#!/bin/sh
while IFS= read -r line; do
	case "$line" in
		:quit) exit 0 ;;
		:*) continue ;;
		*) printf 'stub> %s\n' "$line" ;;
	esac
done
`

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "triton-e2e")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	binPath = filepath.Join(dir, "triton")
	if out, err := exec.Command("go", "build", "-o", binPath, "..").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build: %v\n%s", err, out)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// newTriton starts the binary with HOME pointed at an empty directory so
// the developer's own config cannot leak in, and every dependency either
// stubbed or switched off through the TIDAL_CHARM_* overrides.
func newTriton(t *testing.T, width, height int) *steep.Harness {
	t.Helper()

	dir := t.TempDir()
	home := filepath.Join(dir, "home")
	boot := filepath.Join(dir, "boot.hs")
	stub := filepath.Join(dir, "fake-ghci")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("creating %s: %v", home, err)
	}
	if err := os.WriteFile(boot, []byte("-- stub boot script\n"), 0o644); err != nil {
		t.Fatalf("writing boot stub: %v", err)
	}
	if err := os.WriteFile(stub, []byte(stubGhci), 0o755); err != nil {
		t.Fatalf("writing ghci stub: %v", err)
	}

	cmd := exec.Command(binPath)
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + home,
		"TERM=xterm-256color",
		"TIDAL_CHARM_GHCI_EXECUTABLE=" + stub,
		"TIDAL_CHARM_GHCI_BOOT_SCRIPT=" + boot,
		"TIDAL_CHARM_GHCI_WORKING_DIR=" + dir,
		"TIDAL_CHARM_SUPERDIRT_AUTO_START=false",
		"TIDAL_CHARM_CURSOR_ENABLED=false",
	}
	return steep.NewExecHarness(t, cmd, steep.WithWindowSize(width, height))
}

func TestStartsQuitsCleanly(t *testing.T) {
	h := newTriton(t, screenWidth, screenHeight)
	h.Wait([]string{panelTitleText, consoleTitleText}).
		Assert(markerHintOnView).
		Assert("Ctrl-J: eval block")

	h.KeyCtrlC().
		Wait(quitWarningView).
		KeyCtrlC().
		WaitFinished()
	if status := h.ExitStatus(steep.WithTimeout(exitTimeout)); status != 0 {
		t.Errorf("exit status = %d, want 0", status)
	}
}

func TestStartupSnapshot(t *testing.T) {
	newTriton(t, screenWidth, screenHeight).
		Wait(panelTitleText).
		Wait(echoPrefix + " setcps").
		AssertSnapshot()
}

func TestTypingReplacesPlaceholder(t *testing.T) {
	newTriton(t, screenWidth, screenHeight).
		Wait(panelTitleText).
		Type(evalProbe).
		Wait(evalProbe).
		WaitNot(placeholderOnView)
}

func TestEvalReachesGhciAndEchoesBack(t *testing.T) {
	// The typed text is already on screen in the editor; only the stub's
	// echoed line proves the keystroke traveled through to GHCi stdin
	// and the output came back into the console.
	newTriton(t, screenWidth, screenHeight).
		Wait(panelTitleText).
		Type(evalProbe).
		Key("ctrl+j").
		Wait(echoPrefix + " " + evalProbe)
}

func TestQuitWarningCancelsOnEsc(t *testing.T) {
	newTriton(t, screenWidth, screenHeight).
		Wait(panelTitleText).
		KeyCtrlC().
		Wait(quitWarningView).
		KeyEsc().
		WaitNot(quitWarningView)
}

func TestResizeSwitchesLayoutMode(t *testing.T) {
	h := newTriton(t, screenWidth, screenHeight)
	h.Wait([]string{panelTitleText, consoleTitleText})

	// The small layout shows one pane at a time: the focused editor, with
	// the status and console hidden until Tab switches to them. Waits are
	// used at every frame transition, since a bare Assert can catch the
	// screen mid-redraw after a resize.
	h.Resize(smallWidth, smallHeight).
		WaitNot(panelTitleText).
		Wait(placeholderOnView).
		Key("tab").
		Wait([]string{panelTitleText, consoleTitleText}).
		AssertNot(placeholderOnView)

	// Back to the wide layout: the combined view gives way to the editor
	// rendered beside the status panes.
	h.Resize(screenWidth, screenHeight).
		Wait(placeholderOnView).
		Assert([]string{panelTitleText, consoleTitleText})
}

func TestTooSmallTerminalShowsNotice(t *testing.T) {
	newTriton(t, tinyWidth, tinyHeight).
		Wait(tooSmallView)
}
