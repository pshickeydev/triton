package model

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	errSave      = "save error: "
	errOpen      = "open error: "
	errEmptyPath = "no file path given"
	msgSaved     = "saved to "
	msgOpened    = "opened "

	promptTitleOpen      = "Open file"
	promptTitleSaveAs    = "Save as"
	promptPlaceholder    = "path/to/file.tidal"
	promptHelpText       = " Enter: confirm | Esc: cancel"
	overwriteWarningText = "File exists. Press Enter to overwrite, Esc to cancel"
	overwriteHelpText    = " Enter: overwrite | Esc: cancel"
	discardWarningText   = "Unsaved changes will be lost. Press Enter to open, Esc to cancel"
	discardHelpText      = " Enter: open | Esc: cancel"
	promptInputWidth     = 40
	untitledFile         = "untitled.tidal"
	untitledLabel        = "untitled"
	dirtyMark            = "*"
)

type promptKind int

const (
	promptOpen promptKind = iota
	promptSaveAs
)

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmOverwrite
	confirmDiscard
)

func newPromptInput() textinput.Model {
	input := textinput.New()
	input.Placeholder = promptPlaceholder
	input.CharLimit = 0
	input.Width = promptInputWidth
	return input
}

// promptView renders the filename input box for open and save-as.
func (m Model) promptView() string {
	title := promptTitleOpen
	if m.promptKind == promptSaveAs {
		title = promptTitleSaveAs
	}
	return m.styles.Prompt.Render(title + "\n" + m.promptInput.View())
}

// confirmWarning names the pending file action in the warning overlay.
func (m Model) confirmWarning() string {
	if m.confirm == confirmDiscard {
		return discardWarningText
	}
	return overwriteWarningText
}

// fileLabel names the bound file for the help bar, marking unsaved
// changes with the edited-pattern marker.
func (m Model) fileLabel() string {
	name := untitledLabel
	if m.doc.Path() != "" {
		name = filepath.Base(m.doc.Path())
	}
	if m.dirty() {
		return name + dirtyMark
	}
	return name
}

// dirty reports whether the buffer differs from the bound document.
func (m Model) dirty() bool {
	return m.doc.Dirty(m.textarea.Value())
}

func (m Model) handleSave() (tea.Model, tea.Cmd) {
	if m.doc.Path() == "" {
		cmd := m.openPrompt(promptSaveAs)
		return m, cmd
	}
	return m.saveFile("")
}

// handlePromptKey routes keys to the filename input while it is open.
func (m Model) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.promptOn = false
		return m, nil
	case "ctrl+c":
		m.promptOn = false
		m.confirmQ = true
		return m, nil
	case "enter":
		return m.submitPrompt()
	}
	var cmd tea.Cmd
	m.promptInput, cmd = m.promptInput.Update(msg)
	return m, cmd
}

// handleConfirmKey routes keys while a file-action warning is shown: Enter
// proceeds, Esc returns to the filename prompt, Ctrl-C asks to quit.
func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		kind := m.confirm
		m.confirm = confirmNone
		if kind == confirmDiscard {
			return m.loadFile(m.pendingPath)
		}
		return m.saveFile(m.pendingPath)
	case "esc":
		m.confirm = confirmNone
		m.promptOn = true
		return m, m.promptInput.Focus()
	case "ctrl+c":
		m.confirm = confirmNone
		m.confirmQ = true
	}
	return m, nil
}

func (m Model) submitPrompt() (tea.Model, tea.Cmd) {
	path := strings.TrimSpace(m.promptInput.Value())
	if path == "" {
		m.console.AddError(errPrefix + errEmptyPath)
		return m, nil
	}
	m.promptOn = false
	if m.promptKind == promptOpen {
		if !m.dirty() {
			return m.loadFile(path)
		}
		m.confirm = confirmDiscard
		m.pendingPath = path
		return m, nil
	}
	exists, err := m.doc.Exists(path)
	if err != nil {
		m.console.AddError(errPrefix + errSave + err.Error())
		return m, nil
	}
	if !exists {
		return m.saveFile(path)
	}
	m.confirm = confirmOverwrite
	m.pendingPath = path
	return m, nil
}

// loadFile replaces the editor buffer with a file's content. Playing
// patterns are left alone: they show as edited until re-evaluated.
func (m Model) loadFile(path string) (tea.Model, tea.Cmd) {
	content, err := m.doc.Load(path)
	if err != nil {
		m.console.AddError(errPrefix + errOpen + err.Error())
		return m, nil
	}
	m.textarea.SetValue(content)
	m.refreshEdited()
	m.refreshSpans()
	m.console.AddOutput(statusPrefix + msgOpened + m.doc.Path())
	return m, nil
}

func (m Model) saveFile(path string) (tea.Model, tea.Cmd) {
	if err := m.doc.Save(path, m.textarea.Value()); err != nil {
		m.console.AddError(errPrefix + errSave + err.Error())
		return m, nil
	}
	m.console.AddOutput(statusPrefix + msgSaved + m.doc.Path())
	return m, nil
}

// openPrompt shows the filename input for opening or saving as, prefilling
// save-as with untitled.tidal (save-as only triggers for unbound buffers).
func (m *Model) openPrompt(kind promptKind) tea.Cmd {
	m.promptKind = kind
	m.promptOn = true
	m.confirmQ = false
	m.promptInput.SetValue("")
	if kind == promptSaveAs {
		m.promptInput.SetValue(untitledFile)
		m.promptInput.CursorEnd()
	}
	return m.promptInput.Focus()
}
