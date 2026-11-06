package status

import (
	"maps"
	"sync"
)

type StreamState int

const (
	StreamIdle StreamState = iota
	StreamActive
	StreamMuted
	StreamSoloed
)

// maxStream is Tidal's highest pattern stream id (d16): streams up to it
// are valid and stay tracked even when the panel shows fewer of them.
const maxStream = 16

// Tracker mirrors Tidal's transport state. Mute and solo are independent
// per-stream flags, like Tidal's psMute and psSolo: while any stream is
// soloed, solo takes precedence over mute, so a muted and soloed stream
// derives as soloed (and is audible). Only the visible stream ids are
// pre-registered; hidden valid streams join as soon as they carry state.
type Tracker struct {
	mu      sync.RWMutex
	visible int
	playing map[int]string
	edited  map[int]bool
	muted   map[int]bool
	soloed  map[int]bool
	cps     float64
	ghciOK  bool
}

func NewTracker(visibleStreams int) *Tracker {
	return &Tracker{
		visible: visibleStreams,
		playing: make(map[int]string),
		edited:  make(map[int]bool),
		muted:   make(map[int]bool),
		soloed:  make(map[int]bool),
	}
}

// Visible returns the number of streams the panel shows.
func (t *Tracker) Visible() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.visible
}

// valid reports whether streamNum is one of Tidal's d1..d16.
func valid(streamNum int) bool {
	return streamNum >= 1 && streamNum <= maxStream
}

// SetPlaying records the code last evaluated for a stream so the panel
// can distinguish playing patterns from editor edits that were not
// evaluated yet. Muted and soloed streams keep their transport state.
// A hidden but valid stream becomes tracked.
func (t *Tracker) SetPlaying(streamNum int, code string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !valid(streamNum) {
		return
	}
	t.playing[streamNum] = code
}

func (t *Tracker) Playing() map[int]string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	cp := make(map[int]string, len(t.playing))
	maps.Copy(cp, t.playing)
	return cp
}

func (t *Tracker) SetEdited(streamNum int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if valid(streamNum) {
		t.edited[streamNum] = true
	}
}

func (t *Tracker) ClearEdited(streamNum int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.edited, streamNum)
}

func (t *Tracker) Edited() map[int]bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	cp := make(map[int]bool, len(t.edited))
	maps.Copy(cp, t.edited)
	return cp
}

func (t *Tracker) SetMuted(streamNum int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if valid(streamNum) {
		t.muted[streamNum] = true
	}
}

func (t *Tracker) SetSoloed(streamNum int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if valid(streamNum) {
		t.soloed[streamNum] = true
	}
}

// ToggleMute flips a stream's mute flag, leaving its solo flag alone,
// and returns the stream's new muted state.
func (t *Tracker) ToggleMute(streamNum int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !valid(streamNum) || !t.tracked(streamNum) {
		return false
	}
	t.muted[streamNum] = !t.muted[streamNum]
	return t.muted[streamNum]
}

// ToggleSolo flips a stream's solo flag, leaving its mute flag alone,
// and returns the stream's new soloed state.
func (t *Tracker) ToggleSolo(streamNum int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !valid(streamNum) || !t.tracked(streamNum) {
		return false
	}
	t.soloed[streamNum] = !t.soloed[streamNum]
	return t.soloed[streamNum]
}

// MuteAll mutes every stream, the way Tidal's muteAll does; solo flags
// are untouched.
func (t *Tracker) MuteAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for n := 1; n <= maxStream; n++ {
		t.muted[n] = true
	}
}

// UnmuteAll restores muted streams to the state they would have without
// the mute flag: those with playing code become active again, the rest
// return to idle. Solo flags are untouched.
func (t *Tracker) UnmuteAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	clear(t.muted)
}

// HushAll clears the playing state, like Tidal's hush stopping every
// pattern. Mute and solo flags survive, the way they do in Tidal.
func (t *Tracker) HushAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	clear(t.playing)
	clear(t.edited)
}

// Reset clears every tracked state, for a freshly booted (or dead) GHCi
// whose transport state no longer exists.
func (t *Tracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	clear(t.playing)
	clear(t.edited)
	clear(t.muted)
	clear(t.soloed)
}

// Streams derives every tracked stream's display state with solo
// precedence: soloed over muted over playing over idle. Hidden streams
// are included while they carry state.
func (t *Tracker) Streams() map[int]StreamState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	states := make(map[int]StreamState, t.visible)
	for n := 1; n <= maxStream; n++ {
		if !t.tracked(n) {
			continue
		}
		_, playing := t.playing[n]
		switch {
		case t.soloed[n]:
			states[n] = StreamSoloed
		case t.muted[n]:
			states[n] = StreamMuted
		case playing:
			states[n] = StreamActive
		default:
			states[n] = StreamIdle
		}
	}
	return states
}

// tracked reports whether stream n is visible or carries any state.
// Callers hold the mutex.
func (t *Tracker) tracked(n int) bool {
	if n <= t.visible {
		return true
	}
	_, inPlaying := t.playing[n]
	_, inEdited := t.edited[n]
	_, inMuted := t.muted[n]
	_, inSoloed := t.soloed[n]
	return inPlaying || inEdited || inMuted || inSoloed
}

func (t *Tracker) SetCPS(cps float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cps = cps
}

func (t *Tracker) CPS() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.cps
}

func (t *Tracker) SetGhciOK(ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ghciOK = ok
}

func (t *Tracker) GhciOK() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ghciOK
}
