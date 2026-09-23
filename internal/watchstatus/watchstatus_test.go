package watchstatus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testdata/manager-watch.status is a verbatim copy of the watchdog's state file
// on the machine, so this parser is tested against what it actually writes:
// plain key=value lines, one scope.<name>.<field> family per scope.
func TestLoadReadsTheRealStateFile(t *testing.T) {
	status, err := Load(filepath.Join("testdata", "manager-watch.status"), 0)
	if err != nil {
		t.Fatal(err)
	}
	// Only the fields that do not move between passes are pinned: the counters
	// and the heartbeat advance while the watchdog runs.
	for key, want := range map[string]string{
		"pid":                   "1980822",
		"manager":               "manager-v3",
		"interval":              "20",
		"scope.local.result":    "ok",
		"scope.server-1.result": "ok",
		"scope.server-2.result": "ok",
	} {
		if got := status.Fields[key]; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
	for _, key := range []string{"passes", "queue", "heartbeat_epoch", "started_epoch"} {
		if status.Fields[key] == "" {
			t.Errorf("%s was not read", key)
		}
	}
	if status.Missing() {
		t.Fatal("a state file with a pid is not missing")
	}
}

func TestLoadTakesTheTailOfTheLogItNames(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "manager-watch.log")
	lines := []string{}
	for i := 1; i <= 12; i++ {
		lines = append(lines, "line "+string(rune('0'+i%10)))
	}
	if err := os.WriteFile(logPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(dir, "manager-watch.status")
	if err := os.WriteFile(statePath, []byte("pid=1\nlog="+logPath+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	status, err := Load(statePath, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Log) != 3 {
		t.Fatalf("log tail = %d lines, want 3", len(status.Log))
	}
	if status.Log[2] != lines[11] {
		t.Fatalf("the newest line is not last: %#v", status.Log)
	}
}

// A log that is gone must not take the status with it: the watchdog may rotate.
func TestLoadSurvivesAMissingLog(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "manager-watch.status")
	if err := os.WriteFile(statePath, []byte("pid=7\nlog="+filepath.Join(dir, "gone.log")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	status, err := Load(statePath, 5)
	if err != nil {
		t.Fatal(err)
	}
	if status.Fields["pid"] != "7" {
		t.Fatalf("pid = %q, want 7", status.Fields["pid"])
	}
	if len(status.Log) != 0 {
		t.Fatalf("log = %#v, want nothing", status.Log)
	}
}

// An absent state file is reported as such, not as an empty watchdog.
func TestLoadReportsAMissingStateFile(t *testing.T) {
	status, err := Load(filepath.Join(t.TempDir(), "manager-watch.status"), 5)
	if err == nil {
		t.Fatal("a missing state file must be reported")
	}
	if !status.Missing() {
		t.Fatal("a status with no pid is missing")
	}
}

func TestSignatureFollowsTheContent(t *testing.T) {
	first := Status{Fields: map[string]string{"pid": "1", "passes": "2"}}
	same := Status{Fields: map[string]string{"passes": "2", "pid": "1"}}
	later := Status{Fields: map[string]string{"pid": "1", "passes": "3"}}

	if Signature(first) != Signature(same) {
		t.Fatal("the signature must not depend on map order")
	}
	if Signature(first) == Signature(later) {
		t.Fatal("a new pass must change the signature")
	}
	withLog := Status{Fields: first.Fields, Log: []string{"entregado: relay-pwa-ui"}}
	if Signature(withLog) == Signature(first) {
		t.Fatal("a new event must change the signature")
	}
}
