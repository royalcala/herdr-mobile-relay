import { agentStatusGroup } from './agents';
import type { Agent, Machine, RelayWorkspace } from './types';

/** Machine ids are per-server, so pairing the relay with the id is the identity. */
export type MachineKey = string;

/**
 * A saved machine normalizes to `local` when it carries no id, so a remote
 * machine and the relay's own host never share a bucket.
 */
export function normalizeMachineId(machineId?: string): string {
  const id = String(machineId || '');
  return id && id !== 'local' ? id : 'local';
}

export function machineKey(relayId: string, machineId?: string): MachineKey {
  return `${relayId}\u0000${normalizeMachineId(machineId)}`;
}

/**
 * Machines are reported per relay, but callers often hold a single flat list.
 * The relay-scoped entry wins; an unqualified id still resolves so a machine
 * list from before the relay tag existed keeps labelling its rail.
 */
export interface MachineIndex {
  get(relayId: string, machineId?: string): Machine | undefined;
}

export function buildMachineIndex(machines: Machine[] = []): MachineIndex {
  const scoped = new Map<string, Machine>();
  const unscoped = new Map<string, Machine>();
  for (const machine of machines) {
    const id = normalizeMachineId(machine.machine_id);
    if (machine.relay_id) scoped.set(`${machine.relay_id}\u0000${id}`, machine);
    if (!unscoped.has(id)) unscoped.set(id, machine);
  }
  return {
    get(relayId, machineId) {
      const id = normalizeMachineId(machineId);
      return scoped.get(`${relayId}\u0000${id}`) ?? unscoped.get(id);
    },
  };
}

/** Everything the UI needs to name and qualify one machine. */
export interface MachineRef {
  key: MachineKey;
  relayId: string;
  relayLabel: string;
  machineId: string;
  label: string;
  host: string;
  local: boolean;
  reachable: boolean;
  /** Non-local machines are mirrored for reading; reader-role devices too. */
  readOnly: boolean;
  error: string;
}

export function machineRef(
  index: MachineIndex,
  relayId: string,
  relayLabel: string,
  machineId: string | undefined,
  fallbackHost = '',
  readOnlyRelays: ReadonlySet<string> = new Set(),
  fallbackLabel = '',
): MachineRef {
  const id = normalizeMachineId(machineId);
  const machine = index.get(relayId, id);
  const local = id === 'local';
  return {
    key: machineKey(relayId, id),
    relayId,
    relayLabel,
    machineId: id,
    label: machine?.label || fallbackLabel || relayLabel || id || 'relay',
    host: machine?.host || fallbackHost,
    local,
    reachable: machine ? machine.reachable !== false : true,
    readOnly: !local || readOnlyRelays.has(relayId),
    error: machine?.error || '',
  };
}

export function agentMachineRef(
  agent: Partial<Agent>,
  machines: Machine[] = [],
  readOnlyRelays: ReadonlySet<string> = new Set(),
): MachineRef {
  return machineRef(
    buildMachineIndex(machines),
    String(agent.relay_id || ''),
    String(agent.relay_label || ''),
    agent.machine_id,
    String(agent.host || ''),
    readOnlyRelays,
  );
}

/** `local` for the relay's own host, `remote`/`offline` for a mirrored machine. */
export function machineTag(ref: MachineRef): 'local' | 'remote' | 'offline' {
  if (ref.local) return 'local';
  return ref.reachable ? 'remote' : 'offline';
}

export interface MachineCounts {
  total: number;
  blocked: number;
  working: number;
  done: number;
  idle: number;
}

export interface MachineSection {
  key: MachineKey;
  ref: MachineRef;
  agents: Agent[];
  workspaces: RelayWorkspace[];
  counts: MachineCounts;
}

function emptyCounts(): MachineCounts {
  return { total: 0, blocked: 0, working: 0, done: 0, idle: 0 };
}

/**
 * Buckets a relay's agents and workspaces by the machine hosting them so the
 * home screen can render one section per machine, local first. Machines the
 * relay reported but that host nothing yet still get a section, because an
 * unreachable machine is exactly what the user needs to see.
 */
export function groupAgentsByMachine(
  agents: Agent[],
  workspaces: RelayWorkspace[] = [],
  machines: Machine[] = [],
  readOnlyRelays: ReadonlySet<string> = new Set(),
): MachineSection[] {
  const index = buildMachineIndex(machines);
  const sections = new Map<MachineKey, MachineSection>();
  const ensure = (ref: MachineRef): MachineSection => {
    let section = sections.get(ref.key);
    if (!section) {
      section = { key: ref.key, ref, agents: [], workspaces: [], counts: emptyCounts() };
      sections.set(ref.key, section);
    }
    return section;
  };

  for (const agent of agents) {
    const ref = machineRef(
      index,
      String(agent.relay_id || ''),
      String(agent.relay_label || ''),
      agent.machine_id,
      String(agent.host || ''),
      readOnlyRelays,
    );
    const section = ensure(ref);
    section.agents.push(agent);
    section.counts.total += 1;
    const group = agentStatusGroup(agent);
    if (group === 'blocked' || group === 'attention') section.counts.blocked += 1;
    else if (group === 'working') section.counts.working += 1;
    else if (group === 'done') section.counts.done += 1;
    else section.counts.idle += 1;
  }

  for (const workspace of workspaces) {
    const ref = machineRef(
      index,
      workspace.relay_id,
      workspace.relay_label,
      workspace.machine_id,
      '',
      readOnlyRelays,
    );
    ensure(ref).workspaces.push(workspace);
  }

  for (const machine of machines) {
    // Without a relay tag a machine can not be placed, so it is skipped rather
    // than merged into an unrelated relay's section.
    if (!machine.relay_id) continue;
    const ref = machineRef(index, machine.relay_id, '', machine.machine_id, machine.host || '', readOnlyRelays, machine.label);
    ensure(ref);
  }

  return [...sections.values()].sort((left, right) =>
    Number(right.ref.local) - Number(left.ref.local)
    || left.ref.label.localeCompare(right.ref.label, undefined, { sensitivity: 'base' }));
}

export type StatusFilterKey = 'all' | 'blocked' | 'working' | 'done' | 'idle';

export interface StatusFilterOption {
  key: StatusFilterKey;
  label: string;
  tone: 'danger' | 'warning' | 'success' | 'muted';
}

export const STATUS_FILTERS: readonly StatusFilterOption[] = [
  { key: 'all', label: 'All', tone: 'muted' },
  { key: 'blocked', label: 'Blocked', tone: 'danger' },
  { key: 'working', label: 'Working', tone: 'warning' },
  { key: 'done', label: 'Done', tone: 'success' },
  { key: 'idle', label: 'Idle', tone: 'muted' },
];
