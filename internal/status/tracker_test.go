package status

import (
	"testing"
)

func TestNewTracker(t *testing.T) {
	tr := NewTracker(4)
	streams := tr.Streams()
	if len(streams) != 4 {
		t.Fatalf("expected 4 streams, got %d", len(streams))
	}
	for i := 1; i <= 4; i++ {
		if streams[i] != StreamIdle {
			t.Errorf("stream %d: expected idle, got %d", i, streams[i])
		}
	}
}

func TestToggleMute(t *testing.T) {
	tr := NewTracker(4)
	tr.SetPlaying(2, "d2 $ sound \"cp\"")
	tr.SetMuted(2)
	newState := tr.ToggleMute(2)
	if newState != StreamActive {
		t.Errorf("expected active after unmuting a playing stream, got %d", newState)
	}

	newState = tr.ToggleMute(1)
	if newState != StreamMuted {
		t.Errorf("expected muted after first toggle, got %d", newState)
	}
	newState = tr.ToggleMute(1)
	if newState != StreamIdle {
		t.Errorf("unmuting a stream with nothing playing should return it to idle, got %d", newState)
	}
}

func TestUnmuteAll(t *testing.T) {
	tr := NewTracker(4)
	tr.SetPlaying(1, "d1 $ sound \"bd\"")
	tr.SetMuted(1)
	tr.SetMuted(2)
	tr.SetSoloed(3)

	tr.UnmuteAll()
	streams := tr.Streams()
	if streams[1] != StreamActive {
		t.Errorf("muted stream with playing code should unmute to active, got %d", streams[1])
	}
	if streams[2] != StreamIdle {
		t.Errorf("muted stream with nothing playing should unmute to idle, got %d", streams[2])
	}
	if streams[3] != StreamSoloed {
		t.Errorf("unmuteAll should leave soloed streams alone, got %d", streams[3])
	}
}

func TestToggleSolo(t *testing.T) {
	tr := NewTracker(4)
	newState := tr.ToggleSolo(2)
	if newState != StreamSoloed {
		t.Errorf("expected soloed after first toggle, got %d", newState)
	}
	newState = tr.ToggleSolo(2)
	if newState != StreamIdle {
		t.Errorf("unsoloing a stream with nothing playing should return it to idle, got %d", newState)
	}
}

func TestHushAll(t *testing.T) {
	tr := NewTracker(4)
	tr.SetPlaying(1, "d1 $ sound \"bd\"")
	tr.SetPlaying(2, "d2 $ sound \"sn\"")
	tr.SetMuted(3)
	tr.HushAll()
	streams := tr.Streams()
	for i := 1; i <= 4; i++ {
		if streams[i] != StreamIdle {
			t.Errorf("stream %d: expected idle after hush, got %d", i, streams[i])
		}
	}
}

func TestToggleMute_InvalidStream(t *testing.T) {
	tr := NewTracker(4)
	newState := tr.ToggleMute(99)
	if newState != StreamIdle {
		t.Errorf("expected idle for invalid stream, got %d", newState)
	}
}

func TestSetCPS(t *testing.T) {
	tr := NewTracker(4)
	tr.SetCPS(1.35)
	if tr.CPS() != 1.35 {
		t.Errorf("expected 1.35, got %f", tr.CPS())
	}
}

func TestSetGhciOK(t *testing.T) {
	tr := NewTracker(4)
	tr.SetGhciOK(false)
	if tr.GhciOK() {
		t.Error("expected false")
	}
	tr.SetGhciOK(true)
	if !tr.GhciOK() {
		t.Error("expected true")
	}
}

func TestSetPlaying(t *testing.T) {
	tr := NewTracker(4)
	tr.SetPlaying(2, "d2 $ sound \"cp\"")

	if tr.Streams()[2] != StreamActive {
		t.Errorf("expected idle stream to become active, got %d", tr.Streams()[2])
	}
	if code := tr.Playing()[2]; code != "d2 $ sound \"cp\"" {
		t.Errorf("expected playing code to be recorded, got %q", code)
	}
}

func TestSetPlaying_KeepsTransportState(t *testing.T) {
	tr := NewTracker(4)
	tr.SetMuted(1)
	tr.SetPlaying(1, "d1 $ sound \"bd\"")

	if tr.Streams()[1] != StreamMuted {
		t.Errorf("muted stream should stay muted after evaluation, got %d", tr.Streams()[1])
	}
	if _, ok := tr.Playing()[1]; !ok {
		t.Error("playing code should be recorded even for muted streams")
	}
}

func TestSetPlaying_InvalidStream(t *testing.T) {
	tr := NewTracker(4)
	tr.SetPlaying(99, "d99 $ sound \"bd\"")

	if _, ok := tr.Playing()[99]; ok {
		t.Error("unknown stream should not be recorded")
	}
}

func TestHushAll_ClearsPlaying(t *testing.T) {
	tr := NewTracker(4)
	tr.SetPlaying(1, "d1 $ sound \"bd\"")
	tr.SetEdited(1)
	tr.HushAll()

	if len(tr.Playing()) != 0 {
		t.Errorf("hush should clear playing code, got %v", tr.Playing())
	}
	if len(tr.Edited()) != 0 {
		t.Errorf("hush should clear edited flags, got %v", tr.Edited())
	}
}

func TestEditedFlags(t *testing.T) {
	tr := NewTracker(4)
	tr.SetEdited(1)
	tr.SetEdited(2)

	edited := tr.Edited()
	if !edited[1] || !edited[2] {
		t.Errorf("expected streams 1 and 2 to be edited, got %v", edited)
	}

	tr.ClearEdited(1)
	edited = tr.Edited()
	if edited[1] {
		t.Error("stream 1 should no longer be edited")
	}
	if !edited[2] {
		t.Error("clearing stream 1 should not affect stream 2")
	}
}

func TestSetEdited_InvalidStream(t *testing.T) {
	tr := NewTracker(4)
	tr.SetEdited(99)

	if tr.Edited()[99] {
		t.Error("unknown stream should not be marked edited")
	}
}
