import { describe, expect, it } from 'vitest';
import { buildOrchestration, branchSegment, liveStatus, taskMatchesAgent } from '$lib/orchestration';
import type { Agent, QueueTask } from '$lib/types';

function agent(overrides: Partial<Agent>): Agent {
  return {
    relay_id: 'relay-1',
    relay_label: 'Laptop',
    raw_pane_id: 'w1:p1',
    pane_id: 'w1:p1',
    ...overrides,
  } as Agent;
}

function task(overrides: Partial<QueueTask>): QueueTask {
  return { id: 't1', title: 'A task', state: 'working', ...overrides };
}

describe('orchestration crossing', () => {
  it('matches by the owner name herdr was given', () => {
    expect(taskMatchesAgent(
      task({ owner: 'relay-pwa-ui' }),
      agent({ name: 'relay-pwa-ui', cwd: '/somewhere/else' }),
    )).toBe(true);
  });

  it('matches by the branch checked out in the worktree path', () => {
    expect(taskMatchesAgent(
      task({ branch: 'feat/pwa-machines-ui' }),
      agent({ name: 'someone-else', cwd: '/repo/.worktrees/pwa-machines-ui' }),
    )).toBe(true);
  });

  it('matches by the repository in the path', () => {
    expect(taskMatchesAgent(
      task({ repo: 'herdr-mobile-relay' }),
      agent({ name: 'someone-else', cwd: '/home/me/Documents/github/herdr-mobile-relay' }),
    )).toBe(true);
  });

  it('does not match a path segment that is only a prefix', () => {
    expect(taskMatchesAgent(
      task({ branch: 'main' }),
      agent({ name: 'x', cwd: '/srv/maintenance' }),
    )).toBe(false);
    expect(taskMatchesAgent(
      task({ repo: 'remobi' }),
      agent({ name: 'x', cwd: '/srv/remobi-trial' }),
    )).toBe(false);
  });

  it('takes the last branch segment, because herdr worktrees are named after it', () => {
    expect(branchSegment('feat/turso-fase2-spec')).toBe('turso-fase2-spec');
    expect(branchSegment('main')).toBe('main');
    expect(branchSegment('')).toBe('');
  });

  it('reports the most alarming live status of a task', () => {
    expect(liveStatus([agent({ status: 'done' }), agent({ status: 'blocked' })])).toBe('blocked');
    expect(liveStatus([agent({ status: 'idle' }), agent({ status: 'working' })])).toBe('working');
    expect(liveStatus([])).toBe('');
  });

  it('splits the board into what a person must move, what is running, and what is done', () => {
    const view = buildOrchestration(
      [
        task({ id: 'wait', waits_on_human: true, owner: 'manager-v3' }),
        task({ id: 'blocked', state: 'blocked' }),
        task({ id: 'running', state: 'working', owner: 'relay-pwa-ui' }),
        task({ id: 'finished', state: 'done' }),
      ],
      [
        agent({ name: 'manager-v3' }),
        agent({ name: 'relay-pwa-ui' }),
        agent({ name: 'nobody-wrote-this-down', pane_id: 'w9:p1', status: 'working' }),
      ],
    );

    expect(view.waitsOnHuman.map((row) => row.task.id)).toEqual(['wait', 'blocked']);
    expect(view.inFlight.map((row) => row.task.id)).toEqual(['running']);
    expect(view.done.map((row) => row.task.id)).toEqual(['finished']);
    // Work nobody wrote down is shown, not hidden.
    expect(view.offBoard.map((entry) => entry.name)).toEqual(['nobody-wrote-this-down']);
    // A task waiting on a person also carries its live agent.
    expect(view.waitsOnHuman[0].agents.map((entry) => entry.name)).toEqual(['manager-v3']);
  });

  it('counts one agent once, even when several rows could claim it', () => {
    const shared = agent({ name: 'relay-pwa-ui', cwd: '/repo/.worktrees/pwa-machines-ui' });
    const view = buildOrchestration(
      [
        task({ id: 'by-owner', owner: 'relay-pwa-ui' }),
        task({ id: 'by-branch', branch: 'feat/pwa-machines-ui' }),
      ],
      [shared],
    );
    expect(view.offBoard).toHaveLength(0);
    expect(view.inFlight).toHaveLength(2);
  });
});
