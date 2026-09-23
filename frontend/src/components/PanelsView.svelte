<script lang="ts">
  import type { Agent, HerdrSession, Machine, RelayConfig, RelayConnectionView } from '$lib/types';

  /**
   * The panels: production and the deploy marker per relay, the machines behind
   * it, and — plainly marked as such — the things a relay on this computer
   * cannot see. A panel that invents a number is worse than a panel that says it
   * has none, so every unobservable row names what would have to be wired up.
   */
  let {
    relays,
    connections,
    machines,
    sessions,
    agents,
    onrefresh,
  }: {
    relays: RelayConfig[];
    connections: Map<string, RelayConnectionView>;
    machines: Map<string, Machine[]>;
    sessions: Map<string, HerdrSession[]>;
    agents: Agent[];
    onrefresh: () => void;
  } = $props();

  function deployState(state: string | undefined): string {
    if (!state || state === 'idle') return 'no deploy in flight';
    return state;
  }

  function markerAgrees(connection: RelayConnectionView | undefined): boolean {
    const target = connection?.appDeploy?.target_version || '';
    if (!target) return true;
    return target === connection?.version;
  }

  function machineHealth(reachable: boolean): string {
    return reachable ? 'reachable' : 'unreachable';
  }
</script>

<main class="page panels-view" aria-label="Panels">
  <header class="orchestrator-header">
    <div>
      <h1>Panels</h1>
      <p class="orchestrator-sub">What this computer can see, and what it cannot.</p>
    </div>
    <button type="button" class="board-entry" onclick={onrefresh}>Refresh</button>
  </header>

  <section class="board-section" aria-labelledby="panel-prod">
    <h2 id="panel-prod">Production</h2>
    {#each relays as relay (relay.id)}
      {@const connection = connections.get(relay.id)}
      <article class="board-card">
        <header class="board-card-head">
          <h3>{relay.label || relay.id}</h3>
          <span class={`board-state ${connection?.status === 'connected' ? 'state-done' : 'state-blocked'}`}>
            {connection?.status || 'not connected'}
          </span>
        </header>
        {#if connection}
          <ul class="board-list">
            <li><span class="board-owner">running</span><span class="board-meta-inline">{connection.version} · {connection.revision}</span></li>
            <li><span class="board-owner">marker</span><span class="board-meta-inline">{connection.appDeploy?.target_version || '—'}{connection.appDeploy?.target_revision ? ` · ${connection.appDeploy.target_revision}` : ''}</span></li>
            <li>
              <span class="board-owner">marker vs real</span>
              <span class="board-meta-inline">
                {markerAgrees(connection) ? 'matching' : 'marker and running build differ'}
              </span>
            </li>
            <li><span class="board-owner">deploy</span><span class="board-meta-inline">{deployState(connection.appDeploy?.state)}</span></li>
            <li>
              <span class="board-owner">herdr</span>
              <span class="board-meta-inline">
                client {connection.herdrStatus?.installed_client_version || 'unknown'} ·
                server {connection.herdrStatus?.server_version || 'unknown'} · protocol {connection.protocol}
              </span>
            </li>
            <li><span class="board-owner">inventory</span><span class="board-meta-inline">{connection.inventory.state}{connection.inventory.stale ? ' · stale' : ''}</span></li>
            <li><span class="board-owner">transport</span><span class="board-meta-inline">{connection.path || 'unknown'}</span></li>
            <li><span class="board-owner">sessions</span><span class="board-meta-inline">{(sessions.get(relay.id) || []).length || '—'}</span></li>
            <li><span class="board-owner">agents</span><span class="board-meta-inline">{agents.filter((agent) => agent.relay_id === relay.id).length}</span></li>
          </ul>
        {:else}
          <p class="board-hint">This computer has not answered yet.</p>
        {/if}
      </article>
    {/each}
  </section>

  <section class="board-section" aria-labelledby="panel-machines">
    <h2 id="panel-machines">Machines</h2>
    {#each relays as relay (relay.id)}
      {#each machines.get(relay.id) || [] as machine (machine.machine_id)}
        <article class={`board-card ${machine.reachable === false ? 'state-blocked' : ''}`}>
          <header class="board-card-head">
            <h3>{machine.label}</h3>
            <span class={`board-state ${machine.reachable === false ? 'state-blocked' : 'state-done'}`}>{machineHealth(machine.reachable !== false)}</span>
          </header>
          <p class="board-meta">
            {#if machine.local}<span>local</span>{/if}
            <span class="board-meta-inline">{machine.agent_count} agents · {machine.workspace_count} workspaces</span>
            {#if machine.error}<span class="board-state state-blocked">{machine.error}</span>{/if}
          </p>
        </article>
      {/each}
    {/each}
  </section>

  <section class="board-section" aria-labelledby="panel-blind">
    <h2 id="panel-blind">Not visible from a relay</h2>
    <p class="board-hint">
      These belong to the cluster, not to this computer. They cannot be shown from here,
      and the rows that depend on them are on the board under their own ids.
    </p>
    <article class="board-card">
      <header class="board-card-head"><h3>Watchdog</h3></header>
      <p class="board-hint">Heartbeat, passes and scope health live in the manager's own store; no relay reads it.</p>
    </article>
    <article class="board-card">
      <header class="board-card-head"><h3>Data · writer rows vs anchor</h3></header>
      <p class="board-hint">The RPO gap is measured by the cluster's own writers against the anchor object; the relay holds no cluster credential.</p>
    </article>
    <article class="board-card">
      <header class="board-card-head"><h3>Backups</h3></header>
      <p class="board-hint">Backup runs are reported by the backup host, not by a relay.</p>
    </article>
  </section>
</main>
