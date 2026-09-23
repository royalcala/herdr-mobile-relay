package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0cv/herdr-mobile-relay/internal/coordinator"
)

// The strings below are the ones the real systems produce, captured from a live
// reproduction: herdr's own refusal after `herdr session stop main` deleted the
// socket, herdr's answer for a pane that is gone, and the relay's generation
// check. They are the contract this classification exists for.
func TestPaneIsStaleRecognisesRealSessionLosses(t *testing.T) {
	server := &Server{}
	cases := []struct {
		name   string
		result *coordinator.CommandResult
		want   bool
	}{
		{
			name:   "relay generation check",
			result: &coordinator.CommandResult{PaneID: "w1:p1", Phase: "failed", Error: "pane session was replaced"},
			want:   true,
		},
		{
			name: "herdr after the session socket was removed",
			result: &coordinator.CommandResult{
				PaneID: "w1:p5", Phase: "failed",
				Error: "server_not_running: no herdr server is running at /home/me/.config/herdr/sessions/main/herdr.sock; run `herdr session attach main` to start or attach it",
			},
			want: true,
		},
		{
			name:   "herdr with the pane gone",
			result: &coordinator.CommandResult{PaneID: "w1:p2", Phase: "failed", Error: "pane_not_found: pane w1:p2 not found"},
			want:   true,
		},
		{
			name:   "an ordinary refusal is left alone",
			result: &coordinator.CommandResult{PaneID: "w1:p1", Phase: "failed", Error: "One or more attachments are no longer available for this agent"},
			want:   false,
		},
		{
			name:   "a delivered command is not stale",
			result: &coordinator.CommandResult{PaneID: "w1:p1", OK: true, Phase: "completed"},
			want:   false,
		},
		{
			// The outcome is genuinely unknown: rewording it would hide that.
			name:   "an unknown outcome keeps its own words",
			result: &coordinator.CommandResult{PaneID: "w1:p1", Phase: "dispatched_unknown", Error: "command was dispatched before the pane session was replaced; outcome is unknown"},
			want:   false,
		},
		{
			name:   "a failure that names no pane is not a stale pane",
			result: &coordinator.CommandResult{Phase: "failed", Error: "pane session was replaced"},
			want:   false,
		},
		{name: "nothing to classify", result: nil, want: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := server.paneIsStale(testCase.result); got != testCase.want {
				t.Fatalf("paneIsStale(%+v) = %v, want %v", testCase.result, got, testCase.want)
			}
		})
	}
}

func TestStalePaneResultReplacesHerdrWordingAndAsksForARefresh(t *testing.T) {
	original := &coordinator.CommandResult{
		RequestID: "req-1",
		Action:    "prompt",
		PaneID:    "w1:p5",
		Phase:     "failed",
		Error:     "server_not_running: no herdr server is running at /home/me/.config/herdr/sessions/main/herdr.sock",
		Data:      map[string]any{"attempt": 1},
	}
	result := stalePaneResult(original)

	if result.OK {
		t.Fatal("a stale pane result must stay a failure")
	}
	if result.RequestID != "req-1" || result.Action != "prompt" || result.PaneID != "w1:p5" {
		t.Fatalf("the result lost its identity: %+v", result)
	}
	if strings.Contains(strings.ToLower(result.Error), "server_not_running") || strings.Contains(result.Error, "herdr session attach") {
		t.Fatalf("herdr's own wording survived: %q", result.Error)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("data = %#v, want a map", result.Data)
	}
	if data["code"] != "pane_stale" || data["refresh"] != true {
		t.Fatalf("data = %#v, want code pane_stale and refresh true", data)
	}
	if data["attempt"] != 1 {
		t.Fatalf("the original data was dropped: %#v", data)
	}
}

// A replaced herdr session is a new socket file at the same path, which is what
// the watcher looks for. A session that stops and does not come back leaves
// nothing to stat at all.
func TestSocketIdentityNoticesAReplacedSocket(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "herdr.sock")
	if err := os.WriteFile(socket, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}
	first := socketIdentity(socket)
	if first == "missing" || first == "unset" {
		t.Fatalf("socketIdentity = %q, want a real identity", first)
	}
	if again := socketIdentity(socket); again != first {
		t.Fatalf("socketIdentity is not stable: %q then %q", first, again)
	}

	if err := os.Remove(socket); err != nil {
		t.Fatal(err)
	}
	if stopped := socketIdentity(socket); stopped != "missing" {
		t.Fatalf("socketIdentity of a removed socket = %q, want missing", stopped)
	}

	if err := os.WriteFile(socket, []byte("second-version"), 0o600); err != nil {
		t.Fatal(err)
	}
	if replaced := socketIdentity(socket); replaced == first || replaced == "missing" {
		t.Fatalf("a replaced socket kept its identity: %q", replaced)
	}
	if unset := socketIdentity("  "); unset != "unset" {
		t.Fatalf("socketIdentity of no path = %q, want unset", unset)
	}
}
