package herdr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// Machine is a saved SSH connection profile reported by `herdr machine list`.
// The ID is the stable selector for the global `--machine` option; the label is
// what the sidebar shows.
type Machine struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Target   string `json:"target"`
	Session  string `json:"session"`
	Enabled  bool   `json:"enabled"`
	Selected bool   `json:"selected"`
}

// MachineList lists the saved SSH machines. `herdr machine list --json` prints a
// bare JSON array rather than the {"result": ...} envelope the API commands use,
// so it parses the array directly and still tolerates an enveloped response for
// forward compatibility.
func (c *Client) MachineList(ctx context.Context) ([]Machine, error) {
	out, err := c.runCommand(ctx, "machine", "list", "--json")
	if err != nil {
		return nil, fmt.Errorf("herdr machine list: %w", err)
	}
	return decodeMachineList(out)
}

func decodeMachineList(out []byte) ([]Machine, error) {
	trimmed := bytes.TrimSpace(out)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var machines []Machine
		if err := json.Unmarshal(trimmed, &machines); err != nil {
			return nil, fmt.Errorf("herdr machine list: %w", err)
		}
		return machines, nil
	}
	var envelope struct {
		Result struct {
			Machines []Machine `json:"machines"`
		} `json:"result"`
	}
	if err := json.Unmarshal(trimmed, &envelope); err != nil {
		return nil, fmt.Errorf("herdr machine list: %w", err)
	}
	return envelope.Result.Machines, nil
}

// MachineAgentList reads a saved machine's agent inventory through the global
// `--machine` prefix. Every API command on a machine uses the same prefix.
func (c *Client) MachineAgentList(ctx context.Context, machineID string) ([]Pane, error) {
	var result struct {
		Agents *[]Pane `json:"agents"`
	}
	if err := c.runResult(ctx, &result, "--machine", machineID, "agent", "list"); err != nil {
		return nil, fmt.Errorf("herdr machine agent list: %w", err)
	}
	if result.Agents == nil {
		return nil, fmt.Errorf("herdr machine agent list: response has no agents")
	}
	for i := range *result.Agents {
		(*result.Agents)[i].Session = (*result.Agents)[i].SessionRaw.Value
	}
	return *result.Agents, nil
}

func (c *Client) MachineWorkspaceList(ctx context.Context, machineID string) ([]Workspace, error) {
	var result struct {
		Workspaces *[]Workspace `json:"workspaces"`
	}
	if err := c.runResult(ctx, &result, "--machine", machineID, "workspace", "list"); err != nil {
		return nil, fmt.Errorf("herdr machine workspace list: %w", err)
	}
	if result.Workspaces == nil {
		return nil, fmt.Errorf("herdr machine workspace list: response has no workspaces")
	}
	return *result.Workspaces, nil
}

func (c *Client) MachineTabList(ctx context.Context, machineID string) ([]Tab, error) {
	var result struct {
		Tabs *[]Tab `json:"tabs"`
	}
	if err := c.runResult(ctx, &result, "--machine", machineID, "tab", "list"); err != nil {
		return nil, fmt.Errorf("herdr machine tab list: %w", err)
	}
	if result.Tabs == nil {
		return nil, fmt.Errorf("herdr machine tab list: response has no tabs")
	}
	return *result.Tabs, nil
}

func (c *Client) MachinePaneList(ctx context.Context, machineID string) ([]Pane, error) {
	var result struct {
		Panes *[]Pane `json:"panes"`
	}
	if err := c.runResult(ctx, &result, "--machine", machineID, "pane", "list"); err != nil {
		return nil, fmt.Errorf("herdr machine pane list: %w", err)
	}
	if result.Panes == nil {
		return nil, fmt.Errorf("herdr machine pane list: response has no panes")
	}
	for index := range *result.Panes {
		(*result.Panes)[index].Session = (*result.Panes)[index].SessionRaw.Value
	}
	return *result.Panes, nil
}

// ReadPaneMachine reads a pane on a saved machine. Remote panes have no local
// socket API, so this is always the CLI path.
func (c *Client) ReadPaneMachine(
	ctx context.Context,
	machineID, paneID string,
	lines int,
	format, source string,
) (PaneRead, error) {
	if lines < 1 {
		lines = 1
	}
	if format != "ansi" {
		format = "text"
	}
	content, err := c.runCommand(ctx,
		"--machine", machineID,
		"pane", "read", paneID,
		"--lines", strconv.Itoa(lines),
		"--source", source,
		"--format", format,
	)
	if err != nil {
		return PaneRead{}, err
	}
	return PaneRead{Content: content}, nil
}
