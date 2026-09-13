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

type Tracker struct {
	mu      sync.RWMutex
	streams map[int]StreamState
	playing map[int]string
	edited  map[int]bool
	cps     float64
	ghciOK  bool
}

func NewTracker(visibleStreams int) *Tracker {
	t := &Tracker{
		streams: make(map[int]StreamState),
		playing: make(map[int]string),
		edited:  make(map[int]bool),
	}
	for i := 1; i <= visibleStreams; i++ {
		t.streams[i] = StreamIdle
	}
	return t
}

// SetPlaying records the code last evaluated for a stream so the panel
// can distinguish playing patterns from editor edits that were not
// evaluated yet. Muted and soloed streams keep their transport state.
func (t *Tracker) SetPlaying(streamNum int, code string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.streams[streamNum]; !ok {
		return
	}
	t.playing[streamNum] = code
	if t.streams[streamNum] == StreamIdle {
		t.streams[streamNum] = StreamActive
	}
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
	if _, ok := t.streams[streamNum]; ok {
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
	if _, ok := t.streams[streamNum]; ok {
		t.streams[streamNum] = StreamMuted
	}
}

func (t *Tracker) SetSoloed(streamNum int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.streams[streamNum]; ok {
		t.streams[streamNum] = StreamSoloed
	}
}

func (t *Tracker) ToggleMute(streamNum int) StreamState {
	t.mu.Lock()
	defer t.mu.Unlock()
	current, ok := t.streams[streamNum]
	if !ok {
		return StreamIdle
	}
	if current == StreamMuted {
		t.streams[streamNum] = t.unmutedState(streamNum)
		return t.streams[streamNum]
	}
	t.streams[streamNum] = StreamMuted
	return StreamMuted
}

// UnmuteAll restores muted streams: those with playing code become
// active again, the rest return to idle. Streams in other states are
// left alone.
func (t *Tracker) UnmuteAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for n, state := range t.streams {
		if state != StreamMuted {
			continue
		}
		t.streams[n] = t.unmutedState(n)
	}
}

// unmutedState is a stream's state after being unmuted: active when a
// pattern is playing on it, idle otherwise. Callers hold the mutex.
func (t *Tracker) unmutedState(streamNum int) StreamState {
	if _, ok := t.playing[streamNum]; ok {
		return StreamActive
	}
	return StreamIdle
}

func (t *Tracker) ToggleSolo(streamNum int) StreamState {
	t.mu.Lock()
	defer t.mu.Unlock()
	current, ok := t.streams[streamNum]
	if !ok {
		return StreamIdle
	}
	if current == StreamSoloed {
		t.streams[streamNum] = t.unmutedState(streamNum)
		return t.streams[streamNum]
	}
	t.streams[streamNum] = StreamSoloed
	return StreamSoloed
}

func (t *Tracker) HushAll() {
	t.mu.Lock()
	defer t.mu.Unlock()
	for k := range t.streams {
		t.streams[k] = StreamIdle
	}
	clear(t.playing)
	clear(t.edited)
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

func (t *Tracker) Streams() map[int]StreamState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	cp := make(map[int]StreamState, len(t.streams))
	maps.Copy(cp, t.streams)
	return cp
}
