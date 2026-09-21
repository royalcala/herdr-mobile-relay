import {
  agentLastActiveAt,
  agentActivitySeq,
  agentStatusGroup,
  displayName,
  hostLabel,
  sortedAgents,
  tabName,
} from './agents';
import type { Agent, Machine, RelayWorkspace, WorkspaceWorktree } from './types';

export interface WorkspaceTab {
  id: string;
  label: string;
  number: number;
  order: number;
  agents: Agent[];
}

export interface WorkspaceGroup {
  key: string;
  relayId: string;
  relayLabel: string;
  machineId: string;
  workspaceId: string;
  label: string;
  number: number;
  cwd: string;
  host: string;
  agents: Agent[];
  tabs: WorkspaceTab[];
  tabCount: number;
  paneCount: number;
  worktree: WorkspaceWorktree | null;
  attentionCount: number;
  workingCount: number;
  doneCount: number;
  readyCount: number;
  lastActiveAt: number;
  lastActivitySeq: number;
}

export interface RelayWorkspaceTree {
  workspace: RelayWorkspace;
  children: RelayWorkspace[];
  workspaceIds: string[];
}

export interface WorkspaceGroupTree {
  workspace: WorkspaceGroup;
  children: WorkspaceGroup[];
  aggregate: WorkspaceGroup;
}

function pathBase(path: string): string {
  const normalized = path.replace(/[\\/]+$/, '');
  return normalized.split(/[\\/]/).filter(Boolean).at(-1) || '';
}

/**
 * Provenance text for a workspace card, but only when it says something the
 * card does not already say. "Repository · ibkr" inside a card titled "ibkr"
 * repeats the title, and "Linked worktree" under a parent card repeats what
 * the tree already draws.
 */
export function workspaceProvenance(
  workspace: { label: string; worktree?: WorkspaceWorktree | null },
  nested = false,
): string {
  const worktree = workspace.worktree;
  if (!worktree || !worktree.repo_name) return '';
  if (worktree.is_linked_worktree) return nested ? '' : `Worktree of ${worktree.repo_name}`;
  if (worktree.repo_name.trim().toLowerCase() === workspace.label.trim().toLowerCase()) return '';
  return `Repository · ${worktree.repo_name}`;
}

/**
 * A worktree row's path, but only when it says something the label does not:
 * a checkout directory named after its workspace label carries no extra
 * information, and repeating it on every row buries the rows that differ.
 */
export function informativePath(path: string, label: string): string {
  if (!path) return '';
  return pathBase(path).toLowerCase() === label.trim().toLowerCase() ? '' : path;
}

/**
 * A directory as the relay's own tools print it: `~/code/app` under that
 * computer's home directory, the absolute path anywhere else. Matches the
 * relay's directory browser labels.
 */
export function homeRelativePath(path: string, home: string): string {
  if (!path) return '';
  const base = home.replace(/\/+$/, '');
  if (!base) return path;
  if (path === base) return '~';
  return path.startsWith(`${base}/`) ? `~/${path.slice(base.length + 1)}` : path;
}

export function workspaceIdentity(agent: Agent): string {
  const identity = String(agent.workspace_id || agent.cwd || agent.raw_pane_id || agent.pane_id);
  return `${agent.relay_id}\u0000${agent.machine_id || ''}\u0000${identity}`;
}

function groupLabel(agents: Agent[]): string {
  const projects = [...new Set(agents.map((agent) => String(agent.project || '')).filter(Boolean))];
  if (projects.length === 1) return projects[0];
  const cwdNames = [...new Set(agents.map((agent) => pathBase(String(agent.cwd || ''))).filter(Boolean))];
  if (cwdNames.length === 1) return cwdNames[0];
  const firstTab = agents.map(tabName).find(Boolean);
  return firstTab || 'Workspace';
}

function groupCwd(agents: Agent[]): string {
  const paths = [...new Set(agents.map((agent) => String(agent.cwd || '')).filter(Boolean))];
  if (!paths.length) return '';
  return paths.sort((left, right) => left.length - right.length || left.localeCompare(right))[0];
}

export function workspaceGroups(agents: Agent[], workspaces: RelayWorkspace[] = []): WorkspaceGroup[] {
  const grouped = new Map<string, { agents: Agent[]; workspace: RelayWorkspace | null }>();
  for (const workspace of workspaces) {
    const key = `${workspace.relay_id}\u0000${workspace.machine_id || ''}\u0000${workspace.workspace_id}`;
    grouped.set(key, { agents: [], workspace });
  }
  for (const agent of agents) {
    const key = workspaceIdentity(agent);
    const group = grouped.get(key) || { agents: [], workspace: null };
    group.agents.push(agent);
    grouped.set(key, group);
  }

  const groups = [...grouped].map(([key, value]) => {
    const ordered = sortedAgents(value.agents);
    const first = ordered[0];
    const workspace = value.workspace;
    const tabs = new Map<string, Agent[]>();
    for (const agent of ordered) {
      const id = String(agent.tab_id || agent.pane_id);
      tabs.set(id, [...(tabs.get(id) || []), agent]);
    }
    const tabGroups = [...tabs].map(([id, tabAgents]): WorkspaceTab => ({
      id,
      label: tabName(tabAgents[0]) || displayName(tabAgents[0]),
      number: Number(tabAgents[0].tab_number) || Number.MAX_SAFE_INTEGER,
      // Herdr's visual position; tab numbers are stable identities that
      // never change when a tab moves.
      order: Number(tabAgents[0].tab_order) || Number.MAX_SAFE_INTEGER,
      agents: sortedAgents(tabAgents),
    })).sort((left, right) =>
      left.order - right.order
      || left.number - right.number
      || left.label.localeCompare(right.label));
    const statuses = ordered.map(agentStatusGroup);
    return {
      key,
      relayId: workspace?.relay_id || first?.relay_id || '',
      relayLabel: workspace?.relay_label || first?.relay_label || '',
      machineId: workspace?.machine_id || first?.machine_id || '',
      workspaceId: workspace?.workspace_id || key.slice(key.lastIndexOf('\u0000') + 1),
      number: workspace?.number || Number.MAX_SAFE_INTEGER,
      label: workspace?.label || groupLabel(ordered),
      cwd: workspace?.cwd || groupCwd(ordered),
      host: first ? hostLabel(first) : workspace?.relay_label || '',
      agents: ordered,
      tabs: tabGroups,
      tabCount: Math.max(workspace?.tab_count || 0, tabGroups.length),
      paneCount: Math.max(workspace?.pane_count || 0, ordered.length),
      worktree: workspace?.worktree || null,
      attentionCount: statuses.filter((status) => status === 'blocked' || status === 'attention').length,
      workingCount: statuses.filter((status) => status === 'working').length,
      doneCount: statuses.filter((status) => status === 'done').length,
      readyCount: statuses.filter((status) => status === 'ready').length,
      lastActiveAt: Math.max(0, ...ordered.map(agentLastActiveAt)),
      lastActivitySeq: Math.max(0, ...ordered.map(agentActivitySeq)),
    };
  });

  return groups.sort((left, right) =>
    right.attentionCount - left.attentionCount
    || right.lastActiveAt - left.lastActiveAt
    || (left.relayId === right.relayId ? right.lastActivitySeq - left.lastActivitySeq : 0)
    || left.label.localeCompare(right.label, undefined, { sensitivity: 'base' })
    || left.host.localeCompare(right.host, undefined, { sensitivity: 'base' }));
}

export function relayWorkspaceTrees(workspaces: RelayWorkspace[]): RelayWorkspaceTree[] {
  const identity = (workspace: RelayWorkspace): string =>
    `${workspace.relay_id}\u0000${workspace.machine_id || ''}\u0000${workspace.workspace_id}`;
  const parentByRepo = new Map<string, RelayWorkspace>();
  for (const workspace of workspaces) {
    const worktree = workspace.worktree;
    if (worktree && !worktree.is_linked_worktree && worktree.repo_key) {
      parentByRepo.set(`${workspace.relay_id}\u0000${workspace.machine_id || ''}\u0000${worktree.repo_key}`, workspace);
    }
  }
  const children = new Map<string, RelayWorkspace[]>();
  const childIDs = new Set<string>();
  for (const workspace of workspaces) {
    const worktree = workspace.worktree;
    if (!worktree?.is_linked_worktree || !worktree.repo_key) continue;
    const parent = parentByRepo.get(`${workspace.relay_id}\u0000${workspace.machine_id || ''}\u0000${worktree.repo_key}`);
    if (!parent || parent.workspace_id === workspace.workspace_id) continue;
    const key = identity(parent);
    children.set(key, [...(children.get(key) || []), workspace]);
    childIDs.add(identity(workspace));
  }
  return workspaces
    .filter((workspace) => !childIDs.has(identity(workspace)))
    .map((workspace) => {
      const key = identity(workspace);
      const nested = (children.get(key) || []).sort((left, right) =>
        left.number - right.number || left.label.localeCompare(right.label));
      return {
        workspace,
        children: nested,
        workspaceIds: [workspace.workspace_id, ...nested.map((child) => child.workspace_id)],
      };
    });
}

export function workspaceGroupTrees(groups: WorkspaceGroup[]): WorkspaceGroupTree[] {
  const groupKey = (group: { relayId: string; machineId: string; workspaceId: string }): string =>
    `${group.relayId}\u0000${group.machineId || ''}\u0000${group.workspaceId}`;
  const workspaces = groups.map((group): RelayWorkspace => ({
    relay_id: group.relayId,
    relay_label: group.relayLabel,
    machine_id: group.machineId || undefined,
    workspace_id: group.workspaceId,
    number: group.number,
    label: group.label,
    focused: false,
    pane_count: group.paneCount,
    tab_count: group.tabCount,
    active_tab_id: '',
    agent_status: '',
    cwd: group.cwd,
    worktree: group.worktree,
  }));
  const groupByID = new Map(groups.map((group) => [groupKey(group), group]));
  return relayWorkspaceTrees(workspaces).map((tree) => {
    const workspace = groupByID.get(groupKey({
      relayId: tree.workspace.relay_id,
      machineId: tree.workspace.machine_id || '',
      workspaceId: tree.workspace.workspace_id,
    }))!;
    const children = tree.children
      .map((child) => groupByID.get(groupKey({
        relayId: child.relay_id,
        machineId: child.machine_id || '',
        workspaceId: child.workspace_id,
      })))
      .filter((child): child is WorkspaceGroup => Boolean(child));
    const all = [workspace, ...children];
    return {
      workspace,
      children,
      aggregate: {
        ...workspace,
        agents: all.flatMap((group) => group.agents),
        tabCount: all.reduce((total, group) => total + group.tabCount, 0),
        paneCount: all.reduce((total, group) => total + group.paneCount, 0),
        attentionCount: all.reduce((total, group) => total + group.attentionCount, 0),
        workingCount: all.reduce((total, group) => total + group.workingCount, 0),
        doneCount: all.reduce((total, group) => total + group.doneCount, 0),
        readyCount: all.reduce((total, group) => total + group.readyCount, 0),
        lastActiveAt: Math.max(...all.map((group) => group.lastActiveAt)),
        lastActivitySeq: Math.max(...all.map((group) => group.lastActivitySeq)),
      },
    };
  });
}

/**
 * The most notable session state in a mixed workspace card, by the
 * done > working > idle precedence.
 */
export function workspaceStateTone(group: WorkspaceGroup): 'success' | 'warning' | 'muted' {
  if (group.doneCount) return 'success';
  if (group.workingCount) return 'warning';
  return 'muted';
}

export function workspaceMetadataSearchText(group: WorkspaceGroup): string {
  return [
    group.label,
    group.cwd,
    group.host,
    group.relayLabel,
    group.workspaceId,
  ].join(' ').toLocaleLowerCase();
}

export function workspaceSearchText(group: WorkspaceGroup): string {
  return [
    group.label,
    group.cwd,
    group.host,
    group.relayLabel,
    group.workspaceId,
    ...group.tabs.flatMap((tab) => [tab.label, ...tab.agents.flatMap((agent) => [
      displayName(agent),
      String(agent.agent || ''),
      String(agent.session || ''),
    ])]),
  ].join(' ').toLocaleLowerCase();
}

export function agentSearchText(agent: Agent): string {
  return [
    displayName(agent),
    String(agent.agent || ''),
    String(agent.project || ''),
    String(agent.cwd || ''),
    String(agent.session || ''),
    String(agent.tab_label || ''),
    String(agent.workspace_id || ''),
    hostLabel(agent),
    agent.relay_label,
  ].join(' ').toLocaleLowerCase();
}

export interface MachineGroup {
  key: string;
  relayId: string;
  relayLabel: string;
  machineId: string;
  label: string;
  host: string;
  local: boolean;
  reachable: boolean;
  error: string;
  trees: WorkspaceGroupTree[];
  attentionCount: number;
  workingCount: number;
  agentCount: number;
}

/**
 * Buckets workspace trees by machine so the sidebar reads like the desktop
 * panel: one group per machine, the local one first, each keeping its own
 * workspaces. Machine IDs are scoped to a server, so the bucket key carries
 * both the relay and the machine.
 */
export function machineGroups(
  trees: WorkspaceGroupTree[],
  machines: Machine[] = [],
): MachineGroup[] {
  const meta = new Map<string, Machine>();
  for (const machine of machines) meta.set(machine.machine_id, machine);
  const grouped = new Map<string, MachineGroup>();
  for (const tree of trees) {
    const group = tree.workspace;
    const machineId = group.machineId || '';
    const key = `${group.relayId}\u0000${machineId}`;
    let bucket = grouped.get(key);
    if (!bucket) {
      const machine = meta.get(machineId);
      const local = machineId === '' || machineId === 'local';
      bucket = {
        key,
        relayId: group.relayId,
        relayLabel: group.relayLabel,
        machineId,
        label: machine?.label || group.relayLabel || machineId || 'relay',
        host: machine?.host || group.host,
        local,
        reachable: machine ? machine.reachable !== false : true,
        error: machine?.error || '',
        trees: [],
        attentionCount: 0,
        workingCount: 0,
        agentCount: 0,
      };
      grouped.set(key, bucket);
    }
    bucket.trees.push(tree);
    bucket.attentionCount += tree.aggregate.attentionCount;
    bucket.workingCount += tree.aggregate.workingCount;
    bucket.agentCount += tree.aggregate.agents.length;
  }
  return [...grouped.values()].sort((left, right) =>
    Number(right.local) - Number(left.local)
    || left.label.localeCompare(right.label, undefined, { sensitivity: 'base' }));
}

export interface MachineWorkspaceBucket {
  key: string;
  relayId: string;
  relayLabel: string;
  machineId: string;
  label: string;
  host: string;
  local: boolean;
  reachable: boolean;
  error: string;
  groups: WorkspaceGroup[];
  attentionCount: number;
  workingCount: number;
  agentCount: number;
}

/** The flat-list twin of machineGroups, for surfaces that never build trees. */
export function machineWorkspaceBuckets(
  groups: WorkspaceGroup[],
  machines: Machine[] = [],
): MachineWorkspaceBucket[] {
  const meta = new Map<string, Machine>();
  for (const machine of machines) meta.set(machine.machine_id, machine);
  const grouped = new Map<string, MachineWorkspaceBucket>();
  for (const group of groups) {
    const machineId = group.machineId || '';
    const key = `${group.relayId}\u0000${machineId}`;
    let bucket = grouped.get(key);
    if (!bucket) {
      const machine = meta.get(machineId);
      const local = machineId === '' || machineId === 'local';
      bucket = {
        key,
        relayId: group.relayId,
        relayLabel: group.relayLabel,
        machineId,
        label: machine?.label || group.relayLabel || machineId || 'relay',
        host: machine?.host || group.host,
        local,
        reachable: machine ? machine.reachable !== false : true,
        error: machine?.error || '',
        groups: [],
        attentionCount: 0,
        workingCount: 0,
        agentCount: 0,
      };
      grouped.set(key, bucket);
    }
    bucket.groups.push(group);
    bucket.attentionCount += group.attentionCount;
    bucket.workingCount += group.workingCount;
    bucket.agentCount += group.agents.length;
  }
  return [...grouped.values()].sort((left, right) =>
    Number(right.local) - Number(left.local)
    || left.label.localeCompare(right.label, undefined, { sensitivity: 'base' }));
}
