package herdr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Session is one named herdr session, as `herdr session list --json` reports it.
//
// A session owns its own socket, so it is a separate world of workspaces and
// agents. The relay mirrors one at a time; this is what lets the phone see the
// others and ask for one.
type Session struct {
	Name       string `json:"name"`
	Default    bool   `json:"default"`
	Running    bool   `json:"running"`
	SessionDir string `json:"session_dir"`
	SocketPath string `json:"socket_path"`
}

type sessionListPayload struct {
	Sessions []Session `json:"sessions"`
}

// ListSessions asks the herdr CLI which sessions exist. It is global rather than
// scoped to the socket this client mirrors: the answer lists every session,
// including the ones this client is not talking to.
func (c *Client) ListSessions(ctx context.Context) ([]Session, error) {
	if c == nil || strings.TrimSpace(c.bin) == "" {
		return nil, errors.New("herdr binary is unavailable")
	}
	out, err := c.runCommand(ctx, "session", "list", "--json")
	if err != nil {
		return nil, fmt.Errorf("herdr session list: %w", err)
	}
	var payload sessionListPayload
	if err := json.Unmarshal([]byte(extractJSONObject(string(out))), &payload); err != nil {
		return nil, fmt.Errorf("decode herdr session list: %w", err)
	}
	sessions := make([]Session, 0, len(payload.Sessions))
	for _, session := range payload.Sessions {
		session.Name = strings.TrimSpace(session.Name)
		session.SocketPath = strings.TrimSpace(session.SocketPath)
		session.SessionDir = strings.TrimSpace(session.SessionDir)
		if session.Name == "" {
			continue
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// extractJSONObject keeps the first object in the output, so a future banner or
// a trailing notice cannot break the phone.
func extractJSONObject(raw string) string {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end <= start {
		return raw
	}
	return raw[start : end+1]
}

// SessionForName picks the session a relay should mirror. An empty name means
// "the default session", which is what the relay has always mirrored.
func SessionForName(sessions []Session, name string) (Session, bool) {
	if name = strings.TrimSpace(name); name != "" {
		for _, session := range sessions {
			if session.Name == name {
				return session, true
			}
		}
		return Session{}, false
	}
	for _, session := range sessions {
		if session.Default {
			return session, true
		}
	}
	if len(sessions) > 0 {
		return sessions[0], true
	}
	return Session{}, false
}

// SessionForSocket recognises the session a relay is already mirroring, so the
// phone can mark it instead of guessing from a name.
func SessionForSocket(sessions []Session, socketPath string) (Session, bool) {
	if socketPath = strings.TrimSpace(socketPath); socketPath == "" {
		return Session{}, false
	}
	for _, session := range sessions {
		if session.SocketPath == socketPath {
			return session, true
		}
	}
	return Session{}, false
}
