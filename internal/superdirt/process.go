// Package superdirt manages the SuperCollider/SuperDirt subprocess that
// synthesizes triton's audio.
package superdirt

import (
	"strings"
	"sync"
	"time"

	"github.com/pshickeydev/triton/internal/proc"
)

const (
	warnChanBuffer  = 8
	shutdownTimeout = 5 * time.Second
	bootWaitTimeout = 30 * time.Second
	readyMarker     = "SuperDirt ready"
	quitCmd         = "0.exit\n"

	// sc3PluginsMarker matches the warning SuperDirt prints at boot when it
	// skips its synth sound bank library because sc3-plugins is missing.
	sc3PluginsMarker = "sc3plugins are necessary and missing"
)

// sc3PluginsWarning is surfaced to the TUI when SuperDirt skips its synth
// sound banks (supermandolin, supergong, ...): patterns using them stay
// silent until a matching sc3-plugins build is installed.
const sc3PluginsWarning = "SuperDirt: sc3-plugins not found; synth sound banks (supermandolin, supergong, ...) will be silent; install sc3-plugins matching your SuperCollider version and restart"

// OutputMsg is one line of sclang output.
type OutputMsg = proc.Line

// Process drives an sclang session booting SuperDirt from a script.
type Process struct {
	handle  *proc.Handle
	warnCh  chan string
	readyCh chan struct{}
	mu      sync.Mutex
}

// New starts sclang with the SuperDirt boot script.
func New(executable, scriptPath string) (*Process, error) {
	p := &Process{
		warnCh:  make(chan string, warnChanBuffer),
		readyCh: make(chan struct{}),
	}
	h, err := proc.Start(proc.Options{
		Name:      "sclang",
		Exec:      executable,
		Args:      []string{scriptPath},
		QuitCmd:   quitCmd,
		Timeout:   shutdownTimeout,
		Configure: setProcGroup,
		KillCmd:   killProcGroup,
		OnLine:    p.observe,
		OnExit:    func() { close(p.warnCh) },
	})
	if err != nil {
		return nil, err
	}
	p.handle = h
	return p, nil
}

// observe inspects each output line for boot readiness and problems.
func (p *Process) observe(line proc.Line) {
	if isReadyLine(line.Text) {
		p.markReady()
	}
	if isSc3PluginsMissing(line.Text) {
		p.sendWarning(sc3PluginsWarning)
	}
}

func isReadyLine(line string) bool {
	return strings.Contains(line, readyMarker)
}

func isSc3PluginsMissing(line string) bool {
	return strings.Contains(line, sc3PluginsMarker)
}

// sendWarning delivers text to the Warnings channel, dropping it when the
// consumer is too slow: warnings are rare, so losing a duplicate is fine.
func (p *Process) sendWarning(text string) {
	select {
	case p.warnCh <- text:
	default:
	}
}

func (p *Process) markReady() {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.readyCh:
	default:
		close(p.readyCh)
	}
}

// WaitForReady blocks until the boot script reports readiness, the boot
// times out, or the process exits.
func (p *Process) WaitForReady() bool {
	select {
	case <-p.readyCh:
		return true
	case <-time.After(bootWaitTimeout):
		return false
	case <-p.handle.Done():
		return false
	}
}

// Output delivers sclang's output lines; it closes when the process exits.
func (p *Process) Output() <-chan OutputMsg {
	return p.handle.Output()
}

// Warnings delivers notable problems detected in SuperDirt's boot output,
// such as a missing sc3-plugins installation. It closes when the process
// exits.
func (p *Process) Warnings() <-chan string {
	return p.warnCh
}

// Done closes when the sclang process has exited and output is drained.
func (p *Process) Done() <-chan struct{} {
	return p.handle.Done()
}

// Close asks sclang to quit and waits up to five seconds before killing
// its process group.
func (p *Process) Close() error {
	return p.handle.Close()
}
