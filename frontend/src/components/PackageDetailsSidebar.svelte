<script lang="ts">
  import { Browser } from '@wailsio/runtime';
  import type { PackageDetailsView, PackageView } from '../../bindings/sysentra/internal/app/models';
  import PackageStatusBadges from './PackageStatusBadges.svelte';

  export let selected: PackageView;
  export let details: PackageDetailsView | null = null;
  export let loading = false;
  export let error = '';
  export let onClose: () => void;
  export let onAction: (title: string, description: string, destructive?: boolean, operation?: string, targetVersion?: string) => void;

  $: homepage = safeExternalURL(details?.homepage ?? '');

  function action(title: string, description: string, destructive = false, operation = '', targetVersion = '') {
    onAction(title, description, destructive, operation, targetVersion);
  }

  function safeExternalURL(value: string) {
    try {
      const url = new URL(value);
      return url.protocol === 'https:' || url.protocol === 'http:' ? url.toString() : '';
    } catch {
      return '';
    }
  }
</script>

<aside class="package-detail-sidebar flex min-h-0 w-[min(22rem,42%)] shrink-0 flex-col overflow-hidden rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="package-detail-title">
  <header class="flex items-start gap-3 border-b border-surface-300-700 p-3">
    <div class="grid size-10 shrink-0 place-items-center rounded-base border border-surface-300-700 bg-surface-50-900 text-surface-500" title="Package icon">
      {#if details?.iconDataUrl}
        <img class="size-8 object-contain" src={details.iconDataUrl} alt="" aria-hidden="true" />
      {:else}
        <svg viewBox="0 0 24 24" class="size-6" aria-hidden="true"><path fill="currentColor" d="m12 2 9 4.5v11L12 22l-9-4.5v-11L12 2Zm0 2.2L6.1 7.15 12 10.1l5.9-2.95L12 4.2Zm-7 4.57v7.5l6 3v-7.5l-6-3Zm8 10.5 6-3v-7.5l-6 3v7.5Z" /></svg>
      {/if}
    </div>
    <div class="min-w-0 flex-1"><h2 id="package-detail-title" class="truncate font-semibold">{selected.name}</h2><p class="truncate text-xs text-surface-500">{selected.backendId} · {selected.id}</p><div class="mt-1 flex flex-wrap gap-1"><PackageStatusBadges pkg={selected} /></div></div>
    <button type="button" class="btn preset-tonal" aria-label="Close package details" on:click={onClose}>Close</button>
  </header>

  <div class="min-h-0 flex-1 overflow-auto p-3">
    {#if loading}
      <p class="text-sm" aria-live="polite">Loading package details…</p>
    {:else if error}
      <div class="space-y-2"><p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-sm" role="alert">{error}</p><p class="text-sm text-surface-500">{selected.description || 'No additional description is available.'}</p></div>
    {:else if details}
      <div class="space-y-4 text-sm">
        <div class="flex flex-wrap gap-2 border-b border-surface-300-700 pb-3">
          {#if !selected.installed}
            <button type="button" class="btn preset-filled-primary-500" on:click={() => action('Install package', `Install ${selected.name}.`, false, 'install')}>Install</button>
          {:else}
            <button type="button" class="btn bg-error-600 text-white hover:bg-error-700" on:click={() => action('Remove package', `Remove ${selected.name}.`, true, 'remove')}>Remove</button>
            {#if selected.backendId === 'xbps'}
              <button type="button" class="btn border-error-500/50 text-error-700-300 hover:bg-error-500/10" title="Remove with unused dependencies" on:click={() => action('Remove package and unused dependencies', `Remove ${selected.name} and dependencies no longer needed by another package.`, true, 'purge')}>Purge</button>
              <button type="button" class="btn preset-tonal" on:click={() => action(selected.held ? 'Release package hold' : 'Hold package', `${selected.held ? 'Allow' : 'Prevent'} automatic updates for ${selected.name}.`, false, selected.held ? 'unhold' : 'hold')}>{selected.held ? 'Release hold' : 'Hold'}</button>
              <button type="button" class="btn preset-tonal" on:click={() => action(selected.repoLocked ? 'Release repository lock' : 'Lock package repository', `${selected.repoLocked ? 'Allow' : 'Restrict'} repository changes for ${selected.name}.`, false, selected.repoLocked ? 'repo-unlock' : 'repo-lock')}>{selected.repoLocked ? 'Unlock repo' : 'Repo-lock'}</button>
              <button type="button" class="btn preset-tonal" on:click={() => action('Force reinstall package', `Force reinstall ${selected.name} even when its installed version is current.`, false, 'reinstall')}>Force reinstall</button>
              <button type="button" class="btn preset-tonal" on:click={() => action('Rerun post-install configuration', `Rerun only the post-installation configuration for ${selected.name}.`, false, 'reconfigure')}>Rerun configuration</button>
            {/if}
          {/if}
        </div>
        {#if selected.backendId === 'xbps' && selected.installed && details.cachedVersions.length > 0}
          <details class="rounded-base border border-surface-300-700">
            <summary class="cursor-pointer px-3 py-2 font-medium">Cached package versions <span class="text-xs text-surface-500">({details.cachedVersions.length})</span></summary>
            <div class="space-y-2 border-t border-surface-300-700 p-3"><p class="text-xs text-surface-500">Local XBPS cache. Sysentra verifies the archive before installation.</p><div class="flex flex-wrap gap-2">{#each details.cachedVersions as version}<button type="button" class="btn preset-tonal font-mono" on:click={() => action('Install cached package version', `Change ${selected.name} from ${details.version} to ${version}.`, false, 'downgrade', version)}>{version}</button>{/each}</div></div>
          </details>
        {/if}
        <p class="line-clamp-6 leading-relaxed" title={details.description || selected.description}>{details.description || selected.description || 'No description available.'}</p>
        <dl class="grid grid-cols-[6rem_minmax(0,1fr)] gap-x-3 gap-y-2 text-xs">
          <dt class="text-surface-500">Version</dt><dd class="truncate font-mono">{details.version || '—'}</dd>
          <dt class="text-surface-500">Architecture</dt><dd>{details.architecture || '—'}</dd>
          {#if details.scope}<dt class="text-surface-500">Installation</dt><dd class="capitalize">{details.scope}</dd>{/if}
          {#if details.downloadSize}<dt class="text-surface-500">Download size</dt><dd>{details.downloadSize}</dd>{/if}
          {#if details.installedSize}<dt class="text-surface-500">Installed size</dt><dd>{details.installedSize}</dd>{/if}
          {#if details.installDate}<dt class="text-surface-500">Installed on</dt><dd>{details.installDate}</dd>{/if}
          <dt class="text-surface-500">License</dt><dd>{details.license || '—'}</dd>
          <dt class="text-surface-500">Repository</dt><dd class="break-all">{details.repository || selected.repository || '—'}</dd>
          {#if details.sourcePackage}<dt class="text-surface-500">Source package</dt><dd>{details.sourcePackage}</dd>{/if}
          {#if details.maintainer}<dt class="text-surface-500">Maintainer</dt><dd class="break-words">{details.maintainer}</dd>{/if}
          <dt class="text-surface-500">Homepage</dt><dd class="break-all">{#if homepage}<a class="text-primary-700-300 underline hover:no-underline" href={homepage} on:click|preventDefault={() => void Browser.OpenURL(homepage)}>{details.homepage}</a>{:else}{details.homepage || '—'}{/if}</dd>
        </dl>

        {#if details.provides.length > 0 || details.requires.length > 0 || details.exports.length > 0}
          <details class="rounded-base border border-surface-300-700">
            <summary class="cursor-pointer px-3 py-2 font-medium">Provides &amp; Requires</summary>
            <div class="max-h-64 space-y-3 overflow-auto border-t border-surface-300-700 p-3 text-xs">
              {#each [['Provides', details.provides], ['Requires', details.requires], ['Exports', details.exports]] as section}
                {#if section[1].length > 0}<div><h3 class="mb-1 font-semibold">{section[0]}</h3><ul class="font-mono">{#each section[1] as item}<li class="break-all py-0.5">{item}</li>{/each}</ul></div>{/if}
              {/each}
            </div>
          </details>
        {/if}

        {#each [['Dependencies', details.dependencies], ['Required by', details.reverseDependencies], ['Installed files', details.files]] as section}
          {#if section[1].length > 0}
            <details class="rounded-base border border-surface-300-700">
              <summary class="cursor-pointer px-3 py-2 font-medium">{section[0]} <span class="text-xs text-surface-500">({section[1].length})</span></summary>
              <ul class="max-h-52 overflow-auto border-t border-surface-300-700 font-mono text-xs">{#each section[1] as item}<li class="break-all border-b border-surface-200-800 px-3 py-2 last:border-0">{item}</li>{/each}</ul>
            </details>
          {/if}
        {/each}
      </div>
    {/if}
  </div>
</aside>
