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

func main() {
	openPath, err := parseArgs(os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "usage: triton [file.tidal]\n%v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	document := doc.New()
	if openPath != "" {
		if _, err := document.Load(openPath); err != nil {
			fmt.Fprintf(os.Stderr, "open error: %v\n", err)
			os.Exit(1)
		}
	}

	if missing := cfg.MissingDeps(); len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "missing dependencies:\n")
		for _, dep := range missing {
			fmt.Fprintf(os.Stderr, "  %s: %s not found (%s)\n", dep.Name, dep.Path, dep.Hint)
		}
		os.Exit(1)
	}

	var scProc *superdirt.Process
	var scReady chan bool
	if cfg.SuperDirt.AutoStart {
		script, cleanupScript, err := boot.SuperDirt(cfg.SuperDirt.BootScript, superdirtBootScript)
		if err != nil {
			fmt.Fprintf(os.Stderr, "preparing SuperDirt boot script: %v\n", err)
			os.Exit(1)
		}
		defer cleanupScript()
		cfg.SuperDirt.BootScript = script

		scProc, err = superdirt.New(cfg.SuperDirt.Executable, cfg.SuperDirt.BootScript)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to start sclang: %v\n", err)
			os.Exit(1)
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

	bootScript := cfg.GHCi.BootScript
	if cfg.Cursor.Enabled {
		path, cleanupBoot, err := boot.Cursor(cfg.Cursor.BootScript, cursorBootScript, cfg.Cursor.Host, cfg.Cursor.Port)
		if err != nil {
			fmt.Fprintf(os.Stderr, "preparing cursor boot script: %v\n", err)
			os.Exit(1)
		}
		defer cleanupBoot()
		bootScript = path
	}

	ghciProc, err := ghci.New(cfg.GHCi.Executable, bootScript, cfg.GHCi.WorkingDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start ghci: %v\n", err)
		fmt.Fprintf(os.Stderr, "check that %s is installed and %s exists\n", cfg.GHCi.Executable, bootScript)
		if scProc != nil {
			if scErr := scProc.Close(); scErr != nil {
				fmt.Fprintf(os.Stderr, "closing sclang: %v\n", scErr)
			}
		}
		os.Exit(1)
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
		BootScript: bootScript,
		WorkingDir: cfg.GHCi.WorkingDir,
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
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		if ghciErr := ghciProc.Close(); ghciErr != nil {
			fmt.Fprintf(os.Stderr, "closing ghci: %v\n", ghciErr)
		}
		if scProc != nil {
			if scErr := scProc.Close(); scErr != nil {
				fmt.Fprintf(os.Stderr, "closing sclang: %v\n", scErr)
			}
		}
		os.Exit(1)
	}
}
