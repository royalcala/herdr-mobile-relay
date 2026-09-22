// Package machines inventories the saved SSH machines a local Herdr knows about
// through the global `--machine` prefix. Remote panes have no local event stream,
// so the inventory is a periodically refreshed read-only mirror; every row is
// tagged with its machine ID and Remote, and never enters the local
// coordinator.State (which owns generations, attention, and mutations).
package machines

import (
	"context"
	"log/slog"
	"path/filepath"
	"reflect"
	"sync"
	"time"

	"github.com/0cv/herdr-mobile-relay/internal/coordinator"
	"github.com/0cv/herdr-mobile-relay/internal/herdr"
)

// DefaultInterval is slower than the local poll: each refresh costs three CLI
// round trips per machine over SSH.
const DefaultInterval = 10 * time.Second

const fetchTimeout = 12 * time.Second

// Snapshot is one saved machine's read-only inventory.
type Snapshot struct {
	ID         string
	Label      string
	Host       string
	Target     string
	Reachable  bool
	Error      string
	Agents     []*coordinator.AgentState
	Workspaces []herdr.Workspace
}

// Manager periodically refreshes the saved-machine inventory.
type Manager struct {
	client   *herdr.Client
	logger   *slog.Logger
	interval time.Duration
	onChange func() error

	mu        sync.RWMutex
	snapshots []Snapshot
	wake      chan struct{}
}

func NewManager(client *herdr.Client, logger *slog.Logger) *Manager {
	return &Manager{
		client:   client,
		logger:   logger,
		interval: DefaultInterval,
		wake:     make(chan struct{}, 1),
	}
}

// SetOnChange registers the callback that republishes the phone inventory when
// the mirror changes.
func (m *Manager) SetOnChange(fn func() error) { m.onChange = fn }

// Wake asks the run loop for an out-of-band refresh without waiting for the
// interval, mirroring the local poller's wake.
func (m *Manager) Wake() {
	if m == nil {
		return
	}
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

func (m *Manager) Run(ctx context.Context) {
	if m.client == nil {
		return
	}
	if err := m.Refresh(ctx); err != nil && ctx.Err() == nil {
		m.logger.Debug("machine inventory refresh failed", "error", err)
	}
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.Refresh(ctx); err != nil && ctx.Err() == nil {
				m.logger.Debug("machine inventory refresh failed", "error", err)
			}
		case <-m.wake:
			if err := m.Refresh(ctx); err != nil && ctx.Err() == nil {
				m.logger.Debug("machine inventory refresh failed", "error", err)
			}
		}
	}
}

// Refresh re-reads the machine list and each enabled machine's inventory,
// notifying the change callback when the mirror differs from the last snapshot.
func (m *Manager) Refresh(ctx context.Context) error {
	if m.client == nil {
		return nil
	}
	list, err := m.client.MachineList(ctx)
	if err != nil {
		return err
	}
	snapshots := make([]Snapshot, 0, len(list))
	for _, machine := range list {
		if !machine.Enabled {
			continue
		}
		snapshots = append(snapshots, m.fetch(ctx, machine))
	}
	m.mu.Lock()
	changed := !snapshotsEqual(m.snapshots, snapshots)
	m.snapshots = snapshots
	m.mu.Unlock()
	if changed && m.onChange != nil {
		return m.onChange()
	}
	return nil
}

// Snapshot returns a copy of the current mirror.
func (m *Manager) Snapshot() []Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]Snapshot(nil), m.snapshots...)
}

// Agent finds a mirrored agent by machine and per-server pane ID.
func (m *Manager) Agent(machineID, paneID string) (coordinator.AgentState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, snapshot := range m.snapshots {
		if snapshot.ID != machineID {
			continue
		}
		for _, agent := range snapshot.Agents {
			if agent.PaneID == paneID {
				return *agent, true
			}
		}
	}
	return coordinator.AgentState{}, false
}

func (m *Manager) fetch(ctx context.Context, machine herdr.Machine) Snapshot {
	snapshot := Snapshot{
		ID:     machine.ID,
		Label:  machine.Label,
		Host:   machine.Label,
		Target: machine.Target,
	}
	fetchCtx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	panes, err := m.client.MachineAgentList(fetchCtx, machine.ID)
	if err != nil {
		return unreachableSnapshot(snapshot, err)
	}
	workspaces, err := m.client.MachineWorkspaceList(fetchCtx, machine.ID)
	if err != nil {
		return unreachableSnapshot(snapshot, err)
	}
	tabs, tabErr := m.client.MachineTabList(fetchCtx, machine.ID)
	if tabErr != nil {
		m.logger.Debug("machine tab list failed", "machine", machine.ID, "error", tabErr)
	}
	snapshot.Reachable = true
	snapshot.Agents = agentsFromPanes(machine.ID, snapshot.Host, panes, tabs)
	snapshot.Workspaces = agentWorkspaces(snapshot.Agents, workspaces)
	return snapshot
}

// unreachableSnapshot reports a failed fetch. It keeps no inventory: the last
// good agents and workspaces would otherwise survive as ghost panes for as long
// as the machine stays down, and a mirrored row the phone cannot re-verify is
// worse than a dimmed, empty machine.
func unreachableSnapshot(snapshot Snapshot, err error) Snapshot {
	snapshot.Error = err.Error()
	return snapshot
}

// agentWorkspaces keeps only the workspaces that host a mirrored agent. The
// mirror exposes agent sessions, so a workspace whose panes are all shells (or
// whose agents have exited) is not part of it: serving it would hand the phone
// an empty group that no agent can fill.
func agentWorkspaces(agents []*coordinator.AgentState, workspaces []herdr.Workspace) []herdr.Workspace {
	if len(workspaces) == 0 {
		return nil
	}
	hosted := make(map[string]bool, len(agents))
	for _, agent := range agents {
		if agent.WorkspaceID != "" {
			hosted[agent.WorkspaceID] = true
		}
	}
	kept := make([]herdr.Workspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		if hosted[workspace.ID] {
			kept = append(kept, workspace)
		}
	}
	return kept
}

func snapshotsEqual(a, b []Snapshot) bool {
	return reflect.DeepEqual(a, b)
}

// agentsFromPanes mirrors the local topology mapping, but tags every row with
// its machine and Remote so the phone renders it read-only and namespaces the
// per-server pane IDs.
func agentsFromPanes(machineID, host string, panes []herdr.Pane, tabs []herdr.Tab) []*coordinator.AgentState {
	tabByID := make(map[string]herdr.Tab, len(tabs))
	tabOrderByID := make(map[string]int, len(tabs))
	perWorkspace := make(map[string]int)
	for index, tab := range tabs {
		if tab.Number == 0 {
			tab.Number = index + 1
		}
		tabByID[tab.ID] = tab
		perWorkspace[tab.WorkspaceID]++
		tabOrderByID[tab.ID] = perWorkspace[tab.WorkspaceID]
	}

	agents := make([]*coordinator.AgentState, 0, len(panes))
	for _, pane := range panes {
		if pane.Agent == "" {
			continue
		}
		if tab, ok := tabByID[pane.TabID]; ok {
			pane.TabLabel = tab.Label
			pane.TabNumber = tab.Number
		}
		project := ""
		if pane.Cwd != "" {
			project = filepath.Base(pane.Cwd)
		}
		agents = append(agents, &coordinator.AgentState{
			PaneID:          pane.ID,
			RawPaneID:       pane.ID,
			MachineID:       machineID,
			Remote:          true,
			ServerSessionID: "primary",
			TerminalID:      pane.TerminalID,
			TabID:           pane.TabID,
			TabLabel:        pane.TabLabel,
			TabNumber:       pane.TabNumber,
			TabOrder:        tabOrderByID[pane.TabID],
			WorkspaceID:     pane.WorkspaceID,
			Agent:           pane.Agent,
			Name:            pane.Name,
			Status:          pane.Status,
			Focused:         pane.Focused,
			Cwd:             pane.Cwd,
			Project:         project,
			Host:            host,
			Session:         pane.Session,
			ActivitySeq:     pane.StateChangeSeq,
			PaneRevision:    pane.Revision,
			ForegroundCwd:   pane.ForegroundCwd,
		})
	}
	return agents
}
