<script lang="ts">
  import AgentLogo from '$components/AgentLogo.svelte';
  import { agentStatusTone, displayName, tabName } from '$lib/agents';
  import type { Agent, Machine } from '$lib/types';
  import { machineWorkspaceBuckets, workspaceGroups } from '$lib/workspaces';

  let {
    agents,
    machines = [],
    active,
    onopen,
    onjump,
  }: {
    agents: Agent[];
    machines?: Machine[];
    active: Agent;
    onopen: (agent: Agent) => void;
    onjump: () => void;
  } = $props();

  // Mirrors the desktop sidebar: one section per machine, the local one first,
  // each keeping its own workspaces.
  const machineBuckets = $derived(machineWorkspaceBuckets(workspaceGroups(agents), machines));
</script>

<aside class="agent-rail" aria-label="Agent navigation">
  <header>
    <strong>Agents</strong>
    <button type="button" onclick={onjump} aria-label="Search all agents" title="Search all agents">⌕</button>
  </header>
  <div class="agent-rail-groups">
    {#each machineBuckets as machine (machine.key)}
      <div class="agent-rail-machine" class:remote={!machine.local} class:offline={!machine.reachable}>
        <h3 class="agent-rail-machine-header" title={machine.error || machine.host}>
          <span>{machine.label}</span>
          {#if !machine.local}<small>{machine.reachable ? 'remote' : 'offline'}</small>{/if}
        </h3>
        {#each machine.groups as group (group.key)}
          <section aria-label={`${group.label} workspace on ${machine.label}`}>
            <h2 title={group.cwd}>{group.label}<small>@{group.host}</small></h2>
            {#each group.agents as agent (agent.pane_id)}
              <button class:active={agent.pane_id === active.pane_id} type="button" aria-current={agent.pane_id === active.pane_id ? 'page' : undefined} onclick={() => onopen(agent)}>
                <span class="agent-identity">
                  <AgentLogo agent={agent.agent} />
                  <span class={`status-dot status-${agentStatusTone(agent)}`} aria-hidden="true"></span>
                </span>
                <span>
                  <strong>{displayName(agent)}</strong>
                  <small>{[tabName(agent), agent.session].filter(Boolean).join(' · ')}</small>
                </span>
              </button>
            {/each}
          </section>
        {/each}
      </div>
    {/each}
  </div>
</aside>
