package app

import (
	"context"
	"strings"
	"time"

	"github.com/0cv/herdr-mobile-relay/internal/coordinator"
	"github.com/0cv/herdr-mobile-relay/internal/herdr"
	"github.com/0cv/herdr-mobile-relay/internal/protocol"
	"github.com/0cv/herdr-mobile-relay/internal/queue"
	"github.com/0cv/herdr-mobile-relay/internal/transport"
)

// handleSessionSnapshot answers for ONE session, whichever it is: its agents, its
// panes, its board and the panels the registry gives it.
//
// It reads that session's own socket on demand rather than keeping a poller per
// session: herdr gives every session its own socket, so a read is a dial away,
// and the session being mirrored keeps its stream untouched. A session that
// cannot be read answers with a coded refusal, so the phone refreshes its list
// instead of showing herdr's own words.
func (s *Server) handleSessionSnapshot(
	client *transport.ClientConn,
	inbound protocol.Inbound,
	action string,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	name := strings.TrimSpace(inbound.Name)
	if name == "" {
		s.failSessionSnapshot(client, inbound, action, "A session name is required")
		return
	}
	sessions, err := s.herdrC.ListSessions(ctx)
	if err != nil {
		s.failSessionSnapshot(client, inbound, action, "This computer's herdr sessions could not be listed")
		return
	}
	session, ok := herdr.SessionForName(sessions, name)
	if !ok {
		s.failSessionSnapshot(client, inbound, action, "That herdr session does not exist any more")
		return
	}
	resolved := s.orchestrationFor(session.Name)

	payload := map[string]any{
		"session":    session.Name,
		"label":      resolved.Label,
		"running":    session.Running,
		"registered": resolved.Registered,
		"project":    resolved.Project,
		"queue":      resolved.Queue,
		"scopes":     resolved.Scopes,
		"panels":     resolved.Panels,
		"manager":    resolved.Manager,
		"mirrored":   session.SocketPath == s.herdrC.SocketPath(),
		"agents":     []any{},
		"panes":      []any{},
	}
	if board, boardErr := queue.Load(resolved.Queue); boardErr == nil {
		tasks := make([]any, 0, len(board.Tasks))
		open := 0
		for _, task := range board.Tasks {
			if task.State != "done" {
				open++
			}
			tasks = append(tasks, map[string]any{
				"id": task.ID, "title": task.Title, "owner": task.Owner,
				"state": task.State, "branch": task.Branch, "waits_on_human": task.WaitsOnHuman,
			})
		}
		payload["board"] = map[string]any{"available": true, "tasks": tasks, "open": open, "path": resolved.Queue}
	} else {
		payload["board"] = map[string]any{"available": false, "path": resolved.Queue}
	}

	if !session.Running || session.SocketPath == "" {
		// A stopped session is not an error to retry blindly: say what is true.
		payload["read_error"] = "The session is not running"
		s.completeSessionSnapshot(client, inbound, action, payload)
		return
	}

	// A throwaway client on that session's socket: the mirror's client stays
	// pointed at the session it is responsible for.
	other := herdr.NewClient(s.cfg.HerdrBin, session.SocketPath)
	inventory, inventoryErr := other.GetInventory(ctx)
	panes, panesErr := other.PaneList(ctx)
	if inventoryErr != nil && panesErr != nil {
		payload["read_error"] = inventoryErr.Error()
		s.completeSessionSnapshot(client, inbound, action, payload)
		return
	}
	agents := []any{}
	if inventoryErr == nil {
		for _, pane := range inventory.Panes {
			agents = append(agents, sessionPanePayload(pane))
		}
	}
	allPanes := []any{}
	if panesErr == nil {
		for _, pane := range panes {
			allPanes = append(allPanes, sessionPanePayload(pane))
		}
	}
	payload["agents"] = agents
	payload["panes"] = allPanes
	payload["agent_count"] = len(agents)
	payload["pane_count"] = len(allPanes)

	s.completeSessionSnapshot(client, inbound, action, payload)
}

func (s *Server) completeSessionSnapshot(
	client *transport.ClientConn,
	inbound protocol.Inbound,
	action string,
	payload map[string]any,
) {
	s.publishCommandResult(client, &coordinator.CommandResult{
		RequestID: inbound.RequestID,
		Action:    action,
		OK:        true,
		Phase:     "completed",
		Data:      payload,
	})
}

// failSessionSnapshot tells the phone what happened and asks it to reload, in
// the same coded shape a stale pane uses: the list is the truth, and it changed.
func (s *Server) failSessionSnapshot(
	client *transport.ClientConn,
	inbound protocol.Inbound,
	action string,
	message string,
) {
	s.sendCommandResult(client, inbound.RequestID, action, false, "failed", message, "", map[string]any{
		"code": "session_unavailable", "refresh": true,
	})
}

// sessionPanePayload is the part of a pane a phone needs to recognise it.
func sessionPanePayload(pane herdr.Pane) map[string]any {
	return map[string]any{
		"pane_id":      pane.ID,
		"agent":        pane.Agent,
		"agent_status": pane.Status,
		"name":         pane.Name,
		"cwd":          pane.Cwd,
		"workspace_id": pane.WorkspaceID,
		"tab_id":       pane.TabID,
		"tab_label":    pane.TabLabel,
	}
}
