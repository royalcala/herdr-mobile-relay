package app

import (
	"context"
	"strings"
	"time"

	"github.com/0cv/herdr-mobile-relay/internal/herdr"
	"github.com/0cv/herdr-mobile-relay/internal/protocol"
	"github.com/0cv/herdr-mobile-relay/internal/transport"
)

// sessionsPayload lists every herdr session on this computer and marks the one
// this relay mirrors. Herdr keeps each session on its own socket, so a relay
// can only be inside one world of workspaces and agents at a time; this is how
// the phone shows the others and asks to move.
func (s *Server) sessionsPayload(ctx context.Context) map[string]any {
	activeSocket := s.herdrC.SocketPath()
	sessions := make([]any, 0, 4)
	active := ""
	if list, err := s.herdrC.ListSessions(ctx); err == nil {
		if current, ok := herdr.SessionForSocket(list, activeSocket); ok {
			active = current.Name
			s.sessionMu.Lock()
			s.mirroredSession = current.Name
			s.sessionMu.Unlock()
		}
		for _, session := range list {
			sessions = append(sessions, map[string]any{
				"name":    session.Name,
				"default": session.Default,
				"running": session.Running,
				"active":  session.SocketPath == activeSocket,
				"dir":     session.SessionDir,
			})
		}
	}
	return map[string]any{"type": "sessions", "sessions": sessions, "active": active}
}

// broadcastSessions pushes the session list to every phone.
func (s *Server) broadcastSessions(ctx context.Context) {
	s.hub.Broadcast(s.sessionsPayload(ctx))
}

// sessionsSignature is the cheap comparison that decides whether the list is
// worth another broadcast.
func sessionsSignature(payload map[string]any) string {
	sessions, _ := payload["sessions"].([]any)
	var builder strings.Builder
	for _, entry := range sessions {
		session, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		name, _ := session["name"].(string)
		running, _ := session["running"].(bool)
		builder.WriteString(name)
		if running {
			builder.WriteByte('+')
		}
		builder.WriteByte(',')
	}
	builder.WriteString("|")
	builder.WriteString(payload["active"].(string))
	return builder.String()
}

// watchSessions keeps the session list fresh for every connected phone, without
// running the herdr CLI on every poll.
func (s *Server) watchSessions(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	last := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			payload := s.sessionsPayload(ctx)
			signature := sessionsSignature(payload)
			if signature == last {
				continue
			}
			last = signature
			s.hub.Broadcast(payload)
		}
	}
}

// handleSelectSession moves the whole relay to another herdr session: the
// socket every read goes through, the event stream, and the inventory behind
// it. The phone is told about it in three steps so it never shows the old
// session's agents as if they belonged to the new one.
func (s *Server) handleSelectSession(
	client *transport.ClientConn,
	inbound protocol.Inbound,
	action string,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	name := strings.TrimSpace(inbound.Name)
	if name == "" {
		s.sendCommandResult(client, inbound.RequestID, action, false, "failed", "A session name is required", "", nil)
		return
	}
	list, err := s.herdrC.ListSessions(ctx)
	if err != nil {
		s.sendCommandResult(client, inbound.RequestID, action, false, "failed", "This computer's herdr sessions could not be listed", "", nil)
		return
	}
	session, ok := herdr.SessionForName(list, name)
	if !ok {
		s.sendCommandResult(client, inbound.RequestID, action, false, "failed", "Unknown herdr session", "", nil)
		return
	}
	if session.SocketPath == "" {
		s.sendCommandResult(client, inbound.RequestID, action, false, "failed", "That session has no socket to mirror", "", nil)
		return
	}

	if session.SocketPath != s.herdrC.SocketPath() {
		s.sessionMu.Lock()
		s.herdrC.SetSocketPath(session.SocketPath)
		if s.eventClient != nil {
			s.eventClient.SetPath(session.SocketPath)
		}
		// The previous session's panes are gone. Dropping them now keeps pane
		// generations from authorising reads against panes that no longer exist.
		s.state.Reset()
		s.sessionMu.Unlock()

		s.broadcastCommitted(map[string]any{"type": "agents", "agents": []any{}})
		s.broadcastCommitted(map[string]any{"type": "workspaces", "workspaces": []any{}})
		// Wake everything that fills the inventory back in, so the phone is not
		// left staring at an empty screen until the next scheduled poll.
		s.poller.Wake()
		if s.machinesM != nil {
			s.machinesM.Wake()
		}
		s.logger.Info("mirroring another herdr session", "session", session.Name, "socket", session.SocketPath)
	}

	s.broadcastSessions(ctx)
	s.sendCommandResult(client, inbound.RequestID, action, true, "completed", "", "", map[string]any{
		"session": session.Name,
	})
}
