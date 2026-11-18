<script lang="ts">
  import { onMount } from 'svelte';
  import type { PackageView } from '../../bindings/sysentra/internal/app/models';

  import { createPackageSearchIndex, filterPackages, filterPackageSource, filterPackageState, visibleRange } from '../lib/package-list.js';
  import PackageStatusBadges from './PackageStatusBadges.svelte';

  const rowHeight = 44;
  export let packages: PackageView[];
  export let title: string;
  export let description: string;
  export let action: 'install' | 'remove' | 'update';
  export let showStateFilters = false;
  export let showSourceFilter = false;
  export let query = '';
  export let resultCount = 0;
  export let selectedPackageKey = '';
  export let selectedItems: PackageView[] = [];
  export let onPackageSelect: (pkg: PackageView) => void = () => {};
  export let onExecuteSelection: (action: 'install' | 'remove' | 'update' | 'force-remove') => void = () => {};

  let scrollTop = 0;
  let viewportHeight = 440;
  let viewport: HTMLDivElement;
  let selected = new Set<string>();
  let stateFilter: 'all' | 'held' | 'marked' | 'orphaned' | 'repoLocked' = 'all';
  let sourceFilter = '';

  $: sources = [...new Set(packages.map((pkg) => pkg.repository).filter(Boolean))].sort();
  $: searchIndex = createPackageSearchIndex(packages);
  $: searchedPackages = filterPackages(packages, query, searchIndex);
  $: sourcedPackages = filterPackageSource(searchedPackages, sourceFilter);
  $: filteredPackages = filterPackageState(sourcedPackages, stateFilter, selected);
  $: resultCount = filteredPackages.length;
  $: range = visibleRange(scrollTop, viewportHeight, filteredPackages.length, rowHeight, 8);
  $: visiblePackages = filteredPackages.slice(range.start, range.end);
  $: selectionAction = action === 'install' && selectedItems[0]?.installed ? 'remove' : action;
  $: packages, action, (selected = new Set<string>(), selectedItems = [], sourceFilter = '');
  $: if (viewport) {
    `${query}\n${sourceFilter}\n${stateFilter}`;
    viewport.scrollTop = 0;
    scrollTop = 0;
  }

  function packageKey(pkg: PackageView) {
    return `${pkg.backendId}:${pkg.id}`;
  }

  function canSelect(pkg: PackageView) {
    const first = selectedItems[0];
    if (first && (first.backendId !== pkg.backendId || first.installed !== pkg.installed || first.scope !== pkg.scope)) return false;
    if (action === 'install') return true;
    if (action === 'remove') return pkg.installed;
    return pkg.newVersion !== '';
  }

  function togglePackage(pkg: PackageView) {
    const next = new Set(selected);
    const key = packageKey(pkg);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    selected = next;
    selectedItems = packages.filter((item) => next.has(packageKey(item)));
  }

  function clearSelection() {
    selected = new Set<string>();
    selectedItems = [];
  }

  function selectPackageFromKeyboard(event: KeyboardEvent, pkg: PackageView) {
    if (event.target !== event.currentTarget || (event.key !== 'Enter' && event.key !== ' ')) return;
    event.preventDefault();
    onPackageSelect(pkg);
  }

  function updateViewport() {
    scrollTop = viewport.scrollTop;
    viewportHeight = viewport.clientHeight;
  }

  onMount(() => {
    viewportHeight = viewport.clientHeight;
    const observer = new ResizeObserver(updateViewport);
    observer.observe(viewport);
    return () => observer.disconnect();
  });
</script>

<section aria-labelledby="packages-heading" class="package-table flex h-full min-h-0 min-w-0 flex-1 flex-col overflow-hidden rounded-base border border-surface-300-700 bg-surface-100-800">
  <header class="flex flex-wrap items-center justify-between gap-3 border-b border-surface-300-700 px-3 py-2">
    <div>
      <h2 id="packages-heading" class="text-sm font-semibold">{title}</h2>
      <p class="text-xs text-surface-500">{description}</p>
    </div>
    <div class="flex flex-wrap gap-2"><slot name="actions" /></div>
  </header>

  {#if showStateFilters || showSourceFilter}
    <div class="flex flex-wrap items-center gap-1.5 border-b border-surface-300-700 bg-surface-50-900 px-2 py-2" aria-label="Package filters">
      {#each [['all', 'All'], ['held', 'On Hold'], ['marked', 'Marked'], ['orphaned', 'Orphaned'], ['repoLocked', 'Repo-Locked']] as filter}
        {#if showStateFilters}<button type="button" class="rounded-base border px-2.5 py-1 text-xs {stateFilter === filter[0] ? 'border-primary-500 bg-primary-500/10 font-semibold text-primary-700-300' : 'border-surface-300-700 hover:bg-surface-200-800'}" aria-pressed={stateFilter === filter[0]} on:click={() => stateFilter = filter[0] as typeof stateFilter}>{filter[1]}</button>{/if}
      {/each}
      {#if showSourceFilter}
        <label for="package-source" class="ml-auto text-xs text-surface-500">Source</label>
        <select id="package-source" class="select h-7 max-w-64 py-0.5 text-xs" bind:value={sourceFilter}>
          <option value="">All sources</option>
          {#each sources as source}<option value={source}>{source}</option>{/each}
        </select>
      {/if}
    </div>
  {/if}

  {#if selected.size > 0}
    <div class="flex flex-wrap items-center gap-2 border-b border-surface-300-700 bg-primary-500/10 px-3 py-2 text-sm" role="status">
      <strong>{selected.size} selected for {selectionAction === 'remove' ? 'removal' : selectionAction === 'update' ? 'update' : 'installation'}</strong>
      <button type="button" class="btn ml-auto {selectionAction === 'remove' ? 'bg-error-600 text-white hover:bg-error-700' : 'preset-filled-primary-500'}" on:click={() => onExecuteSelection(selectionAction)}>{selectionAction === 'remove' ? 'Remove selected' : selectionAction === 'update' ? 'Update selected' : 'Install selected'}</button>
      {#if selectionAction === 'remove' && selectedItems[0]?.backendId === 'xbps'}<button type="button" class="btn border-error-500/50 text-error-700-300" on:click={() => onExecuteSelection('force-remove')}>Force remove</button>{/if}
      <button type="button" class="btn preset-tonal" on:click={clearSelection}>Clear</button>
    </div>
  {/if}

  <div class="flex min-h-0 flex-1 flex-col overflow-hidden" role="table" aria-label="Packages" aria-rowcount={filteredPackages.length + 1} aria-colcount="6">
    <div class="package-table-header package-grid grid border-b border-surface-300-700 text-xs font-semibold" role="row">
      <span class="px-3 py-2" role="columnheader"><span class="sr-only">Select</span></span>
      <span class="px-3 py-2" role="columnheader">Package</span>
      <span class="px-3 py-2" role="columnheader">Description</span>
      <span class="px-3 py-2" role="columnheader">Version</span>
      <span class="package-source px-3 py-2" role="columnheader">Source</span>
      <span class="package-status px-3 py-2" role="columnheader">Status</span>
    </div>

    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <!-- The inline height prevents WebKit from measuring the spacer before CSS loads. -->
    <div
      bind:this={viewport}
      class="min-h-0 flex-1 overflow-auto"
      style="height: 100%; min-height: 0"
      tabindex="0"
      aria-label="Scrollable package rows"
      on:scroll={updateViewport}
    >
      {#if filteredPackages.length === 0}
        <p class="p-6 text-center text-sm text-surface-500" role="status">No packages match this search.</p>
      {:else}
        <div class="relative min-w-0" style:height={`${range.spacerHeight}px`}>
          <div class="absolute inset-x-0 top-0" style:transform={`translateY(${range.offset}px)`}>
            {#each visiblePackages as pkg, index (packageKey(pkg))}
              <div
                class="package-grid grid h-11 cursor-pointer border-b border-surface-200-800 text-sm hover:bg-primary-500/5 {selectedPackageKey === packageKey(pkg) ? 'bg-primary-500/10' : ''}"
                role="row"
                tabindex="0"
                aria-selected={selectedPackageKey === packageKey(pkg)}
                aria-rowindex={range.start + index + 2}
                on:click={() => onPackageSelect(pkg)}
                on:keydown={(event) => selectPackageFromKeyboard(event, pkg)}
              >
                <span class="grid place-items-center" role="cell">
                  <input
                    type="checkbox"
                    checked={selected.has(packageKey(pkg))}
                    disabled={!canSelect(pkg)}
                    aria-label={`${action === 'install' && pkg.installed ? 'remove' : action} ${pkg.name} from ${pkg.backendId}`}
                    on:change={() => togglePackage(pkg)}
                    on:click|stopPropagation
                  />
                </span>
                <span class="truncate px-3 py-2 font-medium" role="cell">{pkg.name}</span>
                <span class="truncate px-3 py-3 text-surface-600-400" role="cell" title={pkg.description}>{pkg.description || '—'}</span>
                <span class="truncate px-3 py-3 font-mono text-xs" role="cell">{pkg.newVersion ? `${pkg.version || '—'} → ${pkg.newVersion}` : pkg.version}</span>
                <span class="package-source truncate px-3 py-3 text-surface-600-400" role="cell">{pkg.repository || '—'}</span>
                <span class="package-status flex min-w-0 items-center gap-1 overflow-hidden px-3 py-2" role="cell">
                  <PackageStatusBadges {pkg} />
                </span>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    </div>
  </div>
</section>
