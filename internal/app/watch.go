package app

import (
	"strings"

	"github.com/0cv/herdr-mobile-relay/internal/watchstatus"
)

// watchLogLines is how much of the watchdog's log the phone gets: enough to see
// what it last did, not the whole history.
const watchLogLines = 40

// watchStatus reads the watchdog's own state, plus the tail of its log. A
// missing file is not an error the phone should see as a broken panel: it means
// the watchdog has not run yet, and that is worth saying plainly.
func (s *Server) watchStatus() (watchstatus.Status, string, bool) {
	path := strings.TrimSpace(s.cfg.WatchStatusPath)
	if path == "" {
		return watchstatus.Status{}, "", false
	}
	status, err := watchstatus.Load(path, watchLogLines)
	if err != nil {
		return watchstatus.Status{}, path, false
	}
	return status, path, true
}

// watchPayload is the watchdog's state as the phone needs it: the key=value
// pairs it writes (heartbeat, passes, queue depth, per-scope results) plus the
// tail of its log. The relay does not reinterpret the keys — a new one on the
// watchdog's side shows up on the phone without a relay change.
func (s *Server) watchPayload() map[string]any {
	status, path, ok := s.watchStatus()
	if !ok {
		reason := "the watchdog has not written its state file"
		if path == "" {
			reason = "no watchdog path configured"
		}
		return map[string]any{
			"type": "watch", "available": false, "path": path, "reason": reason,
			"fields": map[string]string{}, "events": []string{},
		}
	}
	return map[string]any{
		"type":      "watch",
		"available": true,
		"path":      path,
		"fields":    status.Fields,
		"events":    status.Log,
	}
}
