package config

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/pshickeydev/triton/internal/tidal"
)

type Config struct {
	GHCi      GHCiConfig      `toml:"ghci"`
	OSC       OSCConfig       `toml:"osc"`
	UI        UIConfig        `toml:"ui"`
	SuperDirt SuperDirtConfig `toml:"superdirt"`
	Cursor    CursorConfig    `toml:"cursor"`
}

type GHCiConfig struct {
	Executable string `toml:"executable"`
	BootScript string `toml:"boot_script"`
	WorkingDir string `toml:"working_dir"`
}

type OSCConfig struct {
	TidalControlHost string `toml:"tidal_control_host"`
	TidalControlPort int    `toml:"tidal_control_port"`
}

type UIConfig struct {
	Theme          string  `toml:"theme"`
	DefaultTempo   float64 `toml:"default_tempo"`
	VisibleStreams int     `toml:"visible_streams"`
}

type SuperDirtConfig struct {
	Executable string `toml:"executable"`
	BootScript string `toml:"boot_script"`
	AutoStart  bool   `toml:"auto_start"`
}

// CursorConfig configures the pattern tracing cursor: the listener for
// Tidal's OSCContext messages and the boot script that enables them.
// BootScript overrides the bundled BootTidal-triton.hs; leave it empty to
// use the bundled copy. It requires Tidal >= 1.9.
type CursorConfig struct {
	Enabled    bool   `toml:"enabled"`
	Host       string `toml:"host"`
	Port       int    `toml:"port"`
	BootScript string `toml:"boot_script"`
}

func Default() Config {
	return Config{
		GHCi: GHCiConfig{
			Executable: "ghci",
			BootScript: "",
			WorkingDir: ".",
		},
		OSC: OSCConfig{
			TidalControlHost: "127.0.0.1",
			TidalControlPort: 6010,
		},
		UI: UIConfig{
			Theme:          "dark",
			DefaultTempo:   0.6,
			VisibleStreams: 8,
		},
		SuperDirt: SuperDirtConfig{
			Executable: "sclang",
			BootScript: "start-superdirt.scd",
			AutoStart:  true,
		},
		Cursor: CursorConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    6013,
		},
	}
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "triton", "config.toml"), nil
}

func Load() (*Config, error) {
	cfg := Default()

	path, err := configPath()
	if err == nil {
		if _, err := os.Stat(path); err == nil {
			md, err := toml.DecodeFile(path, &cfg)
			if err != nil {
				return nil, fmt.Errorf("decoding config: %w", err)
			}
			if unknown := md.Undecoded(); len(unknown) > 0 {
				return nil, fmt.Errorf("unknown config keys: %v", unknown)
			}
		}
	}

	applyEnvOverrides(&cfg)
	cfg.GHCi.BootScript = expandHome(cfg.GHCi.BootScript)
	cfg.GHCi.WorkingDir = expandHome(cfg.GHCi.WorkingDir)
	cfg.SuperDirt.BootScript = expandHome(cfg.SuperDirt.BootScript)
	cfg.Cursor.BootScript = expandHome(cfg.Cursor.BootScript)

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

const (
	minPort = 1
	maxPort = 65535
)

func (c *Config) validate() error {
	if c.UI.VisibleStreams < 1 || c.UI.VisibleStreams > tidal.MaxStream {
		return fmt.Errorf("ui.visible_streams must be 1-%d, got %d", tidal.MaxStream, c.UI.VisibleStreams)
	}
	if c.UI.DefaultTempo <= 0 {
		return fmt.Errorf("ui.default_tempo must be positive, got %g", c.UI.DefaultTempo)
	}
	if c.OSC.TidalControlPort < minPort || c.OSC.TidalControlPort > maxPort {
		return fmt.Errorf("osc.tidal_control_port must be %d-%d, got %d", minPort, maxPort, c.OSC.TidalControlPort)
	}
	if c.Cursor.Enabled && (c.Cursor.Port < minPort || c.Cursor.Port > maxPort) {
		return fmt.Errorf("cursor.port must be %d-%d, got %d", minPort, maxPort, c.Cursor.Port)
	}
	return nil
}

type depKind int

const (
	depExecutable depKind = iota
	depFile
	depDir
)

type Dependency struct {
	Name string
	Path string
	Hint string
	kind depKind
}

func (d Dependency) Missing() bool {
	if d.kind == depExecutable {
		_, err := exec.LookPath(d.Path)
		return err != nil
	}
	info, err := os.Stat(d.Path)
	if err != nil {
		return true
	}
	if d.kind == depDir {
		return !info.IsDir()
	}
	return info.IsDir()
}

func (c *Config) MissingDeps() []Dependency {
	deps := []Dependency{
		{Name: "GHCi", Path: c.GHCi.Executable, Hint: "install GHC (e.g. via ghcup)", kind: depExecutable},
		{Name: "GHCi working directory", Path: c.GHCi.WorkingDir, Hint: "set ghci.working_dir to an existing directory", kind: depDir},
	}
	if !c.Cursor.Enabled {
		deps = append(deps,
			Dependency{Name: "Tidal boot script", Path: c.GHCi.BootScript, Hint: "set ghci.boot_script to your BootTidal.hs (see README)", kind: depFile},
		)
	}
	if c.SuperDirt.AutoStart {
		deps = append(deps,
			Dependency{Name: "sclang", Path: c.SuperDirt.Executable, Hint: "install SuperCollider", kind: depExecutable},
			Dependency{Name: "SuperDirt boot script", Path: c.SuperDirt.BootScript, Hint: "run: sclang install-superdirt.scd", kind: depFile},
		)
	}
	if c.Cursor.Enabled && c.Cursor.BootScript != "" {
		deps = append(deps,
			Dependency{Name: "Cursor boot script", Path: c.Cursor.BootScript, Hint: "set cursor.boot_script to an existing BootTidal.hs copy (Tidal >= 1.9)", kind: depFile},
		)
	}

	var missing []Dependency
	for _, dep := range deps {
		if dep.Missing() {
			missing = append(missing, dep)
		}
	}
	return missing
}

func expandHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("TIDAL_CHARM_GHCI_EXECUTABLE"); v != "" {
		cfg.GHCi.Executable = v
	}
	if v := os.Getenv("TIDAL_CHARM_GHCI_BOOT_SCRIPT"); v != "" {
		cfg.GHCi.BootScript = v
	}
	if v := os.Getenv("TIDAL_CHARM_GHCI_WORKING_DIR"); v != "" {
		cfg.GHCi.WorkingDir = v
	}
	if v := os.Getenv("TIDAL_CHARM_OSC_TIDAL_CONTROL_HOST"); v != "" {
		cfg.OSC.TidalControlHost = v
	}
	if v := os.Getenv("TIDAL_CHARM_OSC_TIDAL_CONTROL_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.OSC.TidalControlPort = port
		}
	}
	if v := os.Getenv("TIDAL_CHARM_UI_THEME"); v != "" {
		cfg.UI.Theme = v
	}
	if v := os.Getenv("TIDAL_CHARM_UI_DEFAULT_TEMPO"); v != "" {
		if tempo, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.UI.DefaultTempo = tempo
		}
	}
	if v := os.Getenv("TIDAL_CHARM_UI_VISIBLE_STREAMS"); v != "" {
		if streams, err := strconv.Atoi(v); err == nil {
			cfg.UI.VisibleStreams = streams
		}
	}
	if v := os.Getenv("TIDAL_CHARM_SUPERDIRT_EXECUTABLE"); v != "" {
		cfg.SuperDirt.Executable = v
	}
	if v := os.Getenv("TIDAL_CHARM_SUPERDIRT_BOOT_SCRIPT"); v != "" {
		cfg.SuperDirt.BootScript = v
	}
	if v := os.Getenv("TIDAL_CHARM_SUPERDIRT_AUTO_START"); v != "" {
		if autoStart, err := strconv.ParseBool(v); err == nil {
			cfg.SuperDirt.AutoStart = autoStart
		}
	}
	if v := os.Getenv("TIDAL_CHARM_CURSOR_ENABLED"); v != "" {
		if enabled, err := strconv.ParseBool(v); err == nil {
			cfg.Cursor.Enabled = enabled
		}
	}
	if v := os.Getenv("TIDAL_CHARM_CURSOR_HOST"); v != "" {
		cfg.Cursor.Host = v
	}
	if v := os.Getenv("TIDAL_CHARM_CURSOR_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Cursor.Port = port
		}
	}
	if v := os.Getenv("TIDAL_CHARM_CURSOR_BOOT_SCRIPT"); v != "" {
		cfg.Cursor.BootScript = v
	}
}
