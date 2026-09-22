import { render, screen, within } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';
import AgentList from '$components/AgentList.svelte';
import { setHomeLayout } from '$lib/preferences';
import type { Agent, RelayConnectionView, RelayWorkspace } from '$lib/types';

function agent(overrides: Partial<Agent>): Agent {
  return {
    relay_id: 'fedora',
    relay_label: 'Fedora',
    raw_pane_id: 'w1:p1',
    pane_id: 'fedora::w1:p1',
    agent: 'claude',
    name: 'alpha',
    status: 'working',
    workspace_id: 'w1',
    project: 'proj',
    ...overrides,
  };
}

function workspace(overrides: Partial<RelayWorkspace>): RelayWorkspace {
  return {
    relay_id: 'fedora',
    relay_label: 'Fedora',
    workspace_id: 'w1',
    number: 1,
    label: 'workspace',
    focused: false,
    pane_count: 1,
    tab_count: 1,
    active_tab_id: '',
    agent_status: '',
    cwd: '/srv/workspace',
    ...overrides,
  };
}

function readyConnection(): RelayConnectionView {
  return { status: 'connected', inventory: { state: 'ready' } } as unknown as RelayConnectionView;
}

describe('mirror state', () => {
  it('keeps a workspace that hosts a working agent out of the idle section', () => {
    // Regression: workspaceIdentity became machine-scoped, but the idle
    // section still keyed its workspace records by relay+workspace. Every
    // record then looked unoccupied, so each mirrored workspace was rendered
    // as an extra empty idle card even when it hosted a working agent.
    setHomeLayout('state');
    const localWorking = agent({
      workspace_id: 'wR',
      raw_pane_id: 'wR:p2',
      pane_id: 'fedora::wR:p2',
      project: 'alpha',
      status: 'working',
    });
    const remoteIdle = agent({
      machine_id: 'm1',
      remote: true,
      host: 'server-1',
      workspace_id: 'w2',
      raw_pane_id: 'w2:p2',
      pane_id: 'fedora::m1::w2:p2',
      project: 'beta',
      status: 'idle',
    });
    const { container } = render(AgentList, {
      agents: [localWorking, remoteIdle],
      relays: [{ id: 'fedora', label: 'Fedora', url: 'wss://fedora', token: '' }],
      workspaces: [
        workspace({ workspace_id: 'wR', label: 'alpha', cwd: '/srv/alpha' }),
        workspace({ machine_id: 'm1', workspace_id: 'w2', label: 'beta', cwd: '/srv/beta' }),
      ],
      connections: new Map([['fedora', readyConnection()]]),
      responding: new Set<string>(),
      onopen: vi.fn(),
    });

    const idle = screen.getByRole('heading', { name: 'Idle' }).closest('section')!;
    const working = screen.getByRole('heading', { name: 'Working' }).closest('section')!;
    // The mirrored idle workspace is the only idle card; the local workspace
    // that hosts a working agent keeps its own working card instead.
    expect(idle.querySelectorAll('.workspace-card')).toHaveLength(1);
    expect(within(idle).getByText('beta', { selector: 'summary strong' })).toBeInTheDocument();
    expect(within(idle).queryByText('alpha', { selector: 'summary strong' })).not.toBeInTheDocument();
    expect(working.querySelectorAll('.workspace-card')).toHaveLength(1);
    expect(within(working).getByText('alpha', { selector: 'summary strong' })).toBeInTheDocument();
    expect(container.querySelectorAll('.workspace-card')).toHaveLength(2);
    setHomeLayout('mixed');
  });
});
