package main

import (
	"os"
	"strings"
	"testing"

	"github.com/pshickeydev/triton/internal/config"
)

func TestCursorBootPathOverride(t *testing.T) {
	path, cleanup, err := cursorBootPath("my-boot.hs", "127.0.0.1", 7013)
	if err != nil {
		t.Fatal(err)
	}
	if path != "my-boot.hs" {
		t.Errorf("override path: got %q, want %q", path, "my-boot.hs")
	}
	cleanup()
}

func TestCursorBootPathSubstitutesHostPort(t *testing.T) {
	path, cleanup, err := cursorBootPath("", "127.0.0.2", 7013)
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

func TestCursorBootPathDefaultsUnchanged(t *testing.T) {
	path, cleanup, err := cursorBootPath("", "127.0.0.1", 6013)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	script, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(script) != cursorBootScript {
		t.Error("default host and port should keep the bundled script verbatim")
	}
}

func TestSuperdirtBootPathExisting(t *testing.T) {
	dir := t.TempDir()
	script := dir + "/custom.scd"
	if err := os.WriteFile(script, []byte("~dirt.start"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, cleanup, err := superdirtBootPath(script)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if path != script {
		t.Errorf("existing script: got %q, want %q", path, script)
	}
}

func TestSuperdirtBootPathBundledFallback(t *testing.T) {
	t.Chdir(t.TempDir())
	path, cleanup, err := superdirtBootPath(config.DefaultSuperDirtBootScript)
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

func TestSuperdirtBootPathMissingCustom(t *testing.T) {
	t.Chdir(t.TempDir())
	path, cleanup, err := superdirtBootPath("missing.scd")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if path != "missing.scd" {
		t.Errorf("missing custom script: got %q, want it unchanged for the dependency check", path)
	}
}

func TestParseArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    string
		wantErr bool
	}{
		{name: "no args", args: []string{"triton"}, want: ""},
		{name: "file", args: []string{"triton", "set.tidal"}, want: "set.tidal"},
		{name: "flag", args: []string{"triton", "-x"}, wantErr: true},
		{name: "extra", args: []string{"triton", "a", "b"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseArgs(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseArgs(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("parseArgs(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}
