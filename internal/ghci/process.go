package ghci

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
	shutdownTimeout   = 3 * time.Second
	quitCmd           = ":quit\n"
	multiLineStart    = ":{"
	multiLineEnd      = ":}"
	bootScriptCmdFmt  = ":script %s\n"
)

type OutputMsg struct {
	Text    string
	IsError bool
}

type Process struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	stderr   io.ReadCloser
	outputCh chan OutputMsg
	outputWg sync.WaitGroup
	done     chan struct{}
	mu       sync.Mutex
	running  bool
}

func New(executable, bootScript, workingDir string) (*Process, error) {
	args := []string{}
	cmd := exec.Command(executable, args...)
	cmd.Dir = workingDir

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
		stdout:   stdout,
		stderr:   stderr,
		outputCh: make(chan OutputMsg, outputChanBuffer),
		done:     make(chan struct{}),
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting ghci: %w", err)
	}
	p.running = true

	p.outputWg.Add(2)
	go p.readPipe(stdout, pipeKindStdout)
	go p.readPipe(stderr, pipeKindStderr)
	go func() {
		cmd.Wait()
		p.outputWg.Wait()
		// running flips and the channels close under the mutex so Close
		// cannot write to stdin or send on a closed channel concurrently.
		p.mu.Lock()
		p.running = false
		close(p.outputCh)
		close(p.done)
		p.mu.Unlock()
	}()

	if bootScript != "" {
		if _, err := fmt.Fprintf(p.stdin, bootScriptCmdFmt, bootScript); err != nil {
			p.cmd.Process.Kill()
			<-p.done
			return nil, fmt.Errorf("sending boot script command: %w", err)
		}
	}

	return p, nil
}

type pipeKind int

const (
	pipeKindStdout pipeKind = iota
	pipeKindStderr
)

func (p *Process) readPipe(r io.ReadCloser, kind pipeKind) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, scannerInitialBuf), scannerMaxBuf)
	for scanner.Scan() {
		chbuf.Send(p.outputCh, OutputMsg{Text: scanner.Text(), IsError: kind == pipeKindStderr})
	}
	// A closed pipe on shutdown is not a read failure.
	if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
		chbuf.Send(p.outputCh, OutputMsg{Text: "reading ghci output: " + err.Error(), IsError: true})
	}
	p.outputWg.Done()
}

func (p *Process) Evaluate(code string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.writeLocked(code)
}

func (p *Process) writeLocked(code string) error {
	if !p.running {
		return fmt.Errorf("ghci process is not running")
	}

	_, err := fmt.Fprintf(p.stdin, "%s\n", code)
	if err != nil {
		return fmt.Errorf("writing to ghci stdin: %w", err)
	}
	return nil
}

func (p *Process) EvaluateMulti(code string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return fmt.Errorf("ghci process is not running")
	}

	lines := strings.Split(strings.TrimSpace(code), "\n")
	if len(lines) <= 1 {
		return p.writeLocked(code)
	}

	groups := groupStatements(lines)
	for _, group := range groups {
		if len(group) == 1 {
			if err := p.writeLocked(group[0]); err != nil {
				return err
			}
			continue
		}

		if _, err := fmt.Fprintln(p.stdin, multiLineStart); err != nil {
			return fmt.Errorf("writing multi-line start: %w", err)
		}
		for _, line := range group {
			if _, err := fmt.Fprintln(p.stdin, line); err != nil {
				return fmt.Errorf("writing multi-line body: %w", err)
			}
		}
		if _, err := fmt.Fprintln(p.stdin, multiLineEnd); err != nil {
			return fmt.Errorf("writing multi-line end: %w", err)
		}
	}
	return nil
}

func groupStatements(lines []string) [][]string {
	var groups [][]string
	var current []string

	for _, line := range lines {
		if line == "" {
			continue
		}

		isContinuation := len(line) > 0 && (line[0] == ' ' || line[0] == '\t')

		if isContinuation {
			current = append(current, line)
			continue
		}

		if len(current) > 0 {
			groups = append(groups, current)
		}
		current = []string{line}
	}

	if len(current) > 0 {
		groups = append(groups, current)
	}

	return groups
}

func (p *Process) Output() <-chan OutputMsg {
	return p.outputCh
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
		// timeout fall back to killing the process.
		chbuf.Send(p.outputCh, OutputMsg{Text: "sending quit command: " + err.Error(), IsError: true})
	}
	p.stdin.Close()
	p.mu.Unlock()

	select {
	case <-p.done:
		return nil
	case <-time.After(shutdownTimeout):
		return p.cmd.Process.Kill()
	}
}
