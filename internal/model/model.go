// Package model defines Rekon's bubbletea Model, Update, and View.
//
// This is Sprint 2's core proof, now extended in Sprint 3 with real
// parsed metrics: incoming poller.Snapshot values arrive as just another
// kind of bubbletea message (tea.Msg) — the same category as a
// keypress — and Update reacts to them by producing a new Model, which
// View then renders. No shared mutable state between the polling
// goroutine and the UI: the channel from internal/poller is the only
// handoff, exactly as designed in ARCHITECTURE.md section 2.
package model

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rekon/rekon/internal/metrics"
	"github.com/rekon/rekon/internal/poller"
	"github.com/rekon/rekon/internal/redis"
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
	lastErr   error
	pollCount int
	memory    metrics.Memory
	ops       metrics.Ops
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
// whatever arrives into a tea.Msg. A tea.Cmd fires once per call: if
// this isn't re-issued after every snapshot, Rekon would render exactly
// one poll result and then permanently stop updating, even though the
// poller keeps ticking forever with nowhere for its results to go.
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
// optionally another Cmd to run.
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
			info := redis.ParseInfo(msg.Info)
			m.memory = metrics.ParseMemory(info)
			m.ops = metrics.ParseOps(info)
		}
		// Keep listening for the next snapshot — see waitForSnapshot's
		// doc comment for why this re-issue is required every time.
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

// Panel styling. Colors are chosen per metrics.Status so the same
// threshold judgment computed in internal/metrics drives what the user
// sees — the View layer never re-decides what's concerning, it only
// renders the decision metrics.go already made.
var (
	panelBorder = lipgloss.RoundedBorder()

	statusColor = map[metrics.Status]lipgloss.Color{
		metrics.StatusOK:               lipgloss.Color("42"),  // green
		metrics.StatusWarn:             lipgloss.Color("214"), // orange
		metrics.StatusCritical:         lipgloss.Color("196"), // red
		metrics.StatusUnknown:          lipgloss.Color("240"), // grey
		metrics.StatusInsufficientData: lipgloss.Color("240"), // grey, same as unknown — neither is an alarm color
	}

	titleStyle = lipgloss.NewStyle().Bold(true)
)

func panelStyle(status metrics.Status) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(panelBorder).
		BorderForeground(statusColor[status]).
		Padding(0, 1).
		MarginRight(2)
}

// View renders the current Model to text.
func (m Model) View() string {
	if m.lastErr != nil {
		return fmt.Sprintf("rekon\n\nconnection error: %v\n\npress q to quit\n", m.lastErr)
	}
	if !m.connected {
		return "rekon\n\nwaiting for first poll...\n\npress q to quit\n"
	}

	memPanel := m.renderMemoryPanel()
	opsPanel := m.renderOpsPanel()

	row := lipgloss.JoinHorizontal(lipgloss.Top, memPanel, opsPanel)

	return fmt.Sprintf("%s\n\n%s\n\npolls received: %d — press q to quit\n",
		titleStyle.Render("rekon"), row, m.pollCount)
}

func (m Model) renderMemoryPanel() string {
	status := m.memory.FragmentationStatus()

	ratioText := fmt.Sprintf("%.2f", m.memory.FragmentationRatio)
	if status == metrics.StatusInsufficientData {
		ratioText = fmt.Sprintf("%.2f (not enough data to judge, <%dMB used)",
			m.memory.FragmentationRatio, metrics.MinMeaningfulMemoryBytes/(1024*1024))
	}

	content := fmt.Sprintf(
		"Memory\nused: %d bytes\nfragmentation ratio: %s\nmaxmemory policy: %s\nevicted keys: %d",
		m.memory.UsedMemoryBytes,
		ratioText,
		valueOr(m.memory.MaxMemoryPolicy, "unknown"),
		m.memory.EvictedKeys,
	)
	return panelStyle(status).Render(content)
}

func (m Model) renderOpsPanel() string {
	status := m.ops.HitRatioStatus()
	ratio, ok := m.ops.HitRatio()
	ratioText := "no data yet"
	if ok {
		ratioText = fmt.Sprintf("%.1f%%", ratio*100)
	}
	content := fmt.Sprintf(
		"Ops\nops/sec: %d\nkeyspace hit ratio: %s\nhits: %d  misses: %d",
		m.ops.OpsPerSec, ratioText, m.ops.KeyspaceHits, m.ops.KeyspaceMisses,
	)
	return panelStyle(status).Render(content)
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
