// Package ghci manages the GHCi subprocess that evaluates Tidal code.
package ghci

import (
	"fmt"
	"strings"
	"time"

	"github.com/pshickeydev/triton/internal/proc"
)

const (
	shutdownTimeout = 3 * time.Second
	quitCmd         = ":quit\n"
	multiLineStart  = ":{"
	multiLineEnd    = ":}"
	bootScriptCmd   = ":script %s\n"
)

// OutputMsg is one line of GHCi output, with stderr lines flagged.
type OutputMsg = proc.Line

// Process drives a GHCi session running a Tidal boot script.
type Process struct {
	handle *proc.Handle
}

// New starts GHCi in workingDir and loads the boot script, when set.
func New(executable, bootScript, workingDir string) (*Process, error) {
	h, err := proc.Start(proc.Options{
		Name:    "ghci",
		Exec:    executable,
		Dir:     workingDir,
		QuitCmd: quitCmd,
		Timeout: shutdownTimeout,
	})
	if err != nil {
		return nil, err
	}
	if bootScript != "" {
		if err := h.Write(fmt.Sprintf(bootScriptCmd, bootScript)); err != nil {
			h.Kill()
			return nil, fmt.Errorf("sending boot script command: %w", err)
		}
	}
	return &Process{handle: h}, nil
}

// Evaluate sends a single-line statement to GHCi.
func (p *Process) Evaluate(code string) error {
	return p.handle.Write(code + "\n")
}

// EvaluateMulti sends a multi-line block to GHCi. Independent statements
// are sent as single lines; expressions with continuation lines are
// wrapped in :{ ... :} so GHCi evaluates them as one unit.
func (p *Process) EvaluateMulti(code string) error {
	lines := strings.Split(strings.TrimSpace(code), "\n")
	if len(lines) <= 1 {
		return p.Evaluate(code)
	}

	var out strings.Builder
	for _, group := range groupStatements(lines) {
		if len(group) == 1 {
			out.WriteString(group[0] + "\n")
			continue
		}
		out.WriteString(multiLineStart + "\n")
		for _, line := range group {
			out.WriteString(line + "\n")
		}
		out.WriteString(multiLineEnd + "\n")
	}
	return p.handle.Write(out.String())
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

// Output delivers GHCi's output lines; it closes when the process exits.
func (p *Process) Output() <-chan OutputMsg {
	return p.handle.Output()
}

// Done closes when the GHCi process has exited and output is drained.
func (p *Process) Done() <-chan struct{} {
	return p.handle.Done()
}

// Close asks GHCi to quit and waits up to three seconds before killing it.
func (p *Process) Close() error {
	return p.handle.Close()
}
