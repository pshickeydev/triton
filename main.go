package main

import (
	_ "embed"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pshickeydev/triton/internal/boot"
	"github.com/pshickeydev/triton/internal/config"
	"github.com/pshickeydev/triton/internal/cursor"
	"github.com/pshickeydev/triton/internal/doc"
	"github.com/pshickeydev/triton/internal/ghci"
	"github.com/pshickeydev/triton/internal/model"
	"github.com/pshickeydev/triton/internal/osc"
	"github.com/pshickeydev/triton/internal/status"
	"github.com/pshickeydev/triton/internal/superdirt"
	"github.com/pshickeydev/triton/internal/ui"
)

//go:embed boot/BootTidal-triton.hs
var cursorBootScript string

//go:embed start-superdirt.scd
var superdirtBootScript string

// parseArgs returns the optional .tidal file to open at startup.
func parseArgs(args []string) (string, error) {
	if len(args) <= 1 {
		return "", nil
	}
	if strings.HasPrefix(args[1], "-") {
		return "", fmt.Errorf("unknown flag %q", args[1])
	}
	if len(args) > 2 {
		return "", fmt.Errorf("unexpected arguments: %v", args[2:])
	}
	return args[1], nil
}

// bootScripts carries the resolved boot script paths and their cleanups.
type bootScripts struct {
	ghci     string
	cleanups []func()
}

// cleanup runs every registered script cleanup, removing temp files.
func (b *bootScripts) cleanup() {
	for _, cleanup := range b.cleanups {
		cleanup()
	}
}

// resolveBootScripts resolves the SuperDirt and cursor boot scripts,
// embedding the bundled fallbacks when the configured paths are the
// default names and no local copy exists. It must run before the
// dependency check: otherwise triton exits with a missing boot script
// whenever it runs outside its checkout.
func resolveBootScripts(cfg *config.Config) (*bootScripts, error) {
	b := &bootScripts{ghci: cfg.GHCi.BootScript}
	if cfg.SuperDirt.AutoStart {
		script, cleanup, err := boot.SuperDirt(cfg.SuperDirt.BootScript, superdirtBootScript)
		if err != nil {
			return nil, fmt.Errorf("preparing SuperDirt boot script: %w", err)
		}
		b.cleanups = append(b.cleanups, cleanup)
		cfg.SuperDirt.BootScript = script
	}
	if cfg.Cursor.Enabled {
		path, cleanup, err := boot.Cursor(cfg.Cursor.BootScript, cursorBootScript, cfg.Cursor.Host, cfg.Cursor.Port)
		if err != nil {
			return nil, fmt.Errorf("preparing cursor boot script: %w", err)
		}
		b.cleanups = append(b.cleanups, cleanup)
		b.ghci = path
	}
	return b, nil
}

// missingDepsReport formats the dependency check failure, one
// indented entry per missing dependency.
func missingDepsReport(missing []config.Dependency) string {
	entries := make([]string, 0, len(missing))
	for _, dep := range missing {
		entries = append(entries, fmt.Sprintf("  %s: %s not found (%s)", dep.Name, dep.Path, dep.Hint))
	}
	return strings.Join(entries, "\n")
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func run() error {
	openPath, err := parseArgs(os.Args)
	if err != nil {
		return fmt.Errorf("usage: triton [file.tidal]\n%v", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	document := doc.New()
	if openPath != "" {
		if _, err := document.Load(openPath); err != nil {
			return fmt.Errorf("open error: %w", err)
		}
	}

	scripts, err := resolveBootScripts(cfg)
	if err != nil {
		return err
	}
	defer scripts.cleanup()

	if missing := cfg.MissingDeps(); len(missing) > 0 {
		return fmt.Errorf("missing dependencies:\n%s", missingDepsReport(missing))
	}

	var scProc *superdirt.Process
	var scReady chan bool
	if cfg.SuperDirt.AutoStart {
		scProc, err = superdirt.New(cfg.SuperDirt.Executable, cfg.SuperDirt.BootScript)
		if err != nil {
			return fmt.Errorf("failed to start sclang: %w", err)
		}
		defer func() {
			fmt.Fprintf(os.Stderr, "closing sclang...\n")
			scProc.Close()
		}()

		// The editor and GHCi start without blocking on the audio engine;
		// readiness is reported in the console when it arrives. The channel
		// is buffered so this goroutine never outlives the handoff.
		scReady = make(chan bool, 1)
		go func() { scReady <- scProc.WaitForReady() }()
	}

	ghciProc, err := ghci.New(cfg.GHCi.Executable, scripts.ghci, cfg.GHCi.WorkingDir)
	if err != nil {
		return fmt.Errorf("failed to start ghci: %w\ncheck that %s is installed and %s exists", err, cfg.GHCi.Executable, scripts.ghci)
	}
	defer func() {
		fmt.Fprintf(os.Stderr, "closing ghci...\n")
		ghciProc.Close()
	}()

	oscClient := osc.New(cfg.OSC.TidalControlHost, cfg.OSC.TidalControlPort)
	defer oscClient.Close()

	tracker := status.NewTracker(cfg.UI.VisibleStreams)
	tracker.SetCPS(cfg.UI.DefaultTempo)
	tracker.SetGhciOK(true)

	theme := ui.ThemeFor(cfg.UI.Theme)
	panel := status.NewPanel(tracker, theme.Palette)
	console := status.NewLog(theme.Palette)

	ghciCfg := model.GhciConfig{
		Executable: cfg.GHCi.Executable,
		BootScript: scripts.ghci,
		WorkingDir: cfg.GHCi.WorkingDir,
		Tempo:      cfg.UI.DefaultTempo,
	}

	opts := []model.Option{model.WithSclang(cfg.SuperDirt.AutoStart), model.WithDocument(document)}
	if cfg.SuperDirt.AutoStart {
		opts = append(opts, model.WithSclangWarnings(scProc.Warnings()), model.WithSclangReady(scReady))
	}
	if cfg.Cursor.Enabled {
		listener := cursor.NewListener(cfg.Cursor.Host, cfg.Cursor.Port)
		defer listener.Close()
		listenerErr := make(chan error, 1)
		go func() {
			if err := listener.Listen(); err != nil {
				listenerErr <- err
			}
		}()
		opts = append(opts, model.WithCursor(listener.Highlights(), listenerErr))
	}

	m := model.NewModel(theme, ghciProc, oscClient, tracker, panel, console, ghciCfg, opts...)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		if final, ok := finalModel.(model.Model); ok {
			if closeErr := final.Close(); closeErr != nil {
				return fmt.Errorf("tui error: %w; closing current ghci: %v", err, closeErr)
			}
		}
		return fmt.Errorf("tui error: %w", err)
	}
	return nil
}
