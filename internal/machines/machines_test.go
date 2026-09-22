package machines

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/0cv/herdr-mobile-relay/internal/herdr"
)

const machineScript = `#!/bin/sh
if [ "$1" = "--machine" ]; then
  case "$3 $4" in
    "agent list")
      if [ "${FAKE_MACHINE_UNREACHABLE:-}" = "1" ]; then
        echo "ssh: connect to host server-1 port 22: Connection refused" >&2
        exit 255
      fi
      if [ "${FAKE_MACHINE_NO_AGENT:-}" = "1" ]; then
        printf '%s\n' '{"result":{"type":"agent_list","agents":[{"pane_id":"w2:p2","terminal_id":"term-1","tab_id":"w2:t2","workspace_id":"w2","agent":"","agent_status":"","cwd":"/srv/dev/1us"}]}}'
      else
        printf '%s\n' '{"result":{"type":"agent_list","agents":[{"pane_id":"w2:p2","terminal_id":"term-1","tab_id":"w2:t2","workspace_id":"w2","agent":"cmd","agent_status":"blocked","cwd":"/srv/dev/1us","state_change_seq":92}]}}'
      fi
      ;;
    "workspace list")
      if [ "${FAKE_MACHINE_EMPTY_WORKSPACE:-}" = "1" ]; then
        printf '%s\n' '{"result":{"type":"workspace_list","workspaces":[{"workspace_id":"w2","label":"1us","number":1},{"workspace_id":"w3","label":"shell-only","number":2}]}}'
      else
        printf '%s\n' '{"result":{"type":"workspace_list","workspaces":[{"workspace_id":"w2","label":"1us","number":1}]}}'
      fi
      ;;
    "tab list")
      printf '%s\n' '{"result":{"type":"tab_list","tabs":[{"tab_id":"w2:t2","workspace_id":"w2","label":"account-portal","number":2}]}}'
      ;;
  esac
  exit 0
fi
if [ "$1 $2" = "machine list" ]; then
  printf '%s\n' '[{"id":"m1","label":"server-1","enabled":true},{"id":"m2","label":"server-2","enabled":false}]'
  exit 0
fi
exit 1
`

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "herdr")
	if err := os.WriteFile(bin, []byte(machineScript), 0o755); err != nil {
		t.Fatalf("write herdr script: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewManager(herdr.NewClient(bin, filepath.Join(t.TempDir(), "sock")), logger)
}

func TestRefreshMirrorsEnabledMachines(t *testing.T) {
	manager := newTestManager(t)
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	snapshots := manager.Snapshot()
	if len(snapshots) != 1 {
		t.Fatalf("Snapshot() = %#v, want only the enabled machine", snapshots)
	}
	snapshot := snapshots[0]
	if snapshot.ID != "m1" || snapshot.Label != "server-1" || !snapshot.Reachable {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if len(snapshot.Workspaces) != 1 || len(snapshot.Agents) != 1 {
		t.Fatalf("snapshot inventory = %#v", snapshot)
	}
	agent := snapshot.Agents[0]
	if agent.MachineID != "m1" || !agent.Remote {
		t.Fatalf("agent machine tagging = %#v", agent)
	}
	if agent.PaneID != "w2:p2" || agent.WorkspaceID != "w2" || agent.TabLabel != "account-portal" {
		t.Fatalf("agent mapping = %#v", agent)
	}
	if agent.TabNumber != 2 || agent.Project != "1us" || agent.Host != "server-1" {
		t.Fatalf("agent tab/project mapping = %#v", agent)
	}
	if agent.ServerSessionID != "primary" {
		t.Fatalf("agent server session = %q, want primary", agent.ServerSessionID)
	}
}

func TestAgentLookupByMachineAndPane(t *testing.T) {
	manager := newTestManager(t)
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if agent, ok := manager.Agent("m1", "w2:p2"); !ok || agent.PaneID != "w2:p2" {
		t.Fatalf("Agent(m1, w2:p2) = %#v, %v", agent, ok)
	}
	if _, ok := manager.Agent("m1", "w9:p9"); ok {
		t.Fatal("Agent(m1, unknown) matched unexpectedly")
	}
	if _, ok := manager.Agent("m2", "w2:p2"); ok {
		t.Fatal("Agent(disabled machine) matched unexpectedly")
	}
}

// A pane can outlive its agent: the tab keeps running a shell. The mirror must
// drop the row on the next refresh instead of leaving it as a ghost.
func TestRefreshDropsPaneThatLostItsAgent(t *testing.T) {
	manager := newTestManager(t)
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if initial := manager.Snapshot()[0]; len(initial.Agents) != 1 || len(initial.Workspaces) != 1 {
		t.Fatalf("initial mirror = %#v, want one agent and one workspace", initial)
	}

	t.Setenv("FAKE_MACHINE_NO_AGENT", "1")
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() after the agent exited error = %v", err)
	}
	snapshot := manager.Snapshot()[0]
	if !snapshot.Reachable {
		t.Fatalf("machine should stay reachable: %#v", snapshot)
	}
	if len(snapshot.Agents) != 0 {
		t.Fatalf("mirror kept a ghost agent: %#v", snapshot.Agents)
	}
	if len(snapshot.Workspaces) != 0 {
		t.Fatalf("mirror kept a workspace with no agents: %#v", snapshot.Workspaces)
	}
	if _, ok := manager.Agent("m1", "w2:p2"); ok {
		t.Fatal("Agent(m1, w2:p2) still resolvable after the pane lost its agent")
	}
}

// A machine can report workspaces whose panes are all shells. Those are not
// agent sessions, so they must not reach the phone as empty groups.
func TestRefreshDropsWorkspaceWithoutAgents(t *testing.T) {
	t.Setenv("FAKE_MACHINE_EMPTY_WORKSPACE", "1")
	manager := newTestManager(t)
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	snapshot := manager.Snapshot()[0]
	if len(snapshot.Agents) != 1 || snapshot.Agents[0].WorkspaceID != "w2" {
		t.Fatalf("agents = %#v", snapshot.Agents)
	}
	if len(snapshot.Workspaces) != 1 || snapshot.Workspaces[0].ID != "w2" {
		t.Fatalf("mirrored workspaces = %#v, want only the agent-hosting w2", snapshot.Workspaces)
	}
}

// When a machine cannot be reached, the last good inventory is unverifiable.
// Serving it keeps panes from lost sessions on the phone, so the mirror must
// drop them rather than let them linger.
func TestRefreshUnreachableMachineDropsGhostInventory(t *testing.T) {
	manager := newTestManager(t)
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if len(manager.Snapshot()[0].Agents) != 1 {
		t.Fatalf("initial mirror = %#v", manager.Snapshot())
	}

	t.Setenv("FAKE_MACHINE_UNREACHABLE", "1")
	if err := manager.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() while unreachable error = %v", err)
	}
	snapshot := manager.Snapshot()[0]
	if snapshot.Reachable {
		t.Fatalf("snapshot should be unreachable: %#v", snapshot)
	}
	if snapshot.Error == "" {
		t.Fatal("unreachable snapshot should carry the failure")
	}
	if len(snapshot.Agents) != 0 || len(snapshot.Workspaces) != 0 {
		t.Fatalf("unreachable machine kept ghosts: %#v", snapshot)
	}
	if _, ok := manager.Agent("m1", "w2:p2"); ok {
		t.Fatal("Agent(m1, w2:p2) still resolvable while the machine is unreachable")
	}
}
