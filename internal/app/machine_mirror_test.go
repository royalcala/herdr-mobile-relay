package app

import (
	"testing"

	"github.com/0cv/herdr-mobile-relay/internal/coordinator"
)

// Pane IDs are per-server: the local session and a saved machine can both host
// w1:p1. The merge must key rows by machine too, or the local row's revision
// replaces the remote row (whose mirrored revision is always 0) and the remote
// agent vanishes from the phone.
func TestMergeAgentSnapshotKeepsCollidingMachineRows(t *testing.T) {
	local := &coordinator.AgentState{
		PaneID: "w1:p1", RawPaneID: "w1:p1",
		Agent: "claude", Status: "working", StateRevision: 7,
	}
	remote := &coordinator.AgentState{
		PaneID: "w1:p1", RawPaneID: "w1:p1",
		MachineID: "m1", Remote: true,
		Agent: "cmd", Status: "idle", StateRevision: 0,
	}

	merged := mergeAgentSnapshot(
		[]*coordinator.AgentState{local},
		[]*coordinator.AgentState{local, remote},
	)

	if len(merged) != 2 {
		t.Fatalf("merged = %#v, want the local and the remote row", merged)
	}
	remotes := 0
	for _, agent := range merged {
		if agent.Remote && agent.MachineID == "m1" {
			remotes++
		}
	}
	if remotes != 1 {
		t.Fatalf("merged = %#v, want exactly one mirrored row for m1/w1:p1", merged)
	}
}
