package config

import (
	"os"
	"path/filepath"
	"testing"
)

func testConfig() Config {
	return Config{
		GHCi: GHCiConfig{
			Executable: "ghci",
			BootScript: "BootTidal.hs",
			WorkingDir: ".",
		},
		SuperDirt: SuperDirtConfig{
			Executable: "sclang",
			BootScript: "start-superdirt.scd",
			AutoStart:  true,
		},
	}
}

func depNames(deps []Dependency) []string {
	names := make([]string, len(deps))
	for i, dep := range deps {
		names[i] = dep.Name
	}
	return names
}

func TestMissingDepsAllPresent(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "tool")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	script := filepath.Join(dir, "boot.script")
	if err := os.WriteFile(script, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()
	cfg.GHCi.Executable = "tool"
	cfg.GHCi.BootScript = script
	cfg.SuperDirt.Executable = "tool"
	cfg.SuperDirt.BootScript = script

	if missing := cfg.MissingDeps(); len(missing) != 0 {
		t.Errorf("MissingDeps() = %v, want none", depNames(missing))
	}
}

func TestMissingDepsExecutable(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "boot.script")
	if err := os.WriteFile(script, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := testConfig()
	cfg.GHCi.Executable = "triton-nonexistent-executable"
	cfg.GHCi.BootScript = script
	cfg.SuperDirt.AutoStart = false

	missing := cfg.MissingDeps()
	if len(missing) != 1 || missing[0].Name != "GHCi" {
		t.Errorf("MissingDeps() = %v, want [GHCi]", depNames(missing))
	}
}

func TestMissingDepsBootScript(t *testing.T) {
	cfg := testConfig()
	cfg.GHCi.Executable = "go"
	cfg.GHCi.BootScript = filepath.Join(t.TempDir(), "nonexistent.hs")
	cfg.SuperDirt.AutoStart = false

	missing := cfg.MissingDeps()
	if len(missing) != 1 || missing[0].Name != "Tidal boot script" {
		t.Errorf("MissingDeps() = %v, want [Tidal boot script]", depNames(missing))
	}
}

func TestMissingDepsSkipsSuperDirtWhenDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.GHCi.Executable = "go"
	cfg.GHCi.BootScript = "config.go"
	cfg.SuperDirt.Executable = "triton-nonexistent-executable"
	cfg.SuperDirt.AutoStart = false

	if missing := cfg.MissingDeps(); len(missing) != 0 {
		t.Errorf("MissingDeps() = %v, want none when auto_start is false", depNames(missing))
	}
}

func TestMissingDepsSuperDirtWhenEnabled(t *testing.T) {
	cfg := testConfig()
	cfg.GHCi.Executable = "go"
	cfg.GHCi.BootScript = "config.go"
	cfg.SuperDirt.Executable = "triton-nonexistent-executable"
	cfg.SuperDirt.BootScript = "config.go"
	cfg.SuperDirt.AutoStart = true

	missing := cfg.MissingDeps()
	if len(missing) != 1 || missing[0].Name != "sclang" {
		t.Errorf("MissingDeps() = %v, want [sclang]", depNames(missing))
	}
}

func TestMissingDepsDirectoryIsNotExecutable(t *testing.T) {
	cfg := testConfig()
	cfg.GHCi.Executable = t.TempDir()
	cfg.GHCi.BootScript = "config.go"
	cfg.SuperDirt.AutoStart = false

	missing := cfg.MissingDeps()
	if len(missing) != 1 || missing[0].Name != "GHCi" {
		t.Errorf("MissingDeps() = %v, want [GHCi]", depNames(missing))
	}
}

func TestMissingDepsWorkingDir(t *testing.T) {
	cfg := testConfig()
	cfg.GHCi.Executable = "go"
	cfg.GHCi.BootScript = "config.go"
	cfg.GHCi.WorkingDir = filepath.Join(t.TempDir(), "nonexistent")
	cfg.SuperDirt.AutoStart = false

	missing := cfg.MissingDeps()
	if len(missing) != 1 || missing[0].Name != "GHCi working directory" {
		t.Errorf("MissingDeps() = %v, want [GHCi working directory]", depNames(missing))
	}
}

func TestLoadAppliesEnvOverridesWithoutConfigFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TIDAL_CHARM_GHCI_EXECUTABLE", "custom-ghci")
	t.Setenv("TIDAL_CHARM_SUPERDIRT_AUTO_START", "false")
	t.Setenv("TIDAL_CHARM_CURSOR_ENABLED", "false")
	t.Setenv("TIDAL_CHARM_CURSOR_HOST", "127.0.0.2")
	t.Setenv("TIDAL_CHARM_CURSOR_PORT", "7013")
	t.Setenv("TIDAL_CHARM_CURSOR_BOOT_SCRIPT", "/tmp/custom-boot.hs")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GHCi.Executable != "custom-ghci" {
		t.Errorf("GHCi.Executable = %q, want %q", cfg.GHCi.Executable, "custom-ghci")
	}
	if cfg.SuperDirt.AutoStart {
		t.Error("SuperDirt.AutoStart = true, want false")
	}
	if cfg.Cursor.Enabled {
		t.Error("Cursor.Enabled = true, want false")
	}
	if cfg.Cursor.Host != "127.0.0.2" {
		t.Errorf("Cursor.Host = %q, want %q", cfg.Cursor.Host, "127.0.0.2")
	}
	if cfg.Cursor.Port != 7013 {
		t.Errorf("Cursor.Port = %d, want %d", cfg.Cursor.Port, 7013)
	}
	if cfg.Cursor.BootScript != "/tmp/custom-boot.hs" {
		t.Errorf("Cursor.BootScript = %q, want %q", cfg.Cursor.BootScript, "/tmp/custom-boot.hs")
	}
}

func TestMissingDepsSkipsBootScriptWhenCursorBundled(t *testing.T) {
	cfg := testConfig()
	cfg.GHCi.Executable = "go"
	cfg.GHCi.BootScript = ""
	cfg.SuperDirt.AutoStart = false
	cfg.Cursor.Enabled = true

	if missing := cfg.MissingDeps(); len(missing) != 0 {
		t.Errorf("MissingDeps() = %v, want none: bundled cursor boot script needs no ghci.boot_script", depNames(missing))
	}
}

func TestMissingDepsEmptyBootScriptWhenCursorDisabled(t *testing.T) {
	cfg := testConfig()
	cfg.GHCi.Executable = "go"
	cfg.GHCi.BootScript = ""
	cfg.SuperDirt.AutoStart = false

	missing := cfg.MissingDeps()
	if len(missing) != 1 || missing[0].Name != "Tidal boot script" {
		t.Errorf("MissingDeps() = %v, want [Tidal boot script]", depNames(missing))
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".config", "triton")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.toml"), []byte("[ui]\nvisable_streams = 8\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(); err == nil {
		t.Error("Load() = nil error, want unknown-key error")
	}
}

func TestLoadValidatesValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TIDAL_CHARM_UI_VISIBLE_STREAMS", "99")

	if _, err := Load(); err == nil {
		t.Error("Load() = nil error, want visible_streams range error")
	}
}

func TestLoadAppliesVisibleStreamsEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("TIDAL_CHARM_UI_VISIBLE_STREAMS", "12")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.UI.VisibleStreams != 12 {
		t.Errorf("UI.VisibleStreams = %d, want 12", cfg.UI.VisibleStreams)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	tests := []struct{ in, want string }{
		{"~", home},
		{"~/x", filepath.Join(home, "x")},
		{"~other/x", "~other/x"},
		{"/abs/path", "/abs/path"},
		{"rel/path", "rel/path"},
	}
	for _, tt := range tests {
		if got := expandHome(tt.in); got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
