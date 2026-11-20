import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

test('keeps the virtual viewport bounded and observes window-driven resizing', () => {
  const component = readFileSync(new URL('../components/VirtualPackageTable.svelte', import.meta.url), 'utf8');
  const styles = readFileSync(new URL('../app.css', import.meta.url), 'utf8');

  assert.match(component, /style="height: 100%; min-height: 0"/);
  assert.match(component, /new ResizeObserver\(updateViewport\)/);
  assert.match(component, /observer\.disconnect\(\)/);
  assert.match(component, /viewport\.scrollTop = 0/);
  assert.match(component, /const overscanRows = 24/);
  assert.match(component, /package-table flex/);
  assert.match(styles, /container-type: inline-size/);
  assert.match(styles, /@container package-table/);
});

test('prevents document-controlled navigation and embedded content', () => {
  const html = readFileSync(new URL('../../index.html', import.meta.url), 'utf8');

  assert.match(html, /object-src 'none'/);
  assert.match(html, /frame-src 'none'/);
  assert.match(html, /base-uri 'none'/);
  assert.match(html, /form-action 'none'/);
});

test('keeps package search input responsive and exposes gated detail actions', () => {
  const app = readFileSync(new URL('../App.svelte', import.meta.url), 'utf8');
  const details = readFileSync(new URL('../components/PackageDetailsSidebar.svelte', import.meta.url), 'utf8');
  const component = readFileSync(new URL('../components/VirtualPackageTable.svelte', import.meta.url), 'utf8');

  assert.match(app, /scheduleSearch\(event\.currentTarget\.value\)/);
  assert.match(app, /setTimeout\(\(\) => globalQuery = value, 120\)/);
  assert.match(app, /function selectView\(view: string\) \{\s*clearTimeout\(searchTimer\);\s*searchInput = '';\s*globalQuery = '';/);
  assert.match(app, /finally \{\s*loadPackages\(\);\s*refreshUpdateCount\(\);\s*\}/);
  assert.doesNotMatch(app, /loadPackages\(\)\.then\(\(\) => refreshUpdateCount\(\)\)/);
  assert.match(details, /PackageStatusBadges/);
  assert.match(details, /details\?\.iconDataUrl/);
  assert.match(details, /Browser\.OpenURL/);
  assert.match(details, /Download size/);
  assert.match(details, /Installed size/);
  assert.match(details, /Provides &amp; Requires/);
  assert.match(details, /unused dependencies/);
  assert.match(details, /Force reinstall/);
  assert.match(details, /Rerun post-install configuration/);
  assert.match(details, /false, 'reconfigure'/);
  assert.match(details, /false, 'downgrade', version/);
  assert.match(details, /Install/);
  assert.match(app, /selectedPackageKey/);
  assert.match(app, /PreviewMaintenance\(operation, packageIDs\)/);
  assert.match(app, /PreviewDowngrade\(pkg\.id, targetVersion\)/);
  assert.match(details, />Downgrade<\/button>/);
  assert.match(app, /'remove-orphans'/);
  assert.match(app, /'force-remove'/);
  assert.match(component, /onExecuteSelection\('force-remove'\)\}>Force remove/);
  assert.doesNotMatch(app, /\['Force remove selected',/);
  assert.match(app, /\['Remove orphans',[\s\S]*\['Clean package cache',[\s\S]*\['Clean unused Flatpaks',/);
  assert.match(app, /'clean-cache'/);
  assert.match(app, /'verify-database'/);
  assert.match(app, /'reconfigure-all'/);
  assert.match(app, /Sync database<\/button>\{#if activeView !== 'Discover'\}<button[^>]*on:click=\{openTransactionHistory\}>History/);
  assert.match(app, /\{#if flatpakSourceEnabled\}<section[^>]*aria-labelledby="flatpak-remotes-heading"/);
  assert.match(app, /flatpakSourceEnabled \? ListFlatpakRemotes\(\)\.catch\(\(\) => \[\]\) : Promise\.resolve\(\[\]\)/);
  assert.match(app, /\{#if tool\[0\] !== 'Clean unused Flatpaks' \|\| flatpakSourceEnabled\}/);
  assert.match(app, /const result = await ReconfigureKernel\(item\.package\)/);
  assert.match(app, /kernelReconfigureLogs = result\.diagnostics/);
  assert.match(app, /<ServiceDetailsSidebar/);
  assert.match(app, /GetServiceDetails\(scope, service\.name\)/);
  assert.match(app, /\{:else if item\.installed && !item\.meta\}<span class="inline-flex/);
  assert.match(app, /title="Re-run post-install hooks and regenerate initramfs"/);
  assert.match(app, /GetKernelStatus\(\)/);
  assert.match(app, />Turnstile<\/button>/);
  assert.match(app, /InitializeUserServices\(\)/);
  assert.match(app, /services = await AddService\(serviceScope, serviceName\.trim\(\), serviceCommand\.trim\(\), serviceUser\.trim\(\), serviceGroup\.trim\(\)\)/);
  assert.match(app, />Command<\/span><input[^>]*placeholder="program --flag"/);
  assert.match(app, />Add service<\/button>/);
  assert.doesNotMatch(app, /id="service-action"/);
  assert.match(app, /operation === 'delete'/);
  assert.match(app, /EditUser\(accountUser\.trim\(\), accountHome\.trim\(\), accountShell\.trim\(\), accountMoveHome\)/);
  assert.match(app, /'primary-group-change'/);
  assert.match(app, /'password-unlock' : 'password-lock'/);
  assert.match(app, /ListAccountHistory\(\)/);
  assert.match(app, /user\.sessions \|\| user\.processes/);
  assert.match(app, /<details class="account-actions relative">/);
  assert.match(app, /No supplementary groups to unassign/);
  assert.match(app, /System accounts cannot be removed/);
  assert.match(app, /GetFirewall\(firewallSelectedBackend\)/);
  assert.match(app, /ChangeFirewall\(firewallChange/);
  assert.match(app, /aria-label="Installed firewall backends"/);
  assert.match(app, /Use exclusively/);
  assert.match(app, /SSH access rules are protected from removal/);
  assert.match(app, /firewallDialogMode === 'policy'/);
  assert.match(app, /firewallDialogMode === 'zone'/);
  assert.match(app, /if \(firewallCache\[backend\]\) \{\s*firewall = firewallCache\[backend\];/);
  assert.match(app, /firewallContentLoading/);
  assert.match(app, /on:click=\{reloadFirewallStatus\}/);
  assert.doesNotMatch(app, />Reload firewall<\/button>/);
  assert.doesNotMatch(app, />Reload status<\/button>/);
  assert.match(app, /Loading \{firewallSelectedBackend\} status/);
  assert.match(app, /ChangeTurnstileReady\(operation, names\)/);
  assert.match(app, /UpdateTurnstileEnvironment\(values\)/);
  assert.match(app, />Boot images <span/);
  assert.match(app, />DKMS <span/);
  assert.match(app, /'purge-kernels'/);
  assert.match(app, /on:click=\{\(\) => transactionLogsVisible = !transactionLogsVisible\}/);
  assert.match(app, /Native logs/);
  assert.match(app, /transactionExecutionLogs = result\.diagnostics/);
  assert.match(app, /Package state may have changed\. Reload the catalog/);
  assert.match(app, /InspectRepositoryKey\(repositoryURL\)/);
  assert.match(app, /Signing key fingerprint/);
});

test('keeps system tables scrollable and exposes requested management sections', () => {
  const app = readFileSync(new URL('../App.svelte', import.meta.url), 'utf8');
  const styles = readFileSync(new URL('../app.css', import.meta.url), 'utf8');

  assert.match(styles, /\.system-table\s*\{[^}]*height: 100%/s);
  assert.match(styles, /\.system-table-body\s*\{[^}]*overflow-y: auto/s);
  assert.doesNotMatch(app, /system-table overflow-auto/);
  assert.match(app, /role="switch"[^>]*aria-checked/);
  assert.match(app, /<span>Service<\/span><span>State<\/span><span>Enabled<\/span><span>PID<\/span><span>Command<\/span><span>Time<\/span>/);
  assert.match(app, /'Samba Server'/);
  assert.match(app, /'NFS Client'/);
  assert.match(app, /flex h-full min-h-0 w-full flex-col/);
  assert.doesNotMatch(app, /max-w-\[90rem\]/);
});

test('themes dialogs and shows an explicit selected appearance mode', () => {
  const app = readFileSync(new URL('../App.svelte', import.meta.url), 'utf8');
  const styles = readFileSync(new URL('../app.css', import.meta.url), 'utf8');

  assert.match(styles, /\.bg-surface-100-800\s*\{[^}]*var\(--app-panel\)/s);
  assert.match(styles, /\.app-chrome\s*\{[^}]*var\(--app-bg\)/s);
  assert.match(styles, /\.app-input-surface\s*\{[^}]*border: 1px solid var\(--app-border\)/s);
  assert.match(styles, /--color-success-600: #16a34a;/);
  assert.match(styles, /--color-warning-500: #f59e0b;/);
  assert.match(styles, /--color-error-600: #dc2626;/);
  assert.match(app, /<header class="app-chrome/);
  assert.match(app, /<aside class="app-chrome/);
  assert.match(app, /<footer class="app-chrome/);
  assert.match(styles, /dialog,\s*select,\s*select option\s*\{/s);
  assert.match(styles, /\.settings-mode-active\s*\{/);
  assert.match(styles, /\.package-maintenance\s*\{[^}]*container-type: inline-size/s);
  assert.match(styles, /\.maintenance-grid\s*\{[^}]*repeat\(3, minmax\(0, 1fr\)\)/s);
  assert.match(styles, /@container \(max-width: 50rem\)[^}]*\.maintenance-grid/s);
  assert.match(styles, /:root\s*\{[^}]*--color-surface-50: #edeeef;[^}]*--color-surface-300: #d5d3db;/s);
  assert.doesNotMatch(styles, /:root\[data-theme\]\s*\{/);
  assert.match(app, /aria-pressed=\{colorMode === mode\}/);
  assert.match(app, /type="number" min="10" max="24" step="1" value=\{fontSize\}/);
  assert.match(app, /aria-label="Decrease font size"/);
  assert.match(app, /aria-label="Increase font size"/);
  assert.doesNotMatch(app, /type="range"/);
  assert.match(app, /<NavigationIcon name=\{item\} \/>/);
  assert.match(app, /<NavigationIcon name="Settings" \/>Settings/);
  assert.match(app, /\['Verify package database',[\s\S]*\['Find owning package',[\s\S]*\['Remove orphans',[\s\S]*\['Clean package cache',[\s\S]*\['Clean unused Flatpaks',/);
  assert.match(app, /ListSystemFonts/);
});
