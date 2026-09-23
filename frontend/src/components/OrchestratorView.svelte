<script lang="ts">
  import { buildOrchestration, liveStatus, type CrossedTask } from '$lib/orchestration';
  import type { Agent, QueueBoard, QueueTask, RelayConfig, RelayConnectionView } from '$lib/types';
  import { agentRoles } from '$lib/roles';
  import type { WatchFields } from '$lib/watch';

  /**
   * The thin board: the versioned queue crossed with what herdr is doing right
   * now, plus the production state of the relays behind the phone. It answers
   * three questions in one screen — who works on what, who is blocked, and what
   * is waiting for a person.
   *
   * Everything arrives as a prop: this view is loaded on demand, so it must not
   * pull the store (and the application state behind it) into its own chunk.
   */
  let {
    tasks,
    board,
    agents,
    relays,
    connections,
    onwatch,
    activeSessions,
    onopen,
    onrefresh,
  }: {
    tasks: QueueTask[];
    board: QueueBoard;
    agents: Agent[];
    relays: RelayConfig[];
    connections: Map<string, RelayConnectionView>;
    onwatch: (relayId: string) => Promise<{ fields: WatchFields; events: string[] } | null>;
    activeSessions: Map<string, string>;
    onopen: (agent: Agent) => void;
    onrefresh: () => void;
  } = $props();

  // The watchdog names the agent it steers: the manager is not a guess.
  let watchFields = $state<WatchFields>({});
  const watchRelayId = $derived(
    relays.find((relay) => connections.get(relay.id)?.status === 'connected')?.id || relays[0]?.id || '',
  );
  $effect(() => {
    const relayId = watchRelayId;
    if (!relayId) return;
    let live = true;
    const ask = async () => {
      try {
        const reading = await onwatch(relayId);
        if (live && reading) watchFields = reading.fields;
      } catch {
        // Without a reading the roles fall back to the board alone.
      }
    };
    void ask();
    const timer = setInterval(() => { void ask(); }, 10_000);
    return () => { live = false; clearInterval(timer); };
  });
  const managerName = $derived(watchFields.manager || '');
  const roleRows = $derived(agentRoles(agents, tasks, managerName));
  const sessionLabels = $derived([...activeSessions.values()].filter(Boolean));
  const sessionLabel = $derived(sessionLabels.length ? sessionLabels.join(', ') : 'not reported yet');

  const roleLabel: Record<string, string> = {
    manager: 'orchestrator',
    work: 'work front',
    blocked: 'blocked',
    waiting: 'waiting',
  };

  const boardView = $derived(buildOrchestration(tasks, agents));

  const statusLabel: Record<string, string> = {
    blocked: 'blocked',
    waiting: 'waiting',
    working: 'working',
    done: 'done',
    idle: 'idle',
  };

  function stateClass(task: QueueTask): string {
    if (task.waits_on_human) return 'human';
    if (String(task.state) === 'blocked') return 'blocked';
    if (String(task.state) === 'done') return 'done';
    return String(task.state || 'queued');
  }

  function agentLabel(agent: Agent): string {
    return agent.name || agent.tab_label || agent.project || agent.pane_id;
  }

  function deployLabel(state: string | undefined): string {
    switch (state) {
      case 'deploying': return 'deploying';
      case 'pending_restart': return 'restart pending';
      case 'failed': return 'deploy failed';
      case 'up_to_date': return 'up to date';
      case 'behind': return 'update available';
      default: return state || 'unknown';
    }
  }
</script>

<main class="page orchestrator" aria-label="Orchestration board">
  <header class="orchestrator-header">
    <div>
      <h1>Orchestration</h1>
      <p class="orchestrator-sub">
        {tasks.length} on the board ·
        {boardView.waitsOnHuman.length} waiting on a person ·
        {boardView.inFlight.length} in flight ·
        {boardView.offBoard.length} agents off the board
      </p>
    </div>
    <button type="button" class="board-entry" onclick={onrefresh}>Refresh</button>
  </header>

  {#if !board.available}
    <p class="orchestrator-note" role="status">
      No task board is being served{board.reason ? `: ${board.reason}` : ''}.
      {#if board.path}<span class="orchestrator-path">{board.path}</span>{/if}
    </p>
  {/if}

  {#snippet taskCard(row: CrossedTask)}
    {@const task = row.task}
    <article class={`board-card state-${stateClass(task)}`}>
      <header class="board-card-head">
        <h3>{task.title}</h3>
        <span class={`board-state state-${stateClass(task)}`}>
          {task.waits_on_human ? 'waiting on you' : task.state || 'queued'}
        </span>
      </header>
      <p class="board-meta">
        {#if task.repo}<span class="board-repo">{task.repo}</span>{/if}
        {#if task.branch}<span class="board-branch">{task.branch}</span>{/if}
        {#if task.owner}<span class="board-owner">@{task.owner}</span>{/if}
        {#if task.last_commit}<span class="board-commit">{task.last_commit}</span>{/if}
      </p>
      {#if task.blocked_by}
        <p class="board-blocked"><strong>Blocked by</strong> {task.blocked_by}</p>
      {/if}
      {#if task.notes}
        <p class="board-notes">{task.notes}</p>
      {/if}
      {#if row.agents.length}
        <div class="board-agents" role="group" aria-label="Live agents on this task">
          {#each row.agents as agent (agent.relay_id + agent.pane_id)}
            <button type="button" class="board-agent" onclick={() => onopen(agent)}>
              <span class={`agent-dot status-${String(agent.status || 'idle').toLowerCase()}`} aria-hidden="true"></span>
              <span class="board-agent-name">{agentLabel(agent)}</span>
              <span class="board-agent-state">{statusLabel[String(agent.status || '').toLowerCase()] || agent.status || 'idle'}</span>
            </button>
          {/each}
        </div>
      {:else}
        <p class="board-unmatched" role="status">
          No live agent on this task
          {#if String(task.state) !== 'done'}· nobody is on it right now{/if}
        </p>
      {/if}
    </article>
  {/snippet}

  <section class="board-section" aria-labelledby="board-roles">
    <h2 id="board-roles">Roles</h2>
    <p class="board-hint">
      Manager <strong>{managerName || 'unknown'}</strong> · session <strong>{sessionLabel}</strong>
    </p>
    {#each roleRows as row (row.agent.relay_id + row.agent.pane_id)}
      <button type="button" class={`board-card off-board role-${row.role}`} onclick={() => onopen(row.agent)}>
        <span class={`agent-dot status-${String(row.agent.status || 'idle').toLowerCase()}`} aria-hidden="true"></span>
        <span class="board-agent-name">{row.agent.name || row.agent.tab_label || row.agent.pane_id}</span>
        <span class={`board-state state-${row.role === 'manager' ? 'working' : row.role === 'blocked' ? 'blocked' : row.role === 'work' ? 'working' : 'done'}`}>
          {roleLabel[row.role] || row.role}
        </span>
        {#if row.task}
          <span class="board-meta-inline">{row.task.title} · {row.task.state}{row.task.branch ? ` · ${row.task.branch}` : ''}</span>
        {:else}
          <span class="board-meta-inline">no open row on the board</span>
        {/if}
      </button>
    {/each}
  </section>

  <section class="board-section" aria-labelledby="board-human">
    <h2 id="board-human">Waiting on a person</h2>
    {#if boardView.waitsOnHuman.length}
      {#each boardView.waitsOnHuman as row (row.task.id)}
        {@render taskCard(row)}
      {/each}
    {:else}
      <p class="board-empty" role="status">Nothing is waiting for you.</p>
    {/if}
  </section>

  <section class="board-section" aria-labelledby="board-flight">
    <h2 id="board-flight">In flight</h2>
    {#if boardView.inFlight.length}
      {#each boardView.inFlight as row (row.task.id)}
        {@render taskCard(row)}
      {/each}
    {:else}
      <p class="board-empty" role="status">No open rows on the board.</p>
    {/if}
  </section>

  {#if boardView.offBoard.length}
    <section class="board-section" aria-labelledby="board-off">
      <h2 id="board-off">Agents off the board</h2>
      <p class="board-hint">Running in herdr with no row in the queue.</p>
      {#each boardView.offBoard as agent (agent.relay_id + agent.pane_id)}
        <button type="button" class="board-card off-board" onclick={() => onopen(agent)}>
          <span class={`agent-dot status-${String(agent.status || 'idle').toLowerCase()}`} aria-hidden="true"></span>
          <span class="board-agent-name">{agentLabel(agent)}</span>
          <span class="board-agent-state">{liveStatus([agent]) || 'idle'}</span>
          <span class="board-meta-inline">{agent.project || agent.cwd || ''}</span>
        </button>
      {/each}
    </section>
  {/if}

  <section class="board-section" aria-labelledby="board-gate">
    <h2 id="board-gate">Gate and production</h2>
    <article class="board-card">
      <header class="board-card-head"><h3>Production</h3></header>
      <ul class="board-list">
        {#each relays as relay (relay.id)}
          {@const connection = connections.get(relay.id)}
          <li>
            <span class="board-owner">{relay.label || relay.id}</span>
            <span class="board-meta-inline">
              relay {connection?.releaseVersion || connection?.version || 'unknown'} ·
              inventory {connection?.inventory.state || 'unknown'} ·
              {deployLabel(connection?.appDeploy?.state)}
            </span>
          </li>
        {/each}
      </ul>
    </article>
    <article class="board-card">
      <header class="board-card-head"><h3>Writer gate</h3></header>
      <p class="board-hint">
        The relay does not hold the cluster credentials, so the writer lease in the
        Durable Object is not observable from here. Rows that depend on it are on the
        board under their own ids.
      </p>
    </article>
  </section>

  {#if boardView.done.length}
    <section class="board-section" aria-labelledby="board-done">
      <h2 id="board-done">Done</h2>
      {#each boardView.done as row (row.task.id)}
        {@render taskCard(row)}
      {/each}
    </section>
  {/if}
</main>
