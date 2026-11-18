<script lang="ts">
  import type { PackageView } from '../../bindings/sysentra/internal/app/models';

  export let pkg: PackageView;
</script>

<span class="shrink-0 rounded-full border px-2 py-0.5 text-xs {pkg.status === 'Installed' ? 'border-success-500/40 bg-success-500/10 text-success-700-300' : pkg.status === 'Update available' ? 'border-warning-500/40 bg-warning-500/10 text-warning-700-300' : 'border-surface-400/50 bg-surface-200-800 text-surface-700-300'}">{pkg.status}</span>
{#if pkg.held}<span class="shrink-0 rounded-full border border-warning-500/40 bg-warning-500/10 px-2 py-0.5 text-xs text-warning-700-300">Hold</span>{/if}
{#if pkg.orphaned}<span class="shrink-0 rounded-full border border-error-500/40 bg-error-500/10 px-2 py-0.5 text-xs text-error-700-300">Orphan</span>{/if}
{#if pkg.repoLocked}<span class="shrink-0 rounded-full border border-primary-500/40 bg-primary-500/10 px-2 py-0.5 text-xs text-primary-700-300">Repo-Locked</span>{/if}
