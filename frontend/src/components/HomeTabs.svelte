<script lang="ts">
  import { navigate } from '$lib/router';

  /**
   * The four faces of the home: the agents themselves, the orchestration board,
   * the panels, and the herdr sessions the relay can mirror. Machine-first is
   * kept inside each of them; this is only what you are looking at.
   */
  let { active }: { active: 'agents' | 'board' | 'panels' | 'sessions' } = $props();

  const tabs = [
    { key: 'agents', label: 'Agents', view: { view: 'agents' } as const },
    { key: 'board', label: 'Board', view: { view: 'orchestrator' } as const },
    { key: 'panels', label: 'Panels', view: { view: 'panels' } as const },
    { key: 'sessions', label: 'Sessions', view: { view: 'sessions' } as const },
  ];
</script>

<nav class="home-tabs" aria-label="Home sections">
  {#each tabs as tab (tab.key)}
    <button
      type="button"
      class="home-tab"
      class:active={tab.key === active}
      aria-current={tab.key === active ? 'page' : undefined}
      onclick={() => navigate(tab.view)}
    >{tab.label}</button>
  {/each}
</nav>
