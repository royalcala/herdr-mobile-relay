<script lang="ts">
  import { STATUS_FILTERS, machineTag, type MachineSection, type StatusFilterKey } from '$lib/machines';

  let {
    sections,
    machineFilter,
    statusFilter,
    showMachines = true,
    onmachine,
    onstatus,
  }: {
    sections: MachineSection[];
    machineFilter: string;
    statusFilter: StatusFilterKey;
    showMachines?: boolean;
    onmachine: (key: string) => void;
    onstatus: (key: StatusFilterKey) => void;
  } = $props();

  const total = $derived(sections.reduce((sum, section) => sum + section.counts.total, 0));
</script>

<div class="machine-filters">
  {#if showMachines}
    <div class="filter-row" role="group" aria-label="Filter by machine">
    <button
      type="button"
      class="filter-chip machine-chip"
      class:active={machineFilter === 'all'}
      aria-pressed={machineFilter === 'all'}
      onclick={() => onmachine('all')}
    >
      <span class="filter-chip-label">All machines</span>
      <span class="filter-count" aria-hidden="true">{total}</span>
    </button>
    {#each sections as section (section.key)}
      <button
        type="button"
        class="filter-chip machine-chip"
        class:active={machineFilter === section.key}
        class:remote={!section.ref.local}
        class:offline={!section.ref.reachable}
        aria-pressed={machineFilter === section.key}
        aria-label={`Show ${section.ref.label}${section.ref.local ? '' : `, ${machineTag(section.ref)}, read only`}`}
        title={section.ref.error || section.ref.host}
        onclick={() => onmachine(section.key)}
      >
        {#if section.ref.readOnly}
          <span class="readonly-lock" aria-hidden="true">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round" focusable="false">
              <rect x="4" y="10" width="16" height="10" rx="2"></rect>
              <path d="M8 10V7a4 4 0 0 1 8 0v3"></path>
            </svg>
          </span>
        {/if}
        <span class="filter-chip-label">{section.ref.label}</span>
        {#if !section.ref.local}<small class="filter-chip-tag">{machineTag(section.ref)}</small>{/if}
        <span class="filter-count" aria-hidden="true">{section.counts.total}</span>
        {#if section.counts.blocked}<span class="filter-blocked" aria-hidden="true" title={`${section.counts.blocked} waiting on you`}>{section.counts.blocked}</span>{/if}
      </button>
    {/each}
    </div>
  {/if}
  <div class="filter-row status-row" role="group" aria-label="Filter by status">
    {#each STATUS_FILTERS as option (option.key)}
      <button
        type="button"
        class="filter-chip status-chip"
        class:active={statusFilter === option.key}
        aria-pressed={statusFilter === option.key}
        onclick={() => onstatus(option.key)}
      >
        <span class={`status-dot status-${option.tone}`} class:hollow={option.key === 'all' || option.key === 'idle'} aria-hidden="true"></span>
        {option.label}
      </button>
    {/each}
  </div>
</div>
