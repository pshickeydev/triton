package boot

import (
	"os"
	"strings"
	"testing"

	"github.com/pshickeydev/triton/internal/config"
)

const cursorScript = `tidal <- startTidal superdirtTarget {oAddress = "127.0.0.1", oPort = 57120}
let editor = Target {oName = "charm-cursor", oAddress = "127.0.0.1", oPort = 6013, oLatency = 0.05}
`

const superdirtScript = `Server.default.waitForBoot {
  ~dirt.start(57120, 0 ! 12);
  postln "SuperDirt ready";
};
`

func TestCursorOverride(t *testing.T) {
	path, cleanup, err := Cursor("my-boot.hs", cursorScript, "127.0.0.1", 7013)
	if err != nil {
		t.Fatal(err)
	}
	if path != "my-boot.hs" {
		t.Errorf("override path: got %q, want %q", path, "my-boot.hs")
	}
	cleanup()
}

func TestCursorSubstitutesHostPort(t *testing.T) {
	path, cleanup, err := Cursor("", cursorScript, "127.0.0.2", 7013)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	script, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), `oAddress = "127.0.0.2", oPort = 7013`) {
		t.Errorf("temp boot script should carry the configured host and port")
	}
	if strings.Contains(string(script), `oPort = 6013`) {
		t.Errorf("temp boot script should not keep the default editor port")
	}
	if !strings.Contains(string(script), "oPort = 57120") {
		t.Errorf("SuperDirt target port should be untouched")
	}
}

func TestCursorDefaultsUnchanged(t *testing.T) {
	path, cleanup, err := Cursor("", cursorScript, "127.0.0.1", 6013)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	script, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(script) != cursorScript {
		t.Error("default host and port should keep the bundled script verbatim")
	}
}

func TestCursorMissingEditorTarget(t *testing.T) {
	_, _, err := Cursor("", "let x = 1\n", "127.0.0.1", 6013)
	if err == nil {
		t.Fatal("expected error for a script lacking the editor target address")
	}
}

func TestSuperDirtExisting(t *testing.T) {
	dir := t.TempDir()
	script := dir + "/custom.scd"
	if err := os.WriteFile(script, []byte("~dirt.start"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := SuperDirt(script, superdirtScript)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if path != script {
		t.Errorf("existing script: got %q, want %q", path, script)
	}
}

func TestSuperDirtBundledFallback(t *testing.T) {
	t.Chdir(t.TempDir())
	path, cleanup, err := SuperDirt(config.DefaultSuperDirtBootScript, superdirtScript)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	script, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(script), "SuperDirt ready") {
		t.Error("fallback script should be the bundled start-superdirt.scd")
	}
}

func TestSuperDirtMissingCustom(t *testing.T) {
	t.Chdir(t.TempDir())
	path, cleanup, err := SuperDirt("missing.scd", superdirtScript)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if path != "missing.scd" {
		t.Errorf("missing custom script: got %q, want it unchanged for the dependency check", path)
	}
}
