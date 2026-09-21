package herdr

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

const machineScript = `#!/bin/sh
if [ "$1" = "--machine" ]; then
  case "$3 $4" in
    "agent list")
      printf '%s\n' '{"id":"cli:agent:list","result":{"type":"agent_list","agents":[{"pane_id":"w2:p2","terminal_id":"term-1","tab_id":"w2:t2","workspace_id":"w2","agent":"cmd","agent_status":"blocked","cwd":"/srv/dev/1us","state_change_seq":92}]}}'
      ;;
    "workspace list")
      printf '%s\n' '{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[{"workspace_id":"w2","label":"1us","number":1,"pane_count":1,"tab_count":1}]}}'
      ;;
    "tab list")
      printf '%s\n' '{"id":"cli:tab:list","result":{"type":"tab_list","tabs":[{"tab_id":"w2:t2","workspace_id":"w2","label":"account-portal","number":2}]}}'
      ;;
    "pane read")
      printf 'remote pane text\n'
      ;;
  esac
  exit 0
fi
if [ "$1 $2" = "machine list" ]; then
  printf '%s\n' '[{"id":"m1","label":"server-1","target":"ssh://dev@192.168.1.10","session":"default","enabled":true},{"id":"m2","label":"server-2","enabled":false}]'
  exit 0
fi
exit 1
`

func writeMachineScript(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "herdr")
	if err := os.WriteFile(bin, []byte(machineScript), 0o755); err != nil {
		t.Fatalf("write herdr script: %v", err)
	}
	return bin
}

func TestMachineListParsesBareArray(t *testing.T) {
	client := NewClient(writeMachineScript(t), filepath.Join(t.TempDir(), "sock"))

	machines, err := client.MachineList(context.Background())
	if err != nil {
		t.Fatalf("MachineList() error = %v", err)
	}
	if len(machines) != 2 {
		t.Fatalf("MachineList() = %#v, want 2 machines", machines)
	}
	if machines[0].ID != "m1" || machines[0].Label != "server-1" || !machines[0].Enabled {
		t.Fatalf("machine[0] = %#v", machines[0])
	}
	if machines[1].Enabled {
		t.Fatalf("machine[1] = %#v, want disabled", machines[1])
	}
}

func TestDecodeMachineListToleratesEnvelope(t *testing.T) {
	machines, err := decodeMachineList([]byte(`{"result":{"machines":[{"id":"m1","label":"server-1"}]}}`))
	if err != nil {
		t.Fatalf("decodeMachineList(envelope) error = %v", err)
	}
	if len(machines) != 1 || machines[0].ID != "m1" {
		t.Fatalf("decodeMachineList(envelope) = %#v", machines)
	}
	empty, err := decodeMachineList(nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("decodeMachineList(empty) = %#v, %v", empty, err)
	}
}

func TestMachineInventoryUsesMachinePrefix(t *testing.T) {
	client := NewClient(writeMachineScript(t), filepath.Join(t.TempDir(), "sock"))

	agents, err := client.MachineAgentList(context.Background(), "m1")
	if err != nil {
		t.Fatalf("MachineAgentList() error = %v", err)
	}
	if len(agents) != 1 || agents[0].ID != "w2:p2" || agents[0].Status != "blocked" {
		t.Fatalf("MachineAgentList() = %#v", agents)
	}

	workspaces, err := client.MachineWorkspaceList(context.Background(), "m1")
	if err != nil {
		t.Fatalf("MachineWorkspaceList() error = %v", err)
	}
	if len(workspaces) != 1 || workspaces[0].ID != "w2" {
		t.Fatalf("MachineWorkspaceList() = %#v", workspaces)
	}

	tabs, err := client.MachineTabList(context.Background(), "m1")
	if err != nil {
		t.Fatalf("MachineTabList() error = %v", err)
	}
	if len(tabs) != 1 || tabs[0].ID != "w2:t2" {
		t.Fatalf("MachineTabList() = %#v", tabs)
	}
}

func TestReadPaneMachineReadsViaCLI(t *testing.T) {
	client := NewClient(writeMachineScript(t), filepath.Join(t.TempDir(), "sock"))

	read, err := client.ReadPaneMachine(context.Background(), "m1", "w2:p2", 5, "text", "recent-unwrapped")
	if err != nil {
		t.Fatalf("ReadPaneMachine() error = %v", err)
	}
	if string(read.Content) != "remote pane text\n" {
		t.Fatalf("ReadPaneMachine() content = %q", read.Content)
	}
}
