package app

import (
	"path/filepath"
	"testing"

	"github.com/0cv/herdr-mobile-relay/internal/protocol"
)

// The fields the phone's panel reads, verified against the watchdog's real state
// file (a verbatim copy lives beside the watchstatus tests) rather than against
// a shape invented here.
func TestWatchPayloadCarriesTheRealWatchdogState(t *testing.T) {
	server := testServer()
	server.cfg.WatchStatusPath = filepath.Join("..", "watchstatus", "testdata", "manager-watch.status")

	payload := server.watchPayload()
	if payload["available"] != true {
		t.Fatalf("available = %v, want true (%v)", payload["available"], payload["reason"])
	}
	fields, ok := payload["fields"].(map[string]string)
	if !ok {
		t.Fatalf("fields = %#v, want the key=value pairs", payload["fields"])
	}
	for _, key := range []string{
		"pid", "manager", "interval", "passes", "queue", "heartbeat_epoch", "started_epoch",
		"scope.local.result", "scope.server-1.result", "scope.server-2.result",
	} {
		if fields[key] == "" {
			t.Errorf("the payload dropped %s", key)
		}
	}
	if fields["manager"] == "" {
		t.Error("the panel cannot tell who the manager is")
	}
}

// A machine with no watchdog says so, instead of showing an empty panel.
func TestWatchPayloadReportsAMissingWatchdog(t *testing.T) {
	server := testServer()
	server.cfg.WatchStatusPath = filepath.Join(t.TempDir(), "manager-watch.status")

	payload := server.watchPayload()
	if payload["available"] != false {
		t.Fatalf("available = %v, want false", payload["available"])
	}
	if payload["reason"] == "" {
		t.Error("a missing watchdog must come with a reason")
	}
}

// The command is read-only: a reader-role phone may ask for it.
func TestWatchdogStatusIsAReadAction(t *testing.T) {
	metadata, ok := protocol.ClassifyAction("watchdog_status")
	if !ok {
		t.Fatal("watchdog_status is not a declared action")
	}
	if metadata.Class != protocol.ActionReadOnly {
		t.Fatalf("class = %v, want read-only", metadata.Class)
	}
}
