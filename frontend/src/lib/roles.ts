import type { Agent, QueueTask } from './types';

/**
 * Who directs and who is working on what. The manager comes from the watchdog's
 * own state (it names the agent it steers) and every other row is crossed with
 * the versioned queue. A row that matches no open task is waiting, not
 * idle-by-accident: it is the honest reading of a board that says nothing about
 * it.
 */
export type AgentRole = 'manager' | 'work' | 'blocked' | 'waiting';

export interface AgentRoleRow {
  agent: Agent;
  role: AgentRole;
  task: QueueTask | null;
}

const LIVE_STATUS: Record<string, number> = { blocked: 0, waiting: 1, working: 2, idle: 3, done: 4 };

function openTaskFor(agent: Agent, tasks: QueueTask[]): QueueTask | null {
  const names = [agent.name, agent.session, agent.tab_label, agent.project]
    .filter((value): value is string => typeof value === 'string' && value.trim() !== '')
    .map((value) => value.trim().toLowerCase());
  const paths = [agent.cwd, agent.project, agent.tab_label]
    .filter((value): value is string => typeof value === 'string' && value.trim() !== '')
    .map((value) => value.trim().toLowerCase().replace(/\\/g, '/'));

  for (const task of tasks) {
    if (String(task.state) === 'done') continue;
    const owner = String(task.owner || '').trim().toLowerCase();
    if (owner !== '' && names.includes(owner)) return task;
    const segment = String(task.branch || '').trim().toLowerCase().split('/').filter(Boolean).pop() || '';
    if (segment.length >= 3 && paths.some((path) => path.endsWith(`/${segment}`) || path.includes(`/${segment}/`))) {
      return task;
    }
    const repo = String(task.repo || '').trim().toLowerCase();
    if (repo.length >= 3 && paths.some((path) => path.endsWith(`/${repo}`) || path.includes(`/${repo}/`))) {
      return task;
    }
  }
  return null;
}

/**
 * Who directs and who is working on what: the manager comes from the watchdog's
 * own state (it names the agent it steers), and every other row is crossed with
 * the versioned queue. A row that matches no open task is waiting, not idle-by-
 * accident: it is the honest reading of a board that says nothing about it.
 */
export function agentRoles(agents: Agent[], tasks: QueueTask[], managerName: string): AgentRoleRow[] {
  const manager = managerName.trim().toLowerCase();
  return agents
    .map((agent) => {
      const task = openTaskFor(agent, tasks);
      const isManager = manager !== '' && [agent.name, agent.session, agent.tab_label]
        .filter((value): value is string => typeof value === 'string')
        .some((value) => value.trim().toLowerCase() === manager);
      let role: AgentRole = isManager ? 'manager' : task ? 'work' : 'waiting';
      if (role === 'work' && (String(agent.status || '').toLowerCase() === 'blocked'
        || (task && String(task.state) === 'blocked'))) {
        role = 'blocked';
      }
      return { agent, role, task };
    })
    .sort((left, right) => {
      const rank = (row: AgentRoleRow) => (row.role === 'manager' ? 0 : row.role === 'blocked' ? 1 : row.role === 'work' ? 2 : 3);
      if (rank(left) !== rank(right)) return rank(left) - rank(right);
      const a = LIVE_STATUS[String(left.agent.status || '').toLowerCase()] ?? 5;
      const b = LIVE_STATUS[String(right.agent.status || '').toLowerCase()] ?? 5;
      if (a !== b) return a - b;
      return String(left.agent.name || '').localeCompare(String(right.agent.name || ''));
    });
}
