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
      printf '%s\n' '{"result":{"type":"agent_list","agents":[{"pane_id":"w2:p2","terminal_id":"term-1","tab_id":"w2:t2","workspace_id":"w2","agent":"cmd","agent_status":"blocked","cwd":"/srv/dev/1us","state_change_seq":92}]}}'
      ;;
    "workspace list")
      printf '%s\n' '{"result":{"type":"workspace_list","workspaces":[{"workspace_id":"w2","label":"1us","number":1}]}}'
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
