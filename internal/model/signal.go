package model

import tea "charm.land/bubbletea/v2"

// sigIntMsg is the model-facing form of tea.InterruptMsg. Bubble Tea v2
// consumes InterruptMsg in the program loop before Update ever sees it, so
// FilterSignals rewrites it into this message instead.
type sigIntMsg struct{}

// FilterSignals rewrites OS-signal interrupts into messages the model
// handles, preserving the ctrl+c quit-confirmation flow. Pass it to
// tea.WithFilter when constructing the program.
func FilterSignals(_ tea.Model, msg tea.Msg) tea.Msg {
	if _, ok := msg.(tea.InterruptMsg); ok {
		return sigIntMsg{}
	}
	return msg
}
