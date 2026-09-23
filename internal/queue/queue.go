// Package queue reads the versioned task board the human keeps in the repo
// (queue/tasks.json). It is deliberately dumb: parse, validate, and hand the
// rows over. Crossing them with what herdr is doing right now happens on the
// phone, where the live agent state already lives.
package queue

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Task is one row of the board.
type Task struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Repo       string `json:"repo,omitempty"`
	Owner      string `json:"owner,omitempty"`
	State      string `json:"state,omitempty"`
	Branch     string `json:"branch,omitempty"`
	LastCommit string `json:"last_commit,omitempty"`
	BlockedBy  string `json:"blocked_by,omitempty"`
	// WaitsOnHuman is the row the human has to move. It is separate from State
	// because a task can be blocked on a person rather than on another task.
	WaitsOnHuman bool   `json:"waits_on_human,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

// Board is the whole file.
type Board struct {
	Version   int    `json:"version"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Tasks     []Task `json:"tasks"`
}

// KnownStates are the states the phone knows how to colour and group. Anything
// else is kept but shown as unknown rather than dropped, so a typo is visible
// instead of silent.
var KnownStates = []string{"queued", "working", "blocked", "review", "done"}

// Load reads and validates the board.
func Load(path string) (Board, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Board{}, fmt.Errorf("queue path is empty")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Board{}, err
	}
	var board Board
	if err := json.Unmarshal(raw, &board); err != nil {
		return Board{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if board.Tasks == nil {
		board.Tasks = []Task{}
	}
	seen := make(map[string]bool, len(board.Tasks))
	for index := range board.Tasks {
		task := &board.Tasks[index]
		task.ID = strings.TrimSpace(task.ID)
		task.Title = strings.TrimSpace(task.Title)
		task.State = strings.ToLower(strings.TrimSpace(task.State))
		if task.ID == "" {
			return Board{}, fmt.Errorf("task %d has no id", index)
		}
		if seen[task.ID] {
			return Board{}, fmt.Errorf("duplicate task id %q", task.ID)
		}
		seen[task.ID] = true
	}
	sort.SliceStable(board.Tasks, func(i, j int) bool {
		left, right := board.Tasks[i], board.Tasks[j]
		if (left.WaitsOnHuman || left.State == "blocked") != (right.WaitsOnHuman || right.State == "blocked") {
			return left.WaitsOnHuman || left.State == "blocked"
		}
		if left.State != right.State {
			return stateRank(left.State) < stateRank(right.State)
		}
		return left.ID < right.ID
	})
	return board, nil
}

func stateRank(state string) int {
	for index, known := range KnownStates {
		if state == known {
			return index
		}
	}
	return len(KnownStates)
}

// Signature changes when the board's content does, so a watcher can broadcast
// only real edits. It deliberately ignores formatting.
func Signature(board Board) string {
	var builder strings.Builder
	builder.WriteString(board.UpdatedAt)
	for _, task := range board.Tasks {
		builder.WriteString("|")
		builder.WriteString(task.ID)
		builder.WriteString(task.State)
		builder.WriteString(task.Owner)
		builder.WriteString(task.Branch)
		builder.WriteString(task.LastCommit)
		builder.WriteString(task.BlockedBy)
		if task.WaitsOnHuman {
			builder.WriteString("!")
		}
	}
	return builder.String()
}
