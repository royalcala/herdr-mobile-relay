package app

import (
	"context"
	"strings"
	"time"

	"github.com/0cv/herdr-mobile-relay/internal/queue"
)

// queuePayload is the versioned task board as the phone needs it: the rows plus
// where they came from, so a board that fails to load says so instead of
// looking empty.
func (s *Server) queuePayload() map[string]any {
	path := strings.TrimSpace(s.cfg.QueuePath)
	if path == "" {
		return map[string]any{"type": "queue", "tasks": []any{}, "available": false, "reason": "no board path configured"}
	}
	board, err := queue.Load(path)
	if err != nil {
		return map[string]any{"type": "queue", "tasks": []any{}, "available": false, "reason": "the task board could not be read"}
	}
	tasks := make([]any, 0, len(board.Tasks))
	for _, task := range board.Tasks {
		tasks = append(tasks, map[string]any{
			"id":             task.ID,
			"title":          task.Title,
			"repo":           task.Repo,
			"owner":          task.Owner,
			"state":          task.State,
			"branch":         task.Branch,
			"last_commit":    task.LastCommit,
			"blocked_by":     task.BlockedBy,
			"waits_on_human": task.WaitsOnHuman,
			"updated_at":     task.UpdatedAt,
			"notes":          task.Notes,
		})
	}
	return map[string]any{
		"type":       "queue",
		"tasks":      tasks,
		"available":  true,
		"path":       path,
		"updated_at": board.UpdatedAt,
	}
}

// watchQueue pushes the board when the file behind it changes. The board is
// versioned in git, so edits are deliberate and rare; polling the file is
// cheaper and simpler than watching it.
func (s *Server) watchQueue(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	last := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			payload := s.queuePayload()
			signature := queueSignature(payload)
			if signature == last {
				continue
			}
			last = signature
			s.hub.Broadcast(payload)
		}
	}
}

func queueSignature(payload map[string]any) string {
	tasks, _ := payload["tasks"].([]any)
	var builder strings.Builder
	builder.WriteString(str(payload["available"]))
	for _, entry := range tasks {
		task, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		builder.WriteString("|")
		builder.WriteString(str(task["id"]))
		builder.WriteString(str(task["state"]))
		builder.WriteString(str(task["owner"]))
		builder.WriteString(str(task["branch"]))
		builder.WriteString(str(task["last_commit"]))
		builder.WriteString(str(task["blocked_by"]))
		builder.WriteString(str(task["waits_on_human"]))
	}
	return builder.String()
}

func str(value any) string {
	text, _ := value.(string)
	return text
}
