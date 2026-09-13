package superdirt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/pshickeydev/triton/internal/chbuf"
)

const (
	outputChanBuffer  = 256
	scannerInitialBuf = 64 * 1024
	scannerMaxBuf     = 1024 * 1024
	warnChanBuffer    = 8
	shutdownTimeout   = 5 * time.Second
	bootWaitTimeout   = 30 * time.Second
	readyMarker       = "SuperDirt ready"
	quitCmd           = "0.exit\n"

	// sc3PluginsMarker matches the warning SuperDirt prints at boot when it
	// skips its synth sound bank library because sc3-plugins is missing.
	sc3PluginsMarker = "sc3plugins are necessary and missing"
)

// sc3PluginsWarning is surfaced to the TUI when SuperDirt skips its synth
// sound banks (supermandolin, supergong, ...): patterns using them stay
// silent until a matching sc3-plugins build is installed.
const sc3PluginsWarning = "SuperDirt: sc3-plugins not found; synth sound banks (supermandolin, supergong, ...) will be silent; install sc3-plugins matching your SuperCollider version and restart"

type OutputMsg struct {
	Text string
}

type Process struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	outputCh chan OutputMsg
	warnCh   chan string
	outputWg sync.WaitGroup
	done     chan struct{}
	mu       sync.Mutex
	running  bool
	readyCh  chan struct{}
}

func New(executable, scriptPath string) (*Process, error) {
	cmd := exec.Command(executable, scriptPath)
	setProcGroup(cmd)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	p := &Process{
		cmd:      cmd,
		stdin:    stdin,
		outputCh: make(chan OutputMsg, outputChanBuffer),
		warnCh:   make(chan string, warnChanBuffer),
		done:     make(chan struct{}),
		readyCh:  make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting sclang: %w", err)
	}
	p.running = true

	p.outputWg.Add(2)
	go p.readPipe(stdout)
	go p.readPipe(stderr)
	go func() {
		cmd.Wait()
		p.outputWg.Wait()
		// running flips and the channels close under the mutex so Close
		// cannot write to stdin or send on a closed channel concurrently.
		p.mu.Lock()
		p.running = false
		close(p.outputCh)
		close(p.warnCh)
		close(p.done)
		p.mu.Unlock()
	}()

	return p, nil
}

func (p *Process) readPipe(r io.ReadCloser) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, scannerInitialBuf), scannerMaxBuf)
	for scanner.Scan() {
		line := scanner.Text()
		chbuf.Send(p.outputCh, OutputMsg{Text: line})
		if isReadyLine(line) {
			p.markReady()
		}
		if isSc3PluginsMissing(line) {
			p.sendWarning(sc3PluginsWarning)
		}
	}
	// A closed pipe on shutdown is not a read failure.
	if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
		chbuf.Send(p.outputCh, OutputMsg{Text: "reading sclang output: " + err.Error()})
	}
	p.outputWg.Done()
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

func (p *Process) WaitForReady() bool {
	select {
	case <-p.readyCh:
		return true
	case <-time.After(bootWaitTimeout):
		return false
	case <-p.done:
		return false
	}
}

func (p *Process) Output() <-chan OutputMsg {
	return p.outputCh
}

// Warnings delivers notable problems detected in SuperDirt's boot output,
// such as a missing sc3-plugins installation. It closes when the process
// exits.
func (p *Process) Warnings() <-chan string {
	return p.warnCh
}

func (p *Process) Done() <-chan struct{} {
	return p.done
}

func (p *Process) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *Process) Close() error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return nil
	}
	if _, err := p.stdin.Write([]byte(quitCmd)); err != nil {
		// The quit command never arrived; report it and let the shutdown
		// timeout fall back to killing the process group.
		chbuf.Send(p.outputCh, OutputMsg{Text: "sending quit command: " + err.Error()})
	}
	p.stdin.Close()
	p.mu.Unlock()

	select {
	case <-p.done:
		return nil
	case <-time.After(shutdownTimeout):
		return killProcGroup(p.cmd)
	}
}
