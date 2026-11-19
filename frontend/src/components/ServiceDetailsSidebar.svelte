<script lang="ts">
  import type { ServiceDetailsView, ServiceView } from '../../bindings/sysentra/internal/app/models';

  export let selected: ServiceView;
  export let details: ServiceDetailsView | null = null;
  export let loading = false;
  export let error = '';
  export let onClose: () => void;
  export let onReload: () => void;
  export let onAction: (operation: string) => void;
</script>

<aside class="package-detail-sidebar flex min-h-0 w-[min(24rem,44%)] shrink-0 flex-col overflow-hidden rounded-base border border-surface-300-700 bg-surface-100-800" aria-labelledby="service-detail-title">
  <header class="flex items-start gap-3 border-b border-surface-300-700 p-3">
    <div class="min-w-0 flex-1"><h2 id="service-detail-title" class="truncate font-semibold">{selected.name}</h2><p class="text-xs text-surface-500">Runit service</p></div>
    <button type="button" class="btn preset-tonal" aria-label="Close service details" on:click={onClose}>Close</button>
  </header>
  <div class="min-h-0 flex-1 overflow-auto p-3">
    <div class="mb-4 flex flex-wrap gap-2">
      {#if selected.status === 'Running'}
        <button type="button" class="btn preset-tonal" on:click={() => onAction('restart')}>Restart</button>
        <button type="button" class="btn bg-error-600 text-white hover:bg-error-700" on:click={() => onAction('stop')}>Stop</button>
        <button type="button" class="btn preset-tonal" on:click={() => onAction('reload')}>Reload</button>
        <button type="button" class="btn preset-tonal" on:click={() => onAction('pause')}>Pause</button>
      {:else}
        <button type="button" class="btn preset-filled-primary-500" on:click={() => onAction('start')}>Start</button>
        <button type="button" class="btn preset-tonal" on:click={() => onAction('once')}>Run once</button>
      {/if}
      <button type="button" class="btn preset-tonal" on:click={() => onAction(selected.enabled ? 'disable' : 'enable')}>{selected.enabled ? 'Disable' : 'Enable'}</button>
      <button type="button" class="btn preset-tonal" on:click={onReload} disabled={loading}>Refresh</button>
    </div>
    <dl class="mb-4 grid grid-cols-[5rem_minmax(0,1fr)] gap-x-3 gap-y-2 text-xs">
      <dt class="text-surface-500">State</dt><dd>{selected.status}</dd>
      <dt class="text-surface-500">Enabled</dt><dd>{selected.enabled ? 'Yes' : 'No'}</dd>
      <dt class="text-surface-500">PID</dt><dd class="font-mono">{selected.pid || '—'}</dd>
      <dt class="text-surface-500">Command</dt><dd class="break-all font-mono">{selected.command || '—'}</dd>
      <dt class="text-surface-500">Uptime</dt><dd class="font-mono">{selected.time || '—'}</dd>
      {#if details?.definition}<dt class="text-surface-500">Definition</dt><dd class="break-all font-mono">{details.definition}</dd>{/if}
    </dl>
    {#if loading}<p class="text-sm" aria-live="polite">Loading service details…</p>
    {:else if error}<p class="rounded-base border border-error-500/40 bg-error-500/10 p-3 text-sm" role="alert">{error}</p>
    {:else if details}
      {#if details.runScript}<details class="mb-3 rounded-base border border-surface-300-700"><summary class="cursor-pointer px-3 py-2 text-sm font-medium">Run script</summary><pre class="max-h-40 overflow-auto border-t border-surface-300-700 p-3 whitespace-pre-wrap break-words font-mono text-xs">{details.runScript}</pre></details>{/if}
      <section class="overflow-hidden rounded-base border border-surface-300-700 bg-surface-950 text-surface-50" aria-labelledby="service-log-title"><div class="border-b border-surface-700 px-3 py-2"><h3 id="service-log-title" class="text-xs font-semibold">Recent logs</h3>{#if details.logPath}<p class="truncate font-mono text-[0.65rem] text-surface-400" title={details.logPath}>{details.logPath}</p>{/if}</div><pre class="max-h-72 overflow-auto whitespace-pre-wrap break-words p-3 font-mono text-xs">{details.logs.length ? details.logs.join('\n') : 'No service log found.'}</pre></section>
    {/if}
  </div>
</aside>
