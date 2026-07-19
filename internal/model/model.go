// Package model defines Rekon's bubbletea Model, Update, and View.
//
// Incoming poller.Snapshot values arrive as just another kind of
// bubbletea message (tea.Msg) — the same category as a keypress — and
// Update reacts to them by producing a new Model, which View then
// renders. No shared mutable state between the polling goroutine and
// the UI: the channel from internal/poller is the only handoff, exactly
// as designed in ARCHITECTURE.md section 2.
package model

import (
	"fmt"
	"strings"
	"time"

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
	selfAddr  string
	connected bool
	lastErr   error
	pollCount int

	width  int
	height int

	memory      metrics.Memory
	ops         metrics.Ops
	clients     metrics.Clients
	replication metrics.Replication
	persistence metrics.Persistence

	longIdleClients []metrics.ClientRecord
	clientListErr   error

	allSlowlogEntries  []redis.SlowlogEntry
	newSlowlogIDs      map[int64]bool
	slowlogLastSeenID  int64
	slowlogHasBaseline bool // false until the first successful slowlog poll — see Update's snapshotMsg case for why this matters
	slowlogErr         error
}

// New creates a Model that will listen on results for incoming
// snapshots. selfAddr is Rekon's own connection's local address (from
// redis.Client.LocalAddr()), used to filter Rekon's own polling
// commands out of the Slowlog panel — see metrics.FilterOutSelf.
func New(results <-chan poller.Snapshot, selfAddr string) Model {
	return Model{
		results:  results,
		selfAddr: selfAddr,
	}
}

// waitForSnapshot returns a tea.Cmd that blocks on the results channel
// and turns whatever arrives into a tea.Msg. A tea.Cmd fires once per
// call: if this isn't re-issued after every snapshot, Rekon would
// render exactly one poll result and permanently stop updating, even
// though the poller keeps ticking forever with nowhere for its results
// to go.
func waitForSnapshot(results <-chan poller.Snapshot) tea.Cmd {
	return func() tea.Msg {
		snap, ok := <-results
		if !ok {
			return closedMsg{}
		}
		return snapshotMsg(snap)
	}
}

// Init is called once when the program starts.
func (m Model) Init() tea.Cmd {
	return waitForSnapshot(m.results)
}

// Update reacts to one message and returns the next Model plus
// optionally another Cmd to run.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case snapshotMsg:
		m.pollCount++

		if msg.InfoErr != nil {
			m.connected = false
			m.lastErr = msg.InfoErr
		} else {
			m.connected = true
			m.lastErr = nil
			info := redis.ParseInfo(msg.Info)
			m.memory = metrics.ParseMemory(info)
			m.ops = metrics.ParseOps(info)
			m.clients = metrics.ParseClients(info)
			m.replication = metrics.ParseReplication(info)
			m.persistence = metrics.ParsePersistence(info)
		}

		// Clients panel (CLIENT LIST) is independent of INFO's success —
		// an ACL restricting one command shouldn't blank out a panel
		// that only needed the other.
		if msg.ClientListErr != nil {
			m.clientListErr = msg.ClientListErr
		} else {
			m.clientListErr = nil
			records := metrics.ParseClientList(msg.ClientListRaw)
			m.longIdleClients = metrics.LongIdleClients(records)
		}

		// Slowlog panel. The first successful poll is treated as a
		// baseline, not a flood of "new" entries — a slowlog can
		// legitimately already contain entries from before Rekon
		// started watching, and marking all of them "new" on startup
		// would be misleading (implying they just happened). Only
		// entries appearing from the *second* successful poll onward,
		// with IDs higher than what the baseline already saw, count
		// as new.
		if msg.SlowlogErr != nil {
			m.slowlogErr = msg.SlowlogErr
		} else {
			m.slowlogErr = nil
			m.allSlowlogEntries = metrics.FilterOutSelf(msg.SlowlogEntries, m.selfAddr)

			if !m.slowlogHasBaseline {
				_, maxID := metrics.NewEntriesSince(m.allSlowlogEntries, 0)
				m.slowlogLastSeenID = maxID
				m.slowlogHasBaseline = true
				m.newSlowlogIDs = map[int64]bool{}
			} else {
				newEntries, maxID := metrics.NewEntriesSince(m.allSlowlogEntries, m.slowlogLastSeenID)
				m.slowlogLastSeenID = maxID
				m.newSlowlogIDs = make(map[int64]bool, len(newEntries))
				for _, e := range newEntries {
					m.newSlowlogIDs[e.ID] = true
				}
			}
		}

		// Keep listening for the next snapshot — see waitForSnapshot's
		// doc comment for why this re-issue is required every time.
		return m, waitForSnapshot(m.results)

	case closedMsg:
		m.connected = false
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
// renders the decision.
var (
	panelBorder = lipgloss.RoundedBorder()

	statusColor = map[metrics.Status]lipgloss.Color{
		metrics.StatusOK:               lipgloss.Color("42"),  // green
		metrics.StatusWarn:             lipgloss.Color("214"), // orange
		metrics.StatusCritical:         lipgloss.Color("196"), // red
		metrics.StatusUnknown:          lipgloss.Color("240"), // grey
		metrics.StatusInsufficientData: lipgloss.Color("240"), // grey, same as unknown — neither is an alarm color
	}

	titleStyle  = lipgloss.NewStyle().Bold(true)
	newTagStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// Layout constants. panelChromeWidth is how many columns a panel adds
// beyond its content width: a 1-char border on each side (2) plus
// Padding(0,1) adding 1 column on each side (2) = 4 total.
const (
	panelChromeWidth       = 4
	columnGap              = 2
	twoColumnContentCap    = 34 // don't let panels get silly-wide on huge terminals
	minReadableContentW    = 24
	singleColumnContentCap = 60
	defaultTerminalWidth   = 80 // fallback used only before the first WindowSizeMsg arrives
)

// panelLayout decides panel content width and whether panels render as
// a 2-column grid or a single stacked column, based on the real
// terminal width bubbletea reported via WindowSizeMsg.
//
// This exists because panels previously had zero awareness of terminal
// size — width was driven purely by each panel's own content, so
// anything narrower than the combined width just broke visually
// (lines wrapped mid-content by the terminal itself, boxes
// misaligned). Real bug, found from an actual screenshot of a narrow
// window.
func (m Model) panelLayout() (contentWidth int, twoColumn bool) {
	w := m.width
	if w <= 0 {
		w = defaultTerminalWidth // before the first WindowSizeMsg arrives
	}

	twoColContent := (w - columnGap - 2*panelChromeWidth) / 2
	if twoColContent >= minReadableContentW {
		if twoColContent > twoColumnContentCap {
			twoColContent = twoColumnContentCap
		}
		return twoColContent, true
	}

	singleContent := w - panelChromeWidth
	if singleContent > singleColumnContentCap {
		singleContent = singleColumnContentCap
	}
	if singleContent < minReadableContentW {
		singleContent = minReadableContentW // best effort even on a tiny terminal
	}
	return singleContent, false
}

func panelStyle(status metrics.Status, width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(panelBorder).
		BorderForeground(statusColor[status]).
		Padding(0, 1).
		Width(width).
		MarginRight(columnGap)
}

// truncateToken hard-truncates a single unbroken token (no spaces to
// wrap on) to maxLen, appending "..." if it was cut. This exists
// specifically for Slowlog panel command args: a value like a large
// base64 blob or JSON payload can be one enormous unbroken "word" with
// no spaces at all, which lipgloss's word-wrap can't break cleanly —
// it would otherwise blow out the panel's fixed width regardless of
// the width fix above.
func truncateToken(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// View renders the current Model to text.
func (m Model) View() string {
	if m.lastErr != nil {
		return fmt.Sprintf("rekon\n\nconnection error: %v\nretrying automatically...\n\npress q to quit\n", m.lastErr)
	}
	if !m.connected {
		return "rekon\n\nwaiting for first poll...\n\npress q to quit\n"
	}

	width, twoColumn := m.panelLayout()

	memPanel := m.renderMemoryPanel(width)
	opsPanel := m.renderOpsPanel(width)
	clientsPanel := m.renderClientsPanel(width)
	slowlogPanel := m.renderSlowlogPanel(width)
	replPanel := m.renderReplicationPanel(width)
	persistPanel := m.renderPersistencePanel(width)

	var body string
	if twoColumn {
		row1 := lipgloss.JoinHorizontal(lipgloss.Top, memPanel, opsPanel)
		row2 := lipgloss.JoinHorizontal(lipgloss.Top, clientsPanel, slowlogPanel)
		row3 := lipgloss.JoinHorizontal(lipgloss.Top, replPanel, persistPanel)
		body = lipgloss.JoinVertical(lipgloss.Left, row1, row2, row3)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Left,
			memPanel, opsPanel, clientsPanel, slowlogPanel, replPanel, persistPanel)
	}

	return fmt.Sprintf("%s\n\n%s\n\npolls received: %d — press q to quit\n",
		titleStyle.Render("rekon"), body, m.pollCount)
}

func (m Model) renderMemoryPanel(width int) string {
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
	return panelStyle(status, width).Render(content)
}

func (m Model) renderOpsPanel(width int) string {
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
	return panelStyle(status, width).Render(content)
}

func (m Model) renderClientsPanel(width int) string {
	if m.clientListErr != nil {
		content := fmt.Sprintf("Clients\nunavailable: %v", m.clientListErr)
		return panelStyle(metrics.StatusUnknown, width).Render(content)
	}

	status := metrics.StatusOK
	if len(m.longIdleClients) > 0 {
		status = metrics.StatusWarn
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Clients\nconnected: %d  blocked: %d\n", m.clients.Connected, m.clients.Blocked)
	if len(m.longIdleClients) == 0 {
		b.WriteString(dimStyle.Render("no long-idle connections"))
	} else {
		fmt.Fprintf(&b, "long-idle (>%ds):", metrics.LongIdleThresholdSeconds)
		for _, c := range m.longIdleClients {
			fmt.Fprintf(&b, "\n  %s idle=%ds", c.Addr, c.IdleSeconds)
		}
	}

	return panelStyle(status, width).Render(b.String())
}

func (m Model) renderSlowlogPanel(width int) string {
	if m.slowlogErr != nil {
		content := fmt.Sprintf("Slowlog\nunavailable: %v", m.slowlogErr)
		return panelStyle(metrics.StatusUnknown, width).Render(content)
	}

	status := metrics.StatusOK
	if len(m.newSlowlogIDs) > 0 {
		status = metrics.StatusWarn
	}

	var b strings.Builder
	b.WriteString("Slowlog\n")
	if len(m.allSlowlogEntries) == 0 {
		b.WriteString(dimStyle.Render("no entries"))
	} else {
		// Show at most the 5 most recent entries — Redis returns
		// newest-first already, so no re-sorting is needed.
		limit := 5
		if len(m.allSlowlogEntries) < limit {
			limit = len(m.allSlowlogEntries)
		}
		for i := 0; i < limit; i++ {
			e := m.allSlowlogEntries[i]
			tag := ""
			if m.newSlowlogIDs[e.ID] {
				tag = " " + newTagStyle.Render("NEW")
			}
			// Each arg is truncated individually before joining —
			// a single arg (e.g. a large value passed to SET) can be
			// one unbroken token with no spaces to wrap on, which
			// would otherwise blow out the panel's fixed width
			// regardless of lipgloss's normal word-wrapping.
			args := make([]string, len(e.Args))
			for j, a := range e.Args {
				args[j] = truncateToken(a, 40)
			}
			fmt.Fprintf(&b, "\n%dus %s%s", e.DurationMicros, strings.Join(args, " "), tag)
		}
	}

	return panelStyle(status, width).Render(b.String())
}

func (m Model) renderReplicationPanel(width int) string {
	r := m.replication

	if r.IsReplica() {
		status := r.ReplicaLinkStatus()
		lastIO := "unknown"
		if r.MasterLastIOSecondsAgo >= 0 {
			lastIO = fmt.Sprintf("%ds ago", r.MasterLastIOSecondsAgo)
		}
		content := fmt.Sprintf(
			"Replication\nrole: replica of %s:%s\nlink status: %s\nlast I/O: %s",
			valueOr(r.MasterHost, "unknown"), valueOr(r.MasterPort, "unknown"),
			valueOr(r.MasterLinkStatus, "unknown"), lastIO,
		)
		return panelStyle(status, width).Render(content)
	}

	// Master (which also covers a standalone instance with zero
	// connected replicas — Redis has no separate "standalone" role).
	var b strings.Builder
	fmt.Fprintf(&b, "Replication\nrole: master\nconnected replicas: %d", r.ConnectedSlaves)
	for _, s := range r.Slaves {
		fmt.Fprintf(&b, "\n  %s:%s state=%s lag=%ds", s.IP, s.Port, s.State, s.Lag)
	}
	return panelStyle(metrics.StatusOK, width).Render(b.String())
}

func (m Model) renderPersistencePanel(width int) string {
	p := m.persistence
	status := p.Status()

	lastSave := "never"
	if p.RDBLastSaveTime > 0 {
		elapsed := time.Since(time.Unix(p.RDBLastSaveTime, 0))
		lastSave = fmt.Sprintf("%s ago", elapsed.Round(time.Second))
	}

	aofLine := "disabled"
	if p.AOFEnabled {
		aofLine = fmt.Sprintf("enabled (last write: %s)", valueOr(p.AOFLastWriteStatus, "unknown"))
	}

	inProgress := ""
	if p.RDBBGSaveInProgress {
		inProgress = " (save in progress)"
	}
	if p.AOFRewriteInProgress {
		inProgress += " (AOF rewrite in progress)"
	}

	content := fmt.Sprintf(
		"Persistence\nlast RDB save: %s\nRDB last save status: %s%s\nchanges since last save: %d\nAOF: %s",
		lastSave, valueOr(p.RDBLastBGSaveStatus, "unknown"), inProgress,
		p.RDBChangesSinceLastSave, aofLine,
	)
	return panelStyle(status, width).Render(content)
}

func valueOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
