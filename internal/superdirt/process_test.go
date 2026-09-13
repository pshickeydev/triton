package superdirt

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestIsReadyLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"SuperDirt ready", true},
		{"[sclang] SuperDirt ready", true},
		{"SuperDirt ready extra text", true},
		{"some prefix SuperDirt ready trailing", true},
		{"SuperDirt", false},
		{"not ready", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isReadyLine(tt.line); got != tt.want {
			t.Errorf("isReadyLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestReadPipeDetectsReadyInSubstring(t *testing.T) {
	input := "some prefix SuperDirt ready trailing text\n"
	r := io.NopCloser(strings.NewReader(input))

	p := &Process{
		outputCh: make(chan OutputMsg, outputChanBuffer),
		done:     make(chan struct{}),
		readyCh:  make(chan struct{}),
	}

	p.outputWg.Add(1)
	p.readPipe(r)

	select {
	case <-p.readyCh:
	case <-time.After(time.Second):
		t.Fatal("readyCh not closed: readiness not detected from substring match")
	}
}

func TestIsSc3PluginsMissing(t *testing.T) {
	upstream := "*** Dirt could not load some synths from default-synths.scd, because sc3plugins are necessary and missing."
	tests := []struct {
		line string
		want bool
	}{
		{upstream, true},
		{"prefix " + upstream + " trailing", true},
		{"SuperDirt ready", false},
		{"sc3plugins are installed", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isSc3PluginsMissing(tt.line); got != tt.want {
			t.Errorf("isSc3PluginsMissing(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestReadPipeDetectsMissingSc3Plugins(t *testing.T) {
	input := "*** Dirt could not load some synths from default-synths.scd, because sc3plugins are necessary and missing.\nSuperDirt ready\n"
	r := io.NopCloser(strings.NewReader(input))

	p := &Process{
		outputCh: make(chan OutputMsg, outputChanBuffer),
		warnCh:   make(chan string, warnChanBuffer),
		done:     make(chan struct{}),
		readyCh:  make(chan struct{}),
	}

	p.outputWg.Add(1)
	p.readPipe(r)

	select {
	case got := <-p.warnCh:
		if got != sc3PluginsWarning {
			t.Errorf("warning = %q, want %q", got, sc3PluginsWarning)
		}
	default:
		t.Fatal("no warning delivered for missing sc3-plugins")
	}
}

func TestReadPipeNoWarningWhenSc3PluginsPresent(t *testing.T) {
	p := &Process{
		outputCh: make(chan OutputMsg, outputChanBuffer),
		warnCh:   make(chan string, warnChanBuffer),
		done:     make(chan struct{}),
		readyCh:  make(chan struct{}),
	}

	p.outputWg.Add(1)
	p.readPipe(io.NopCloser(strings.NewReader("SuperDirt ready\n")))

	select {
	case got := <-p.warnCh:
		t.Errorf("unexpected warning %q", got)
	default:
	}
}

func TestReadPipe_KeepsNewestWhenFull(t *testing.T) {
	p := &Process{outputCh: make(chan OutputMsg, 2)}
	p.outputWg.Add(1)

	p.readPipe(io.NopCloser(strings.NewReader("l0\nl1\nl2\nl3\nl4\n")))

	var got []string
	for {
		select {
		case msg := <-p.outputCh:
			got = append(got, msg.Text)
			continue
		default:
		}
		break
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 buffered messages, got %d", len(got))
	}
	if got[0] != "l3" || got[1] != "l4" {
		t.Errorf("expected newest lines l3,l4 kept, got %v", got)
	}
}
