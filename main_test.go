package main

import (
	"os"
	"testing"

	"github.com/pshickeydev/triton/internal/boot"
	"github.com/pshickeydev/triton/internal/config"
)

// missingSuperDirtBootDep is the dependency name cfg.MissingDeps reports
// for the SuperDirt boot script.
const missingSuperDirtBootDep = "SuperDirt boot script"

func TestResolveBootScriptsBeforeDeps(t *testing.T) {
	// Simulate an installed binary running outside the project checkout:
	// the default start-superdirt.scd must not exist in the current
	// directory, so only the embedded fallback can satisfy the check.
	t.Chdir(t.TempDir())

	cfg := &config.Config{
		GHCi:      config.GHCiConfig{Executable: "ghci", WorkingDir: "."},
		SuperDirt: config.SuperDirtConfig{AutoStart: true, Executable: "sclang", BootScript: config.DefaultSuperDirtBootScript},
	}
	if _, err := os.Stat(config.DefaultSuperDirtBootScript); err == nil {
		t.Fatalf("%s unexpectedly exists in the test directory", config.DefaultSuperDirtBootScript)
	}

	scripts, err := resolveBootScripts(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer scripts.cleanup()

	if cfg.SuperDirt.BootScript == config.DefaultSuperDirtBootScript {
		t.Fatalf("superdirt boot script = %q, want the embedded fallback path", cfg.SuperDirt.BootScript)
	}
	if _, err := os.Stat(cfg.SuperDirt.BootScript); err != nil {
		t.Fatalf("resolved superdirt boot script %q is missing: %v", cfg.SuperDirt.BootScript, err)
	}
	for _, dep := range cfg.MissingDeps() {
		if dep.Name == missingSuperDirtBootDep {
			t.Errorf("MissingDeps() reports %q after the embedded fallback was resolved", dep.Name)
		}
	}
}

func TestResolveBootScriptsKeepsOverrides(t *testing.T) {
	override := "my-boot.hs"
	cfg := &config.Config{
		GHCi:      config.GHCiConfig{Executable: "ghci", BootScript: override, WorkingDir: "."},
		SuperDirt: config.SuperDirtConfig{AutoStart: false, BootScript: "unused.scd"},
		Cursor:    config.CursorConfig{Enabled: true, BootScript: override, Host: "127.0.0.1", Port: 6013},
	}

	scripts, err := resolveBootScripts(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer scripts.cleanup()

	if scripts.ghci != override {
		t.Errorf("cursor boot script = %q, want override %q", scripts.ghci, override)
	}
	if cfg.SuperDirt.BootScript != "unused.scd" {
		t.Errorf("superdirt boot script = %q, want override untouched", cfg.SuperDirt.BootScript)
	}
}

func TestBundledCursorScriptCarriesEditorTarget(t *testing.T) {
	_, cleanup, err := boot.Cursor("", cursorBootScript, "127.0.0.1", 6013)
	if err != nil {
		t.Fatal(err)
	}
	cleanup()
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
