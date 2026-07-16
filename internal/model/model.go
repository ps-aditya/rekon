// Package model defines Rekon's bubbletea Model, Update, and View.
//
// This is Sprint 2's core proof: incoming poller.Snapshot values arrive
// as just another kind of bubbletea message (tea.Msg) — the same
// category as a keypress — and Update reacts to them by producing a new
// Model, which View then renders. No shared mutable state between the
// polling goroutine and the UI: the channel from internal/poller is the
// only handoff, exactly as designed in ARCHITECTURE.md section 2.
//
// This sprint intentionally renders raw, unstyled text. Real panels with
// lipgloss styling and parsed fields are Sprint 3+.
package model

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rekon/rekon/internal/poller"
)

// snapshotMsg wraps a poller.Snapshot so it can travel through
// bubbletea's Update as a tea.Msg.
type snapshotMsg poller.Snapshot

// closedMsg signals the poller's Results channel was closed (poller
// stopped), distinct from a snapshot carrying a poll error.
type closedMsg struct{}

// Model holds everything currently true about the running program.
// Update never mutates a Model in place — it returns a new one, the
// same "new row, not an edited row" idea we discussed for SQL UPDATE.
type Model struct {
	results   <-chan poller.Snapshot
	connected bool
	lastLine  string
	lastErr   error
	pollCount int
}

// New creates a Model that will listen on results for incoming snapshots.
func New(results <-chan poller.Snapshot) Model {
	return Model{
		results:   results,
		connected: false,
	}
}

// waitForSnapshot returns a tea.Cmd — a function bubbletea will run in
// its own goroutine — that blocks on the results channel and turns
// whatever arrives into a tea.Msg. A tea.Cmd fires once per call, so
// Update must call this again after each snapshot arrives to keep
// listening; see the snapshotMsg case below.
func waitForSnapshot(results <-chan poller.Snapshot) tea.Cmd {
	return func() tea.Msg {
		snap, ok := <-results
		if !ok {
			return closedMsg{}
		}
		return snapshotMsg(snap)
	}
}

// Init is called once when the program starts. Returning
// waitForSnapshot here is what kicks off listening for the first
// snapshot — without this, Update would never receive one.
func (m Model) Init() tea.Cmd {
	return waitForSnapshot(m.results)
}

// Update reacts to one message and returns the next Model plus
// optionally another Cmd to run. This is the function that actually
// proves the design: a snapshotMsg arriving is handled exactly like a
// keypress arriving — both are just tea.Msg values dispatched here.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case snapshotMsg:
		m.pollCount++
		if msg.Err != nil {
			m.connected = false
			m.lastErr = msg.Err
		} else {
			m.connected = true
			m.lastErr = nil
			m.lastLine = firstLine(msg.Info)
		}
		// Keep listening for the next snapshot. Without re-issuing this
		// Cmd, Rekon would render exactly one snapshot and then go
		// silent forever, since a Cmd only fires once per call.
		return m, waitForSnapshot(m.results)

	case closedMsg:
		m.connected = false
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the current Model to text. It never changes anything —
// it only reads m and produces a string. Deliberately plain/unstyled;
// lipgloss panels are Sprint 3+.
func (m Model) View() string {
	if m.lastErr != nil {
		return fmt.Sprintf("rekon (sprint 2 skeleton)\n\nconnection error: %v\n\npress q to quit\n", m.lastErr)
	}
	if !m.connected {
		return "rekon (sprint 2 skeleton)\n\nwaiting for first poll...\n\npress q to quit\n"
	}
	return fmt.Sprintf(
		"rekon (sprint 2 skeleton)\n\npolls received: %d\nlatest: %s\n\npress q to quit\n",
		m.pollCount, m.lastLine,
	)
}

// firstLine returns just the first line of a multi-line INFO reply, to
// keep this skeleton's View compact. Real field parsing is Sprint 3.
func firstLine(info string) string {
	for i := 0; i < len(info); i++ {
		if info[i] == '\r' || info[i] == '\n' {
			return info[:i]
		}
	}
	return info
}
