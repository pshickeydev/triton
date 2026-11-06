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

	if tr.ToggleMute(2) {
		t.Error("unmuting a playing stream should clear the muted flag")
	}
	if got := tr.Streams()[2]; got != StreamActive {
		t.Errorf("expected active after unmuting a playing stream, got %d", got)
	}

	if !tr.ToggleMute(1) {
		t.Error("first toggle should mute")
	}
	if got := tr.Streams()[1]; got != StreamMuted {
		t.Errorf("expected muted after first toggle, got %d", got)
	}
	if tr.ToggleMute(1) {
		t.Error("second toggle should unmute")
	}
	if got := tr.Streams()[1]; got != StreamIdle {
		t.Errorf("unmuting a stream with nothing playing should return it to idle, got %d", got)
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

	if !tr.ToggleSolo(2) {
		t.Error("first toggle should solo")
	}
	if got := tr.Streams()[2]; got != StreamSoloed {
		t.Errorf("expected soloed after first toggle, got %d", got)
	}
	if tr.ToggleSolo(2) {
		t.Error("second toggle should unsolo")
	}
	if got := tr.Streams()[2]; got != StreamIdle {
		t.Errorf("unsoloing a stream with nothing playing should return it to idle, got %d", got)
	}
}

func TestHushAll(t *testing.T) {
	tr := NewTracker(4)
	tr.SetPlaying(1, "d1 $ sound \"bd\"")
	tr.SetPlaying(2, "d2 $ sound \"sn\"")
	tr.SetMuted(3)
	tr.HushAll()
	streams := tr.Streams()
	if streams[1] != StreamIdle || streams[2] != StreamIdle || streams[4] != StreamIdle {
		t.Errorf("playing streams should idle after hush, got %v", streams)
	}
	if streams[3] != StreamMuted {
		t.Errorf("hush keeps Tidal's mute flags, stream 3 = %d", streams[3])
	}
}

func TestToggleMute_InvalidStream(t *testing.T) {
	tr := NewTracker(4)
	if tr.ToggleMute(99) {
		t.Error("expected no mute for invalid stream")
	}
	if _, ok := tr.Streams()[99]; ok {
		t.Error("invalid stream should not be tracked")
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

// Tidal keeps mute and solo as independent per-stream flags, with solo
// taking precedence while any solo is active.
func TestMuteSoloIndependent(t *testing.T) {
	tr := NewTracker(4)
	tr.SetSoloed(1)
	tr.SetMuted(1)

	if got := tr.Streams()[1]; got != StreamSoloed {
		t.Errorf("a muted and soloed stream is audible (solo precedence), got %d", got)
	}

	tr.ToggleSolo(1)
	if got := tr.Streams()[1]; got != StreamMuted {
		t.Errorf("unsoloing a muted+soloed stream should return it to muted, got %d", got)
	}
}

func TestMuteAllLeavesSoloPrecedence(t *testing.T) {
	tr := NewTracker(4)
	tr.SetSoloed(1)
	tr.MuteAll()

	streams := tr.Streams()
	if streams[1] != StreamSoloed {
		t.Errorf("muteAll should keep the solo flag: soloed d1 got %d", streams[1])
	}
	if streams[2] != StreamMuted {
		t.Errorf("muteAll should mute d2, got %d", streams[2])
	}

	tr.UnmuteAll()
	streams = tr.Streams()
	if streams[1] != StreamSoloed {
		t.Errorf("unmuteAll should leave the soloed stream soloed, got %d", streams[1])
	}
	if streams[2] != StreamIdle {
		t.Errorf("unmuteAll should return d2 to idle, got %d", streams[2])
	}
}

// Hush stops the patterns but does not touch Tidal's mute/solo flags,
// which survive across hush.
func TestHushAllKeepsTransportFlags(t *testing.T) {
	tr := NewTracker(4)
	tr.SetPlaying(1, `d1 $ sound "bd"`)
	tr.SetMuted(2)
	tr.SetSoloed(3)

	tr.HushAll()
	streams := tr.Streams()
	if streams[1] != StreamIdle {
		t.Errorf("hush should return a plain playing stream to idle, got %d", streams[1])
	}
	if streams[2] != StreamMuted {
		t.Errorf("hush should keep the mute flag, got %d", streams[2])
	}
	if streams[3] != StreamSoloed {
		t.Errorf("hush should keep the solo flag, got %d", streams[3])
	}
}

// Streams d1..d16 are valid even when the panel shows fewer: hidden
// streams that get evaluated must stay tracked.
func TestHiddenStreamTracked(t *testing.T) {
	tr := NewTracker(4)
	tr.SetPlaying(5, `d5 $ sound "bd"`)

	if code, ok := tr.Playing()[5]; !ok || code != `d5 $ sound "bd"` {
		t.Errorf("hidden stream 5 should be tracked, playing = %v", tr.Playing())
	}
	if got := tr.Streams()[5]; got != StreamActive {
		t.Errorf("hidden stream 5 should derive active, got %d", got)
	}
}

func TestHiddenStreamMuteTracked(t *testing.T) {
	tr := NewTracker(4)
	tr.MuteAll()
	if got := tr.Streams()[12]; got != StreamMuted {
		t.Errorf("muteAll covers d12, got %d", got)
	}
	tr.UnmuteAll()
	if _, tracked := tr.Streams()[12]; tracked {
		t.Error("unmuted hidden stream with no state should not stay tracked")
	}
}

func TestToggleMuteHiddenUntracked(t *testing.T) {
	tr := NewTracker(4)
	if tr.ToggleMute(9) {
		t.Error("toggling mute on an untracked hidden stream should be a no-op")
	}
	if _, tracked := tr.Streams()[9]; tracked {
		t.Error("toggling mute should not register an untracked stream")
	}
}
