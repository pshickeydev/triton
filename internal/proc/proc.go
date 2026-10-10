// Package proc provides the shared plumbing for triton's child
// processes: line-oriented output forwarding with newest-first retention,
// a done channel, and graceful quit-then-kill shutdown. The GHCi and
// SuperDirt drivers build on it.
package proc

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/pshickeydev/triton/internal/chbuf"
)

const (
	outputChanBuffer  = 256
	scannerInitialBuf = 64 * 1024
	scannerMaxBuf     = 1024 * 1024
)

// Line is one line of child-process output, tagged by origin pipe.
type Line struct {
	Text    string
	IsError bool
}

// Options configures Start.
type Options struct {
	// Name labels the child in error messages, e.g. "ghci".
	Name string
	// Exec and Args specify the command; Dir its working directory.
	Exec string
	Args []string
	Dir  string
	// QuitCmd is written to stdin on Close before the timeout expires.
	QuitCmd string
	// Timeout bounds the graceful shutdown before KillCmd runs.
	Timeout time.Duration
	// Configure adjusts the command before start, e.g. process-group setup.
	Configure func(*exec.Cmd)
	// OnLine observes every output line as it is forwarded.
	OnLine func(Line)
	// OnExit runs once when the process exits, before Output closes.
	OnExit func()
	// KillCmd terminates the process after the shutdown timeout. The
	// default kills only the process itself.
	KillCmd func(*exec.Cmd) error
}

// Handle manages one running child process.
type Handle struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	outputCh chan Line
	outputWg sync.WaitGroup
	done     chan struct{}
	mu       sync.Mutex
	running  bool
	name     string
	quitCmd  string
	timeout  time.Duration
	killCmd  func(*exec.Cmd) error
	onLine   func(Line)
}

// Start launches the child and begins forwarding its output.
func Start(opts Options) (*Handle, error) {
	cmd := exec.Command(opts.Exec, opts.Args...)
	cmd.Dir = opts.Dir
	if opts.Configure != nil {
		opts.Configure(cmd)
	}

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

	h := &Handle{
		cmd:      cmd,
		stdin:    stdin,
		outputCh: make(chan Line, outputChanBuffer),
		done:     make(chan struct{}),
		name:     opts.Name,
		quitCmd:  opts.QuitCmd,
		timeout:  opts.Timeout,
		killCmd:  opts.KillCmd,
		onLine:   opts.OnLine,
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting %s: %w", opts.Name, err)
	}
	h.running = true

	h.outputWg.Add(2)
	go h.readPipe(stdout, pipeKindStdout)
	go h.readPipe(stderr, pipeKindStderr)
	go func() {
		cmd.Wait()
		h.outputWg.Wait()
		// running flips and the channels close under the mutex so Close
		// cannot write to stdin or send on a closed channel concurrently.
		h.mu.Lock()
		h.running = false
		if opts.OnExit != nil {
			opts.OnExit()
		}
		close(h.outputCh)
		close(h.done)
		h.mu.Unlock()
	}()

	return h, nil
}

type pipeKind int

const (
	pipeKindStdout pipeKind = iota
	pipeKindStderr
)

func (h *Handle) readPipe(r io.ReadCloser, kind pipeKind) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, scannerInitialBuf), scannerMaxBuf)
	for scanner.Scan() {
		line := Line{Text: scanner.Text(), IsError: kind == pipeKindStderr}
		chbuf.Send(h.outputCh, line)
		if h.onLine != nil {
			h.onLine(line)
		}
	}
	// A closed pipe on shutdown is not a read failure.
	if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
		chbuf.Send(h.outputCh, Line{Text: fmt.Sprintf("reading %s output: %v", h.name, err), IsError: true})
	}
	h.outputWg.Done()
}

// Write sends text to the child's stdin verbatim.
func (h *Handle) Write(text string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.running {
		return fmt.Errorf("%s process is not running", h.name)
	}
	if _, err := io.WriteString(h.stdin, text); err != nil {
		return fmt.Errorf("writing to %s stdin: %w", h.name, err)
	}
	return nil
}

// Kill terminates the child and waits for its cleanup to settle. It backs
// early-failure paths in drivers, where the process was just started.
func (h *Handle) Kill() {
	if h.killCmd != nil {
		h.killCmd(h.cmd)
	} else {
		h.cmd.Process.Kill()
	}
	<-h.done
}

// Output delivers the child's output lines; it closes when the process
// exits and its pipes are drained.
func (h *Handle) Output() <-chan Line {
	return h.outputCh
}

// Done closes when the process has exited and its output is drained.
func (h *Handle) Done() <-chan struct{} {
	return h.done
}

// Pid reports the child's process id.
func (h *Handle) Pid() int {
	return h.cmd.Process.Pid
}

// Close asks the child to quit and waits up to the configured timeout
// before killing it.
func (h *Handle) Close() error {
	h.mu.Lock()
	if !h.running {
		h.mu.Unlock()
		return nil
	}
	if _, err := io.WriteString(h.stdin, h.quitCmd); err != nil {
		// The quit command never arrived; report it and let the shutdown
		// timeout fall back to killing the process.
		chbuf.Send(h.outputCh, Line{Text: "sending quit command: " + err.Error(), IsError: true})
	}
	h.stdin.Close()
	h.mu.Unlock()

	select {
	case <-h.done:
		return nil
	case <-time.After(h.timeout):
		if h.killCmd != nil {
			return h.killCmd(h.cmd)
		}
		return h.cmd.Process.Kill()
	}
}
