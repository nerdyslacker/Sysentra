import assert from 'node:assert/strict';
import test from 'node:test';

import { createMockPackages, createPackageSearchIndex, filterPackages, filterPackageSource, filterPackageState, updatePackageAfterTransaction, visibleRange } from './package-list.js';

test('creates the requested number of deterministic mock packages', () => {
  const packages = createMockPackages(50_000);

  assert.equal(packages.length, 50_000);
  assert.equal(packages[0].name, 'package-00000');
  assert.equal(packages[49_999].name, 'package-49999');
});

test('filters package names and descriptions without case sensitivity', () => {
  const packages = createMockPackages(100);

  assert.equal(filterPackages(packages, 'PACKAGE-00042')[0].name, 'package-00042');
  assert.equal(filterPackages(packages, 'UTILITY 7').length, 10);
  assert.equal(filterPackages(packages, '   '), packages);
});

test('filters real catalog rows by backend, package ID, and source', () => {
  const packages = [{
    id: 'org.example.Reader',
    backendId: 'flatpak',
    name: 'Reader',
    version: '2.0',
    repository: 'flathub',
    status: 'Available',
    description: '',
  }];

  assert.equal(filterPackages(packages, 'FLATPAK').length, 1);
  assert.equal(filterPackages(packages, 'org.example').length, 1);
  assert.equal(filterPackages(packages, 'flathub').length, 1);
});

test('reuses a normalized search index without rebuilding package text', () => {
  const packages = createMockPackages(100);
  const index = createPackageSearchIndex(packages);

  assert.equal(filterPackages(packages, 'UTILITY 7', index).length, 10);
  assert.equal(filterPackages(packages, 'PACKAGE-00042', index)[0].name, 'package-00042');
  assert.equal(index[0].package, packages[0]);
});

test('ranks an exact package name before related descriptions and language packs', () => {
  const packages = [
    { name: 'browser-helper', description: 'Helper for Firefox', version: '', repository: '', status: 'Available' },
    { name: 'firefox-i18n-en-US', description: 'Firefox language pack', version: '', repository: '', status: 'Available' },
    { name: 'firefox', description: 'Web browser', version: '', repository: '', status: 'Available' },
  ];

  assert.deepEqual(filterPackages(packages, 'firefox').map((pkg) => pkg.name), ['firefox', 'firefox-i18n-en-US', 'browser-helper']);
});

test('filters packages by exact source while retaining the all-sources option', () => {
  const packages = createMockPackages(16);

  assert.equal(filterPackageSource(packages, 'nonfree').length, 2);
  assert.equal(filterPackageSource(packages, '').length, 16);
});

test('filters native XBPS states and current transaction marks independently', () => {
  const packages = [
    { backendId: 'xbps', id: 'held', name: 'held', version: '1_1', repository: '', status: 'Installed', description: '', held: true },
    { backendId: 'xbps', id: 'orphan', name: 'orphan', version: '1_1', repository: '', status: 'Installed', description: '', orphaned: true },
    { backendId: 'xbps', id: 'locked', name: 'locked', version: '1_1', repository: '', status: 'Installed', description: '', repoLocked: true },
  ];
  assert.equal(filterPackageState(packages, 'held', new Set())[0].id, 'held');
  assert.equal(filterPackageState(packages, 'orphaned', new Set())[0].id, 'orphan');
  assert.equal(filterPackageState(packages, 'repoLocked', new Set())[0].id, 'locked');
  assert.equal(filterPackageState(packages, 'marked', new Set(['xbps:orphan']))[0].id, 'orphan');
});

test('updates selected package state after a transaction', () => {
  const installed = { backendId: 'xbps', id: 'firefox', name: 'firefox', version: '130_1', newVersion: '', repository: 'current', status: 'Installed', description: '', installed: true };
  const removed = updatePackageAfterTransaction(installed, 'remove', ['firefox']);
  assert.equal(removed.installed, false);
  assert.equal(removed.status, 'Available');

  const available = { ...installed, installed: false, status: 'Available' };
  const added = updatePackageAfterTransaction(available, 'install', ['firefox']);
  assert.equal(added.installed, true);
  assert.equal(added.status, 'Installed');
  const updated = updatePackageAfterTransaction({ ...installed, newVersion: '131_1', status: 'Update available' }, 'update', ['firefox']);
  assert.equal(updated.version, '131_1');
  assert.equal(updated.newVersion, '');
  assert.equal(updated.status, 'Installed');
  assert.equal(updatePackageAfterTransaction(installed, 'force-remove', ['firefox']).installed, false);
  assert.equal(updatePackageAfterTransaction(installed, 'remove', ['bash']), installed);
});

test('calculates a bounded visible window with overscan', () => {
  assert.deepEqual(visibleRange(440, 220, 100, 44, 2), {
    start: 8,
    end: 17,
    offset: 352,
    spacerHeight: 4_400,
  });
  assert.deepEqual(visibleRange(99_999, 220, 4, 44, 2), {
    start: 0,
    end: 4,
    offset: 0,
    spacerHeight: 176,
  });
});

test('reaches the end of a large virtual inventory', () => {
  assert.deepEqual(visibleRange(2_199_560, 440, 50_000, 44, 8), {
    start: 49_982,
    end: 50_000,
    offset: 2_199_208,
    spacerHeight: 2_200_000,
  });
});
