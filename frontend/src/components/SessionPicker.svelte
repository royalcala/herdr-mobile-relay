<script lang="ts">
  import { relayStore } from '$lib/store';
  import type { HerdrSession, RelayConfig } from '$lib/types';

  /**
   * Herdr keeps every named session on its own socket, so a relay can only be
   * inside one of them at a time. This is the switch: it shows which session
   * each connected computer is mirroring and moves it when tapped.
   */
  let {
    relays,
    sessions,
    activeSessions,
  }: {
    relays: RelayConfig[];
    sessions: Map<string, HerdrSession[]>;
    activeSessions: Map<string, string>;
  } = $props();

  let switching = $state('');

  const rows = $derived(
    relays
      .map((relay) => ({ relay, list: sessions.get(relay.id) || [] }))
      .filter((row) => row.list.length > 1),
  );
  const total = $derived(rows.reduce((sum, row) => sum + row.list.length, 0));

  async function pick(relayId: string, name: string) {
    if (switching) return;
    switching = `${relayId}:${name}`;
    try {
      await relayStore.selectSession(relayId, name);
    } finally {
      switching = '';
    }
  }
</script>

{#if total > 0}
  <div class="session-picker" role="group" aria-label="Herdr sessions">
    {#each rows as row (row.relay.id)}
      <div class="filter-row session-row">
        <!-- Say what these chips are: herdr sessions, not machines or agents. -->
        <span class="session-relay">Sessions</span>
        {#if relays.length > 1}
          <span class="session-relay">{row.relay.label || row.relay.id}</span>
        {/if}
        {#each row.list as session (session.name)}
          {@const active = session.name === (activeSessions.get(row.relay.id) || '')
            || session.active}
          <button
            type="button"
            class="filter-chip session-chip"
            class:active
            aria-pressed={active}
            aria-label={`Mirror session ${session.name}${active ? ' (current)' : ''}`}
            disabled={!session.running || switching !== ''}
            onclick={() => pick(row.relay.id, session.name)}
          >
            <span class={`session-dot ${session.running ? 'running' : 'stopped'}`} aria-hidden="true"></span>
            <span class="filter-chip-label">{session.name}</span>
            {#if active}<span class="session-current">mirroring</span>{/if}
            {#if !session.running}<span class="session-current">stopped</span>{/if}
          </button>
        {/each}
      </div>
    {/each}
  </div>
{/if}
