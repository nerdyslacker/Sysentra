/** @typedef {{ id?: string, backendId?: string, name: string, version: string, repository: string, status: string, description: string }} Package */

/** @param {number} count @returns {Package[]} */
export function createMockPackages(count) {
  return Array.from({ length: count }, (_, index) => ({
    name: `package-${String(index).padStart(5, '0')}`,
    version: `1.${index % 100}.${index % 10}_1`,
    repository: index % 8 === 0 ? 'nonfree' : 'current',
    status: index % 13 === 0 ? 'Update available' : index % 3 === 0 ? 'Installed' : 'Available',
    description: `System utility ${index % 10}`,
  }));
}

/** @template {Package} T @param {T[]} packages */
export function createPackageSearchIndex(packages) {
  return packages.map((pkg) => ({
    package: pkg,
    name: pkg.name.toLowerCase(),
    text: [pkg.name, pkg.id, pkg.backendId, pkg.repository, pkg.description].filter(Boolean).join('\n').toLowerCase(),
  }));
}

/** @template {Package} T @param {T[]} packages @param {string} query @param {{ package: T, name: string, text: string }[]} [index] @returns {T[]} */
export function filterPackages(packages, query, index) {
  const term = query.trim().toLowerCase();
  if (!term) return packages;
  const exact = [], prefix = [], name = [], metadata = [];
  for (const entry of index ?? createPackageSearchIndex(packages)) {
    if (entry.name === term) exact.push(entry.package);
    else if (entry.name.startsWith(term)) prefix.push(entry.package);
    else if (entry.name.includes(term)) name.push(entry.package);
    else if (entry.text.includes(term)) metadata.push(entry.package);
  }
  return exact.concat(prefix, name, metadata);
}

/** @template {Package} T @param {T[]} packages @param {string} source @returns {T[]} */
export function filterPackageSource(packages, source) {
  return source ? packages.filter((pkg) => pkg.repository === source) : packages;
}

/** @template {Package & { held?: boolean, orphaned?: boolean, repoLocked?: boolean }} T @param {T[]} packages @param {'all'|'held'|'marked'|'orphaned'|'repoLocked'} state @param {Set<string>} selected @returns {T[]} */
export function filterPackageState(packages, state, selected) {
  if (state === 'all') return packages;
  return packages.filter((pkg) =>
    (state === 'held' && pkg.held) ||
    (state === 'marked' && selected.has(`${pkg.backendId}:${pkg.id}`)) ||
    (state === 'orphaned' && pkg.orphaned) ||
    (state === 'repoLocked' && pkg.repoLocked),
  );
}

/**
 * @template {Package & { installed?: boolean, newVersion?: string, held?: boolean, orphaned?: boolean, repoLocked?: boolean }} T
 * @param {T} pkg
 * @param {string} operation
 * @param {string[]} changedPackages
 * @returns {T}
 */
export function updatePackageAfterTransaction(pkg, operation, changedPackages) {
  if (!changedPackages.includes(pkg.id ?? pkg.name) && !changedPackages.includes(pkg.name)) return pkg;
  if (operation === 'remove' || operation === 'purge' || operation === 'force-remove' || operation === 'remove-orphans') {
    return { ...pkg, installed: false, status: 'Available', version: '', newVersion: '', held: false, orphaned: false, repoLocked: false };
  }
  if (operation === 'hold' || operation === 'unhold') return { ...pkg, held: operation === 'hold' };
  if (operation === 'repo-lock' || operation === 'repo-unlock') return { ...pkg, repoLocked: operation === 'repo-lock' };
  if (operation === 'install' || operation === 'reinstall' || operation === 'reconfigure' || operation === 'update') {
    return { ...pkg, installed: true, status: 'Installed', newVersion: '' };
  }
  return pkg;
}

/**
 * @param {number} scrollTop
 * @param {number} viewportHeight
 * @param {number} total
 * @param {number} rowHeight
 * @param {number} overscan
 */
export function visibleRange(scrollTop, viewportHeight, total, rowHeight, overscan) {
  const logicalHeight = total * rowHeight;
  const boundedScrollTop = Math.min(scrollTop, Math.max(0, logicalHeight - viewportHeight));
  const firstVisible = Math.min(Math.floor(boundedScrollTop / rowHeight), total);
  const start = Math.max(0, firstVisible - overscan);
  const end = Math.min(total, start + Math.ceil(viewportHeight / rowHeight) + overscan * 2);
  return { start, end, offset: start * rowHeight, spacerHeight: logicalHeight };
}
