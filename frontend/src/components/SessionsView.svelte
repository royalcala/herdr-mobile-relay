<script lang="ts">
  import type { HerdrSession, RelayConfig, RelayConnectionView } from '$lib/types';

  /**
   * Which herdr session each connected computer is mirroring, and what else that
   * computer has. A session is a whole world of workspaces and agents: the relay
   * can only be inside one at a time, so this is where you move it.
   */
  import { onMount } from 'svelte';

  let {
    relays,
    sessions,
    activeSessions,
    connections,
    onselect,
    onread,
  }: {
    relays: RelayConfig[];
    sessions: Map<string, HerdrSession[]>;
    activeSessions: Map<string, string>;
    connections: Map<string, RelayConnectionView>;
    onselect: (relayId: string, name: string) => void;
    onread: (relayId: string, request: Record<string, unknown>) => Promise<Record<string, any> | null>;
  } = $props();

  /** One reading per session, keyed `relay:session`: agents, panes and board. */
  let readings = $state<Record<string, Record<string, any>>>({});

  async function readAll() {
    for (const row of rows) {
      if (!row.connected) continue;
      for (const session of row.list) {
        try {
          const reading = await onread(row.relay.id, { type: 'session_snapshot', name: session.name });
          if (reading) readings = { ...readings, [`${row.relay.id}:${session.name}`]: reading };
        } catch {
          // A session that cannot be read simply keeps what it had.
        }
      }
    }
  }

  onMount(() => {
    void readAll();
    const timer = setInterval(() => void readAll(), 15_000);
    return () => clearInterval(timer);
  });

  let switching = $state('');

  const rows = $derived(relays.map((relay) => ({
    relay,
    list: sessions.get(relay.id) || [],
    active: activeSessions.get(relay.id) || '',
    connected: connections.get(relay.id)?.status === 'connected',
  })));

  async function pick(relayId: string, name: string) {
    if (switching) return;
    switching = `${relayId}:${name}`;
    try {
      onselect(relayId, name);
    } finally {
      switching = '';
    }
  }
</script>

<main class="page sessions-view" aria-label="Herdr sessions">
  <header class="orchestrator-header">
    <div>
      <h1>Herdr sessions</h1>
      <p class="orchestrator-sub">The relay mirrors one session at a time. Pick another to move it.</p>
    </div>
  </header>

  {#if !relays.length}
    <p class="board-empty" role="status">No computer is configured yet.</p>
  {/if}

  {#each rows as row (row.relay.id)}
    <section class="board-section" aria-labelledby={`session-relay-${row.relay.id}`}>
      <h2 id={`session-relay-${row.relay.id}`}>
        {row.relay.label || row.relay.id}
        <span class="board-meta-inline">{row.connected ? 'connected' : 'offline'}</span>
      </h2>
      {#if row.list.length}
        {#each row.list as session (session.name)}
          {@const current = session.name === row.active || session.active}
          {@const reading = readings[`${row.relay.id}:${session.name}`]}
          <article class={`board-card ${current ? 'state-working' : ''}`}>
            <header class="board-card-head">
              <h3>{session.label || session.name}</h3>
              <span class={`board-state ${current ? 'state-working' : ''}`}>
                {current ? 'mirroring' : session.running ? 'available' : 'stopped'}
              </span>
            </header>
            <p class="board-meta">
              <span class="board-meta-inline">{session.name}</span>
              {#if !session.registered}
                <span class="board-state state-human">no registry entry · defaults</span>
              {/if}
              {#if session.default}<span class="board-meta-inline">default session</span>{/if}
            </p>
            {#if reading}
              <p class="board-meta">
                <span class="board-meta-inline">{reading.agent_count ?? (reading.agents || []).length} agents</span>
                <span class="board-meta-inline">{reading.pane_count ?? (reading.panes || []).length} panes</span>
                {#if reading.board?.available}
                  <span class="board-meta-inline">board {reading.board.open ?? 0} open</span>
                {/if}
                {#if reading.manager}<span class="board-meta-inline">manager {reading.manager}</span>{/if}
              </p>
              {#if reading.agents?.length}
                <p class="board-meta">
                  {#each reading.agents.slice(0, 4) as agent (agent.pane_id)}
                    <span class="board-meta-inline">{agent.name || agent.pane_id} · {agent.agent_status || 'unknown'}</span>
                  {/each}
                </p>
              {/if}
              {#if reading.read_error}
                <p class="board-hint">{reading.read_error}</p>
              {/if}
            {/if}
            <p class="board-meta">
              <span class="board-meta-inline">{session.dir || ''}</span>
            </p>
            {#if !current}
              <button
                type="button"
                class="board-entry"
                disabled={!session.running || switching !== ''}
                onclick={() => pick(row.relay.id, session.name)}
              >Mirror this session</button>
            {/if}
          </article>
        {/each}
      {:else}
        <p class="board-empty" role="status">
          This computer has not reported its sessions yet.
        </p>
      {/if}
    </section>
  {/each}
</main>
