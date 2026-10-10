package superdirt

import (
	"testing"

	"github.com/pshickeydev/triton/internal/proc"
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

func TestObserveDetectsReadyInSubstring(t *testing.T) {
	p := &Process{readyCh: make(chan struct{})}

	p.observe(proc.Line{Text: "some prefix SuperDirt ready trailing text"})

	select {
	case <-p.readyCh:
	default:
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

func TestObserveDetectsMissingSc3Plugins(t *testing.T) {
	p := &Process{warnCh: make(chan string, warnChanBuffer), readyCh: make(chan struct{})}

	p.observe(proc.Line{Text: "*** Dirt could not load some synths from default-synths.scd, because sc3plugins are necessary and missing."})

	select {
	case got := <-p.warnCh:
		if got != sc3PluginsWarning {
			t.Errorf("warning = %q, want %q", got, sc3PluginsWarning)
		}
	default:
		t.Fatal("no warning delivered for missing sc3-plugins")
	}
}

func TestObserveNoWarningWhenSc3PluginsPresent(t *testing.T) {
	p := &Process{warnCh: make(chan string, warnChanBuffer), readyCh: make(chan struct{})}

	p.observe(proc.Line{Text: "SuperDirt ready"})

	select {
	case got := <-p.warnCh:
		t.Errorf("unexpected warning %q", got)
	default:
	}
}
