package app

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/0cv/herdr-mobile-relay/internal/coordinator"
	"github.com/0cv/herdr-mobile-relay/internal/transport"
)

// A pane reference goes stale when the herdr session behind it was replaced: the
// session was stopped and started again, or restarted under the relay. The relay
// notices — its own generation check refuses the command, and herdr answers
// `server_not_running` or `pane_not_found` — but the phone used to be handed
// herdr's own wording and left holding a row that no longer exists.
//
// These are the refusals that mean exactly that, from the relay's own
// generation check and from herdr itself.
var stalePaneMarkers = []string{
	"pane session was replaced",
	"server_not_running",
	"no herdr server is running",
	"pane_not_found",
}

// paneIsStale reports whether a failed pane-targeted command failed because the
// pane's session is no longer the one the phone was looking at. It only answers
// for failures that named a pane: a workspace or relay-level failure is not a
// stale pane, and neither is a dispatch whose outcome is simply unknown.
func (s *Server) paneIsStale(result *coordinator.CommandResult) bool {
	if result == nil || result.OK || strings.TrimSpace(result.PaneID) == "" {
		return false
	}
	if result.Phase == "dispatched_unknown" {
		return false
	}
	text := strings.ToLower(result.Error)
	for _, marker := range stalePaneMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// stalePaneResult replaces herdr's wording with something a person can act on,
// and tells the phone to reload its list.
func stalePaneResult(result *coordinator.CommandResult) *coordinator.CommandResult {
	data := map[string]any{"code": "pane_stale", "refresh": true}
	if existing, ok := result.Data.(map[string]any); ok {
		for key, value := range existing {
			data[key] = value
		}
	}
	return &coordinator.CommandResult{
		RequestID: result.RequestID,
		Action:    result.Action,
		OK:        false,
		Phase:     "failed",
		Error:     "This pane's session is no longer running, so nothing was sent. The list has been refreshed — open the agent again.",
		PaneID:    result.PaneID,
		Data:      data,
	}
}

// publishCommandResult is the one place a command result reaches the phone, so
// every refusal that means "that pane is gone" is translated the same way, the
// inventory is reloaded before the phone can ask again with the same stale
// identity, and every failure leaves a trace on the relay side.
func (s *Server) publishCommandResult(client *transport.ClientConn, result *coordinator.CommandResult) {
	if result == nil {
		return
	}
	original := result.Error
	stale := s.paneIsStale(result)
	if stale {
		s.refreshAfterStalePane(result.PaneID)
		result = stalePaneResult(result)
	}
	if !result.OK {
		s.logForwardedFailure(client, result, original, stale)
	}
	message := commandResultMessage(result)
	if client == nil {
		s.hub.Broadcast(message)
		return
	}
	s.hub.Send(client, message)
}

// logForwardedFailure records every failure the phone is told about: herdr's own
// wording, the pane it was about, the session the relay is mirroring and who was
// told. Without this a failure that reaches the phone is invisible in the
// service log, which is exactly what makes one impossible to diagnose.
func (s *Server) logForwardedFailure(
	client *transport.ClientConn,
	result *coordinator.CommandResult,
	original string,
	stale bool,
) {
	clientID := ""
	if client != nil {
		clientID = client.ID()
	}
	attributes := []any{
		"action", result.Action,
		"request_id", result.RequestID,
		"pane", result.PaneID,
		"session", s.mirroredSessionLabel(),
		"phase", result.Phase,
		"herdr_error", original,
		"client", clientID,
	}
	if stale {
		attributes = append(attributes,
			"code", "pane_stale",
			"forwarded_error", result.Error,
			"inventory_reloaded", true,
		)
	}
	s.logger.Warn("relay refused a command and told the phone", attributes...)
}

// mirroredSessionLabel names the session the relay is inside, for the log. The
// socket path always identifies it; the name is added once it has been resolved.
func (s *Server) mirroredSessionLabel() string {
	s.sessionMu.Lock()
	name := s.mirroredSession
	s.sessionMu.Unlock()
	socket := s.herdrC.SocketPath()
	if name == "" {
		return socket
	}
	return name + " (" + socket + ")"
}

// refreshAfterStalePane re-reads the session the relay is mirroring. The session
// may have been replaced under it, so the event stream is dropped (which makes
// the poller resubscribe) and the session list is re-broadcast.
func (s *Server) refreshAfterStalePane(paneID string) {
	s.logger.Info("pane belongs to a session that was replaced; reloading the inventory", "pane", paneID)
	s.dropEventStream()
	s.poller.Wake()
	if s.machinesM != nil {
		s.machinesM.Wake()
	}
	s.broadcastSessions(context.Background())
}

// dropEventStream makes the event loop resubscribe. Re-pointing the client at
// the socket it already uses closes the running subscription, which is how a
// replaced session stops feeding the relay without a restart.
func (s *Server) dropEventStream() {
	if s.eventClient == nil {
		return
	}
	s.eventClient.SetPath(s.herdrC.SocketPath())
}

// watchSessionSocket notices that the herdr session behind the relay's socket was
// replaced — a restarted session gets a new socket file — and re-reads
// everything, so the pane list follows the session instead of ageing out of it.
func (s *Server) watchSessionSocket(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	last := socketIdentity(s.herdrC.SocketPath())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current := socketIdentity(s.herdrC.SocketPath())
			if current == last {
				continue
			}
			s.logger.Info(
				"the mirrored herdr session socket changed; reloading the inventory",
				"socket", s.herdrC.SocketPath(), "before", last, "after", current,
			)
			last = current
			s.dropEventStream()
			s.poller.Wake()
			if s.machinesM != nil {
				s.machinesM.Wake()
			}
			s.broadcastSessions(ctx)
		}
	}
}

// socketIdentity changes when the socket file is replaced, even at the same
// path. Modification time and size are used instead of the inode so this stays
// portable across the platforms the relay is built for.
func socketIdentity(path string) string {
	if strings.TrimSpace(path) == "" {
		return "unset"
	}
	info, err := os.Stat(path)
	if err != nil {
		return "missing"
	}
	return fmt.Sprintf("%d/%d", info.ModTime().UnixNano(), info.Size())
}
