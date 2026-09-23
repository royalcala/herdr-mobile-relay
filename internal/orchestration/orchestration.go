// Package orchestration reads the central registry that describes the human's
// orchestration: which herdr sessions exist, what each one is called, where its
// board lives and which scopes its watchdog watches. The relay reads it to label
// the sessions it serves.
//
// The golden rule is that a session with no entry inherits the defaults, so a
// session that was just born is already described. When the file is missing or
// unreadable the embedded defaults apply and the caller says so loudly: silent
// degradation is worse than a missing label.
package orchestration

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Registry is the file.
type Registry struct {
	Defaults Defaults                `json:"defaults"`
	Sessions map[string]SessionEntry `json:"sessions"`
	Agents   map[string]string       `json:"agents"`
}

// Defaults are what a session without an entry inherits.
type Defaults struct {
	Label    string         `json:"label,omitempty"`
	Manager  string         `json:"manager,omitempty"`
	Panels   []string       `json:"panels,omitempty"`
	Policies map[string]any `json:"policies,omitempty"`
	Scopes   []string       `json:"scopes,omitempty"`
	Queue    string         `json:"queue,omitempty"`
	Machines []string       `json:"machines,omitempty"`
}

// SessionEntry is one session's own configuration.
type SessionEntry struct {
	Label    string   `json:"label,omitempty"`
	Project  string   `json:"project,omitempty"`
	Queue    string   `json:"queue,omitempty"`
	Machines []string `json:"machines,omitempty"`
	Scopes   []string `json:"scopes,omitempty"`
	Manager  string   `json:"manager,omitempty"`
	Panels   []string `json:"panels,omitempty"`
}

// Resolved is what the relay needs to serve one session: its label, its board
// and the scopes its watchdog watches, after the defaults have been applied.
type Resolved struct {
	Name     string
	Label    string
	Project  string
	Queue    string
	Scopes   []string
	Machines []string
	Manager  string
	Panels   []string
	// Registered is false when the session had no entry of its own: the phone
	// can then offer to write one (the birth checklist).
	Registered bool
}

// Embedded is what applies when the registry cannot be read: enough to keep the
// relay honest and the phone usable, and deliberately small.
func Embedded() Registry {
	return Registry{
		Defaults: Defaults{
			Manager: "manager-v3",
			Panels:  []string{"vigilante", "dato", "servicios", "deploys", "backups"},
			Policies: map[string]any{
				"fail_open":       "estrecha",
				"gracia_seg":      60,
				"despertador_min": 30,
			},
			Scopes: []string{"local"},
		},
		Sessions: map[string]SessionEntry{},
		Agents:   map[string]string{},
	}
}

// Load reads the registry, filling in anything the file leaves out from the
// embedded defaults.
func Load(path string) (Registry, error) {
	registry := Embedded()
	path = strings.TrimSpace(path)
	if path == "" {
		return registry, fmt.Errorf("no orchestration registry configured")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return registry, err
	}
	var parsed Registry
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return registry, fmt.Errorf("decode %s: %w", path, err)
	}
	registry = parsed
	if registry.Sessions == nil {
		registry.Sessions = map[string]SessionEntry{}
	}
	if registry.Agents == nil {
		registry.Agents = map[string]string{}
	}
	return registry, nil
}

// Resolve describes one session: its own entry where it has one, the defaults
// everywhere else.
func (r Registry) Resolve(name string) Resolved {
	name = strings.TrimSpace(name)
	entry, registered := r.Sessions[name]
	resolved := Resolved{
		Name:       name,
		Label:      firstNonEmpty(entry.Label, r.Defaults.Label, name),
		Project:    entry.Project,
		Queue:      firstNonEmpty(entry.Queue, r.Defaults.Queue),
		Scopes:     firstNonEmptyList(entry.Scopes, r.Defaults.Scopes),
		Machines:   firstNonEmptyList(entry.Machines, r.Defaults.Machines),
		Manager:    firstNonEmpty(entry.Manager, r.Defaults.Manager),
		Panels:     firstNonEmptyList(entry.Panels, r.Defaults.Panels),
		Registered: registered,
	}
	return resolved
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmptyList(values ...[]string) []string {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return nil
}
