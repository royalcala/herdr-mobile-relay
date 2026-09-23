<script lang="ts">
  import type { HerdrSession, RelayConfig, RelayConnectionView } from '$lib/types';

  /**
   * Which herdr session each connected computer is mirroring, and what else that
   * computer has. A session is a whole world of workspaces and agents: the relay
   * can only be inside one at a time, so this is where you move it.
   */
  let {
    relays,
    sessions,
    activeSessions,
    connections,
    onselect,
  }: {
    relays: RelayConfig[];
    sessions: Map<string, HerdrSession[]>;
    activeSessions: Map<string, string>;
    connections: Map<string, RelayConnectionView>;
    onselect: (relayId: string, name: string) => void;
  } = $props();

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
          <article class={`board-card ${current ? 'state-working' : ''}`}>
            <header class="board-card-head">
              <h3>{session.name}</h3>
              <span class={`board-state ${current ? 'state-working' : ''}`}>
                {current ? 'mirroring' : session.running ? 'available' : 'stopped'}
              </span>
            </header>
            <p class="board-meta">
              {#if session.default}<span>default session</span>{/if}
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
