import type { Agent, QueueTask } from './types';

/**
 * The orchestrator board crosses three sources that live in different places:
 *
 *  - the versioned queue in the repo (queue/tasks.json): what is planned, who
 *    owns it, which branch and commit it was left on, and what blocks it;
 *  - the live herdr agents the relay mirrors: what is actually running, on
 *    which pane, and in which state;
 *  - the connection view (production): which relay build the phone is talking
 *    to and whether a deployment is in flight.
 *
 * The crossing is deliberately heuristic. An agent is matched to a task by the
 * owner name it was given in herdr, by the branch checked out in its working
 * directory (herdr worktrees are named after the branch), or by the repository
 * in the path. Anything that matches nothing is surfaced as "off the board"
 * rather than hidden: work that nobody wrote down is exactly what a board has
 * to show.
 */

export interface CrossedTask {
  task: QueueTask;
  agents: Agent[];
}

export interface Orchestration {
  /** Blocked, or explicitly waiting on a person. */
  waitsOnHuman: CrossedTask[];
  /** Live work: working, in review, or queued. */
  inFlight: CrossedTask[];
  /** Finished rows, kept for context. */
  done: CrossedTask[];
  /** Running agents whose work is not written down on the board. */
  offBoard: Agent[];
}

const LIVE_STATUS_WEIGHT: Record<string, number> = {
  blocked: 0,
  waiting: 1,
  working: 2,
  idle: 3,
  done: 4,
};

export function branchSegment(branch: string | undefined): string {
  return String(branch || '')
    .trim()
    .toLowerCase()
    .split('/')
    .filter(Boolean)
    .pop() || '';
}

function agentNames(agent: Agent): string[] {
  return [agent.name, agent.session, agent.session_name, agent.project, agent.tab_label]
    .filter((value): value is string => typeof value === 'string' && value.trim() !== '')
    .map((value) => value.trim().toLowerCase());
}

function agentPaths(agent: Agent): string[] {
  return [agent.cwd, agent.project, agent.tab_label]
    .filter((value): value is string => typeof value === 'string' && value.trim() !== '')
    .map((value) => value.trim().toLowerCase().replace(/\\/g, '/'));
}

/** True when this agent is working on this task. */
export function taskMatchesAgent(task: QueueTask, agent: Agent): boolean {
  const owner = String(task.owner || '').trim().toLowerCase();
  if (owner !== '' && agentNames(agent).includes(owner)) return true;

  // A path segment has to match whole: `/main` must not match `/maintenance`.
  const segment = branchSegment(task.branch);
  if (segment.length >= 3) {
    for (const path of agentPaths(agent)) {
      if (path.endsWith(`/${segment}`) || path.includes(`/${segment}/`)) return true;
    }
  }

  const repo = String(task.repo || '').trim().toLowerCase();
  if (repo.length >= 3) {
    for (const path of agentPaths(agent)) {
      if (path.endsWith(`/${repo}`) || path.includes(`/${repo}/`)) return true;
    }
  }
  return false;
}

/** The most alarming live status among a task's agents, if any. */
export function liveStatus(agents: Agent[]): string {
  let best = '';
  let bestWeight = Number.POSITIVE_INFINITY;
  for (const agent of agents) {
    const status = String(agent.status || '').toLowerCase();
    const weight = LIVE_STATUS_WEIGHT[status] ?? 5;
    if (weight < bestWeight) {
      bestWeight = weight;
      best = status;
    }
  }
  return best;
}

function isDone(task: QueueTask): boolean {
  return String(task.state || '').toLowerCase() === 'done';
}

function waitsOnHuman(task: QueueTask): boolean {
  return task.waits_on_human === true || String(task.state || '').toLowerCase() === 'blocked';
}

export function buildOrchestration(rows: QueueTask[], agents: Agent[]): Orchestration {
  const crossed: CrossedTask[] = rows.map((task) => ({
    task,
    agents: agents.filter((agent) => taskMatchesAgent(task, agent)),
  }));

  const claimed = new Set<string>();
  for (const row of crossed) {
    for (const agent of row.agents) claimed.add(`${agent.relay_id}:${agent.pane_id}`);
  }

  return {
    // Waiting on a person outranks everything else: it is the only column the
    // human can act on.
    waitsOnHuman: crossed.filter((row) => waitsOnHuman(row.task)),
    inFlight: crossed.filter((row) => !waitsOnHuman(row.task) && !isDone(row.task)),
    done: crossed.filter((row) => !waitsOnHuman(row.task) && isDone(row.task)),
    offBoard: agents
      .filter((agent) => !claimed.has(`${agent.relay_id}:${agent.pane_id}`))
      .sort((left, right) => {
        const a = LIVE_STATUS_WEIGHT[String(left.status || '').toLowerCase()] ?? 5;
        const b = LIVE_STATUS_WEIGHT[String(right.status || '').toLowerCase()] ?? 5;
        if (a !== b) return a - b;
        return String(left.name || '').localeCompare(String(right.name || ''));
      }),
  };
}
