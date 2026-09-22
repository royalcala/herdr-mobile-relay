import { render, screen } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import AgentList from '$components/AgentList.svelte';
import AgentRail from '$components/AgentRail.svelte';
import { clientPaneId, normalizeAgent } from '$lib/agents';
import {
  buildMachineIndex,
  groupAgentsByMachine,
  machineKey,
  machineRef,
  machineTag,
  normalizeMachineId,
} from '$lib/machines';
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

// Machines as the relay reports them: tagged with the relay they belong to, so
// two relays that both call their own host `local` never collide.
const relayMachines: Machine[] = [
  { machine_id: 'local', relay_id: 'relay-a', label: 'Fedora', host: 'fedora', local: true, reachable: true },
  { machine_id: 'server-1', relay_id: 'relay-a', label: 'server-1', host: 'server-1', local: false, reachable: true },
];

describe('machine identity and tags', () => {
  it('normalizes an id-less machine to local and keeps relay-scoped ids apart', () => {
    expect(normalizeMachineId(undefined)).toBe('local');
    expect(normalizeMachineId('')).toBe('local');
    expect(normalizeMachineId('local')).toBe('local');
    expect(normalizeMachineId('server-1')).toBe('server-1');
    expect(machineKey('relay-a', 'server-1')).toBe('relay-a\u0000server-1');
  });

  it('resolves a machine by relay before falling back to the bare id', () => {
    const index = buildMachineIndex([
      { machine_id: 'local', relay_id: 'relay-a', label: 'Alpha', host: 'alpha' },
      { machine_id: 'local', relay_id: 'relay-b', label: 'Beta', host: 'beta' },
    ]);
    expect(index.get('relay-a', 'local')?.label).toBe('Alpha');
    expect(index.get('relay-b', 'local')?.label).toBe('Beta');
    // A machine list from before the relay tag existed still labels its rail.
    expect(index.get('relay-c', 'local')?.label).toBe('Alpha');
  });

  it('tags reachable remotes as remote and unreachable ones as offline', () => {
    const index = buildMachineIndex([
      ...relayMachines,
      { machine_id: 'server-2', relay_id: 'relay-a', label: 'server-2', local: false, reachable: false },
    ]);
    expect(machineTag(machineRef(index, 'relay-a', 'Fedora', undefined))).toBe('local');
    expect(machineTag(machineRef(index, 'relay-a', 'Fedora', 'server-1'))).toBe('remote');
    expect(machineTag(machineRef(index, 'relay-a', 'Fedora', 'server-2'))).toBe('offline');
    // A machine never heard from is optimistically reachable, like any agent.
    expect(machineTag(machineRef(index, 'relay-a', 'Fedora', 'unknown'))).toBe('remote');
  });

  it('marks every machine read-only for a reader-role relay', () => {
    const index = buildMachineIndex(relayMachines);
    expect(machineRef(index, 'relay-a', 'Fedora', undefined, '', new Set(['relay-a'])).readOnly).toBe(true);
  });
});

describe('machine grouping for the home list', () => {
  it('counts each status group per machine, local first', () => {
    const sections = groupAgentsByMachine(
      [
        agent({ project: 'localproj' }),
        agent({ status: 'blocked', attention_kind: 'approval', attention_capable: true, raw_pane_id: 'w1:p2', pane_id: 'relay-a::w1:p2' }),
        agent({
          machine_id: 'server-1', remote: true, status: 'idle', raw_pane_id: 'w1:p1',
          pane_id: 'relay-a::server-1::w1:p1', host: 'server-1',
        }),
      ],
      [],
      relayMachines,
    );

    expect(sections).toHaveLength(2);
    expect(sections[0].ref.local).toBe(true);
    expect(sections[0].ref.label).toBe('Fedora');
    expect(sections[0].counts).toMatchObject({ total: 2, blocked: 1, working: 1 });
    expect(sections[1].ref.readOnly).toBe(true);
    expect(sections[1].counts).toMatchObject({ total: 1, idle: 1, blocked: 0 });
  });

  it('gives a reported but empty machine its own section', () => {
    const sections = groupAgentsByMachine([agent()], [], relayMachines);
    expect(sections).toHaveLength(2);
    expect(sections[1].ref.label).toBe('server-1');
    expect(sections[1].counts.total).toBe(0);
  });
});

describe('machine home sections', () => {
  const relay = { id: 'relay-a', label: 'Fedora', url: 'wss://relay-a', token: '' };
  const local = agent({ project: 'localproj' });
  const blocked = agent({
    project: 'blockedproj', status: 'blocked', attention_kind: 'approval', attention_capable: true,
    raw_pane_id: 'w1:p3', pane_id: 'relay-a::w1:p3',
  });
  const remote = agent({
    project: 'remoteproj', machine_id: 'server-1', remote: true, host: 'server-1',
    raw_pane_id: 'w1:p1', pane_id: 'relay-a::server-1::w1:p1',
  });

  function renderList() {
    return render(AgentList, {
      agents: [local, blocked, remote],
      relays: [relay],
      machines: relayMachines,
      responding: new Set<string>(),
      onopen: vi.fn(),
    });
  }

  it('renders one section per machine and tags a remote as read-only', () => {
    const { container } = renderList();
    const sections = container.querySelectorAll('.machine-section');
    expect(sections).toHaveLength(2);
    expect(sections[0].querySelector('.machine-name')?.textContent).toBe('Fedora');
    expect(sections[0].querySelector('.machine-readonly')).toBeNull();
    expect(sections[1].querySelector('.machine-name')?.textContent).toBe('server-1');
    expect(sections[1].querySelector('.machine-tag')?.textContent?.toLowerCase()).toBe('remote');
    expect(sections[1].querySelector('.machine-readonly')?.textContent).toContain('Read-only');
    // Every remote card wears a locked machine badge, before it is opened.
    expect(sections[1].querySelector('.machine-badge.read-only')?.textContent).toContain('server-1');
  });

  it('scopes the list to the chosen machine and status', async () => {
    const user = userEvent.setup();
    renderList();
    // The workspace card repeats the project name, so target the agent rows.
    const project = (name: string) => screen.queryByText(name, { selector: '.agent-project' });

    expect(project('localproj')).toBeInTheDocument();
    expect(project('remoteproj')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /Show server-1/ }));
    expect(project('remoteproj')).toBeInTheDocument();
    expect(project('localproj')).not.toBeInTheDocument();
    expect(project('blockedproj')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /Show Fedora/ }));
    expect(project('localproj')).toBeInTheDocument();
    expect(project('remoteproj')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Blocked' }));
    expect(project('blockedproj')).toBeInTheDocument();
    expect(project('localproj')).not.toBeInTheDocument();
  });

  it('keeps the flat, status-first list for a single-machine relay', () => {
    const { container } = render(AgentList, {
      agents: [local, blocked],
      relays: [relay],
      machines: [relayMachines[0]],
      responding: new Set<string>(),
      onopen: vi.fn(),
    });
    expect(container.querySelectorAll('.machine-section')).toHaveLength(0);
    expect(container.querySelector('.machine-badge')).toBeNull();
    expect(screen.getByRole('heading', { name: 'Needs input' })).toBeInTheDocument();
  });
});
