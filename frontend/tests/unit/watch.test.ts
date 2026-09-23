import { describe, expect, it } from 'vitest';
import { agentRoles } from '$lib/roles';
import { watchEvents, watchView } from '$lib/watch';
import type { Agent, QueueTask } from '$lib/types';

/**
 * The fields are a verbatim copy of the watchdog's state file on this machine,
 * so the panel is tested against what it really writes.
 */
const FIELDS: Record<string, string> = {
  pid: '1980822',
  manager: 'manager-v3',
  interval: '20',
  fail_threshold: '3',
  started_epoch: '1790114430',
  heartbeat_epoch: '1790183073',
  passes: '2933',
  seed: '0',
  queue: '0',
  last_delivery_epoch: '1790182306',
  last_delivery_name: 'relay-pwa-ui',
  last_delivery_scope: 'local',
  last_delivery_why: 'FINISHED',
  last_delivery_pane: 'w11:p1',
  'scope.local.result': 'ok',
  'scope.local.streak': '0',
  'scope.local.epoch': '1790183069',
  'scope.local.error': '',
  'scope.server-1.result': 'ok',
  'scope.server-1.epoch': '1790183071',
  'scope.server-2.result': 'error',
  'scope.server-2.streak': '2',
  'scope.server-2.epoch': '1790183000',
  'scope.server-2.error': 'ssh: connect to host server-2 port 22: timed out',
};

describe('watchdog panel', () => {
  const now = 1790183073 + 5;

  it('reads the state file the watchdog actually writes', () => {
    const view = watchView(FIELDS, now);
    expect(view.alive).toBe(true);
    expect(view.pid).toBe('1980822');
    expect(view.manager).toBe('manager-v3');
    expect(view.intervalSeconds).toBe(20);
    expect(view.passes).toBe(2933);
    expect(view.queueDepth).toBe(0);
    expect(view.heartbeatAgeSeconds).toBe(5);
    expect(view.uptimeSeconds).toBe(1790183078 - 1790114430);
    expect(view.lastDelivery).toEqual({ name: 'relay-pwa-ui', scope: 'local', why: 'FINISHED', pane: 'w11:p1' });
  });

  it('reports every scope with its last result and how long ago it was scanned', () => {
    const view = watchView(FIELDS, now);
    expect(view.scopes.map((scope) => scope.name)).toEqual(['local', 'server-1', 'server-2']);
    const broken = view.scopes.find((scope) => scope.name === 'server-2');
    expect(broken?.result).toBe('error');
    expect(broken?.streak).toBe('2');
    expect(broken?.error).toContain('timed out');
    expect(broken?.ageSeconds).toBe(78);
  });

  it('treats a watchdog that never ran as not alive, not as an empty panel', () => {
    const view = watchView({}, now);
    expect(view.alive).toBe(false);
    expect(view.heartbeatAgeSeconds).toBeNull();
    expect(view.scopes).toEqual([]);
  });

  it('splits the log lines into their timestamp and event, newest first', () => {
    const events = watchEvents([
      '2026-09-23T10:43:32-06:00  detectado: FINISHED local/relay-pwa-ui (pane w11:p1)',
      '2026-09-23T10:44:00-06:00  entregado: relay-pwa-ui (FINISHED) via wR:p2  [cola=1]',
    ]);
    expect(events[0].at).toBe('2026-09-23T10:44:00-06:00');
    expect(events[0].text).toContain('entregado');
    expect(events[1].text).toContain('detectado');
  });
});

describe('roles', () => {
  const manager: Agent = { relay_id: 'r', relay_label: 'Laptop', raw_pane_id: 'wR:p2', pane_id: 'wR:p2', name: 'manager-v3', status: 'blocked', cwd: '/repo' } as Agent;
  const front: Agent = { relay_id: 'r', relay_label: 'Laptop', raw_pane_id: 'w1:p1', pane_id: 'w1:p1', name: 'relay-pwa-ui', status: 'working', cwd: '/repo/.worktrees/pwa-machines-ui' } as Agent;
  const idle: Agent = { relay_id: 'r', relay_label: 'Laptop', raw_pane_id: 'w9:p1', pane_id: 'w9:p1', name: 'nadie-lo-anoto', status: 'idle', cwd: '/srv/otra-cosa' } as Agent;
  const tasks: QueueTask[] = [
    { id: 't1', title: 'Tablero', state: 'working', owner: 'relay-pwa-ui', branch: 'feat/pwa-machines-ui' },
    { id: 't2', title: 'Espera al humano', state: 'blocked', owner: 'manager-v3', waits_on_human: true },
    { id: 't3', title: 'Ya hecho', state: 'done', owner: 'relay-pwa-ui' },
  ];

  it('names the manager the watchdog steers, and crosses the rest with the queue', () => {
    const rows = agentRoles([manager, front, idle], tasks, 'manager-v3');
    const byName = Object.fromEntries(rows.map((row) => [row.agent.name, row]));
    // The orchestrator keeps its role even while it waits: that is the point of
    // showing roles next to states instead of merging them.
    expect(byName['manager-v3'].role).toBe('manager');
    expect(byName['manager-v3'].task?.id).toBe('t2');
    expect(byName['relay-pwa-ui'].role).toBe('work');
    expect(byName['relay-pwa-ui'].task?.id).toBe('t1');
    // Nothing on the board claims it: that is "waiting", and it is shown.
    expect(byName['nadie-lo-anoto'].role).toBe('waiting');
    expect(byName['nadie-lo-anoto'].task).toBeNull();
  });

  it('puts the orchestrator first, whatever its live state', () => {
    const rows = agentRoles([idle, front, manager], tasks, 'manager-v3');
    expect(rows[0].agent.name).toBe('manager-v3');
    expect(rows[0].role).toBe('manager');
  });
});
