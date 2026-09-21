import { render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import AgentRail from '$components/AgentRail.svelte';
import { clientPaneId, normalizeAgent } from '$lib/agents';
import type { Agent, Machine } from '$lib/types';
import { machineGroups, machineWorkspaceBuckets, workspaceGroups, workspaceGroupTrees } from '$lib/workspaces';

function agent(overrides: Partial<Agent> = {}): Agent {
  return {
    relay_id: 'relay-a',
    relay_label: 'Fedora',
    raw_pane_id: 'w1:p1',
    pane_id: 'relay-a::w1:p1',
    agent: 'claude',
    name: 'alpha',
    status: 'working',
    workspace_id: 'w1',
    project: 'proj',
    ...overrides,
  };
}

const machines: Machine[] = [
  { machine_id: 'local', label: 'Fedora', host: 'fedora', local: true, reachable: true },
  { machine_id: 'machine-1', label: 'server-1', host: 'server-1', local: false, reachable: true },
];

describe('machine-scoped pane ids', () => {
  it('keeps the local key and namespaces remote panes by machine', () => {
    expect(clientPaneId('relay-a', 'w1:p1')).toBe('relay-a::w1:p1');
    expect(clientPaneId('relay-a', 'w1:p1', 'local')).toBe('relay-a::w1:p1');
    expect(clientPaneId('relay-a', 'w1:p1', 'machine-1')).toBe('relay-a::machine-1::w1:p1');
  });

  it('tags a remote agent with its machine and per-server pane id', () => {
    const remote = normalizeAgent('relay-a', 'Fedora', {
      raw_pane_id: 'w1:p1',
      machine_id: 'machine-1',
      remote: true,
      agent: 'cmd',
    });
    expect(remote.machine_id).toBe('machine-1');
    expect(remote.pane_id).toBe('relay-a::machine-1::w1:p1');
  });
});

describe('machine grouping', () => {
  it('separates two machines that both host w1:p1, local first', () => {
    const local = agent();
    const remote = agent({
      machine_id: 'machine-1',
      remote: true,
      raw_pane_id: 'w1:p1',
      pane_id: 'relay-a::machine-1::w1:p1',
      agent: 'cmd',
      name: 'beta',
      host: 'server-1',
    });
    const buckets = machineWorkspaceBuckets(workspaceGroups([local, remote]), machines);

    expect(buckets).toHaveLength(2);
    expect(buckets[0].local).toBe(true);
    expect(buckets[0].label).toBe('Fedora');
    expect(buckets[0].groups[0].agents).toHaveLength(1);
    expect(buckets[1].machineId).toBe('machine-1');
    expect(buckets[1].label).toBe('server-1');
    // Same herdr workspace_id, different machines: the groups never merge.
    expect(buckets[1].groups[0].key).not.toBe(buckets[0].groups[0].key);
    expect(buckets[1].groups[0].agents[0].pane_id).toBe('relay-a::machine-1::w1:p1');
  });

  it('groups workspace trees by machine and marks unreachable ones', () => {
    const trees = workspaceGroupTrees(workspaceGroups([
      agent(),
      agent({ machine_id: 'machine-1', remote: true, raw_pane_id: 'w2:p2', pane_id: 'relay-a::machine-1::w2:p2', workspace_id: 'w2' }),
    ]));
    const withOffline = [machines[0], { ...machines[1], reachable: false, error: 'ssh failed' }];
    const groups = machineGroups(trees, withOffline);

    expect(groups).toHaveLength(2);
    expect(groups[0].local).toBe(true);
    expect(groups[1].reachable).toBe(false);
    expect(groups[1].error).toBe('ssh failed');
    expect(groups[1].trees).toHaveLength(1);
  });

  it('renders one rail section per machine', () => {
    const local = agent();
    const remote = agent({
      machine_id: 'machine-1',
      remote: true,
      pane_id: 'relay-a::machine-1::w1:p1',
      agent: 'cmd',
      name: 'beta',
      host: 'server-1',
    });
    const { container } = render(AgentRail, {
      agents: [local, remote],
      machines,
      active: local,
      onopen: vi.fn(),
      onjump: vi.fn(),
    });

    const sections = container.querySelectorAll('.agent-rail-machine');
    expect(sections).toHaveLength(2);
    expect(sections[0].querySelector('.agent-rail-machine-header')?.textContent).toContain('Fedora');
    expect(sections[1].querySelector('.agent-rail-machine-header')?.textContent).toContain('server-1');
    expect(sections[1].textContent).toContain('beta');
    expect(screen.getAllByRole('button').length).toBeGreaterThanOrEqual(3);
  });
});
