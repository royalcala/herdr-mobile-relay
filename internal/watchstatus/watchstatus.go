// Package watchstatus reads the watchdog's own state files.
//
// The watchdog (manager-watch) writes <state dir>/manager-watch.status as
// key=value lines and appends human-readable events to its log. The relay does
// not own that format: it reads the pairs as they are and hands them to the
// phone, so a new key on the watchdog's side shows up without a relay change.
package watchstatus

import (
	"bufio"
	"maps"
	"os"
	"slices"
	"strings"
)

// Status is one reading of the watchdog's state file.
type Status struct {
	// Fields is every key=value pair, verbatim: pid, manager, interval, passes,
	// queue, heartbeat_epoch, started_epoch, log, last_delivery_*, and one
	// scope.<name>.<field> family per scope.
	Fields map[string]string
	// Log is the tail of the watchdog's log, oldest line first.
	Log []string
}

// Missing reports whether the state file was there at all. A watchdog that has
// never run is different from one that stopped.
func (s Status) Missing() bool {
	_, present := s.Fields["pid"]
	return !present
}

// Load reads the state file, then the tail of the log it names.
func Load(path string, logLines int) (Status, error) {
	status := Status{Fields: map[string]string{}}
	raw, err := os.ReadFile(path)
	if err != nil {
		return status, err
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		status.Fields[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if logPath := status.Fields["log"]; logPath != "" && logLines > 0 {
		status.Log = tail(logPath, logLines)
	}
	return status, nil
}

// tail returns the last n non-empty lines, oldest first. A log that cannot be
// read is not an error: the status file still describes the watchdog.
func tail(path string, n int) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	ring := make([]string, 0, n)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(ring) == n {
			ring = ring[1:]
		}
		ring = append(ring, line)
	}
	return ring
}

// Signature changes when the status file changes, so a watcher can broadcast
// only real movement.
func Signature(status Status) string {
	var builder strings.Builder
	for _, key := range slices.Sorted(maps.Keys(status.Fields)) {
		builder.WriteString(key)
		builder.WriteByte('=')
		builder.WriteString(status.Fields[key])
		builder.WriteByte('\n')
	}
	for _, line := range status.Log {
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}
