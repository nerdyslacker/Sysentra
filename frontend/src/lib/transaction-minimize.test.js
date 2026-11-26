import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

import { isTransactionMinimizable, minimizedStatusFor } from './transaction-minimize.js';

const app = readFileSync(new URL('../App.svelte', import.meta.url), 'utf8');

test('minimizes and restores the package transaction dialog', () => {
  assert.match(app, /let transactionMinimized = false;/);
  assert.match(app, /let minimizedTransactionWidget: HTMLButtonElement;/);
  assert.match(app, /\$: transactionMinimizable = isTransactionMinimizable\(transactionPlan, transactionLoading, confirmationAction, transactionOutcome\);/);
  assert.match(app, /bind:this=\{minimizedTransactionWidget\}/);
  assert.match(app, /tick\(\)\.then\(\(\) => minimizedTransactionWidget\?\.focus\(\)\)/);
  assert.match(app, /function minimizeTransactionDialog\(\): void \{\s*if \(!transactionMinimizable\) return;\s*transactionMinimized = true;\s*confirmationDialog\.close\(\);\s*void tick\(\)\.then\(\(\) => minimizedTransactionWidget\?\.focus\(\)\);\s*\}/);
  assert.match(app, /function restoreTransactionDialog\(\): void \{\s*transactionMinimized = false;\s*if \(!confirmationDialog\.open\) confirmationDialog\.showModal\(\);\s*\}/);
  assert.match(app, /function focusMinimizedTransaction\(\): boolean \{\s*if \(!transactionMinimized\) return false;\s*restoreTransactionDialog\(\);\s*return true;\s*\}/);
});

test('shows a bottom-right widget with live status and no invented percentage', () => {
  assert.match(app, /aria-label="Restore package transaction"/);
  assert.match(app, /class="fixed bottom-12 right-4 z-40/);
  assert.match(app, /<span class="sr-only" role="status" aria-live="polite">\{transactionMinimized \? minimizedTransactionStatus : ''\}<\/span>/);
  assert.doesNotMatch(app, /role="status" aria-live="polite">\{minimizedTransactionStatus\}/);
  assert.match(app, /animate-pulse/);
  assert.doesNotMatch(app, /transactionProgressPercent|percentComplete/);
});

test('exposes minimize only for package transactions', () => {
  assert.match(app, /\{#if transactionMinimizable\}<button type="button" class="btn preset-tonal ml-auto" on:click=\{minimizeTransactionDialog\}>Minimize<\/button>\{\/if\}/);
  assert.match(app, /\$: minimizedTransactionStatus = minimizedStatusFor\(transactionOutcome, transactionError, transactionLoading, streamingTransactionRequest, transactionPlan\);/);
});

test('minimizable excludes plain confirmation actions and finished states stay minimizable', () => {
  assert.equal(isTransactionMinimizable(null, true, () => {}, ''), false);
  assert.equal(isTransactionMinimizable(null, true, null, ''), true);
  assert.equal(isTransactionMinimizable({}, false, null, ''), true);
  assert.equal(isTransactionMinimizable(null, false, null, 'success'), true);
  assert.equal(isTransactionMinimizable(null, false, null, ''), false);
});

test('status reports preview failure, commit phase, and finished outcomes', () => {
  const plan = {};
  assert.equal(minimizedStatusFor('success', '', false, '', plan), 'Transaction complete.');
  assert.equal(minimizedStatusFor('failure', '', false, '', plan), 'Transaction failed.');
  assert.equal(minimizedStatusFor('', 'preview exploded', false, '', null), 'Transaction failed.');
  assert.equal(minimizedStatusFor('', '', true, '', null), 'Preparing transaction preview…');
  assert.equal(minimizedStatusFor('', '', true, 'request-1', plan), 'Applying transaction…');
  assert.equal(minimizedStatusFor('', '', false, '', plan), 'Ready to confirm.');
  assert.equal(minimizedStatusFor('', '', false, '', null), 'Preparing transaction preview.');
});

test('guards every transaction opener against overwriting a minimized transaction', () => {
  assert.equal((app.match(/if \(focusMinimizedTransaction\(\)\) return;/g) ?? []).length, 9);
});

test('keeps only Minimize in the dialog header and the logs toggle beside Cancel', () => {
  assert.match(app, /\{#if transactionMinimizable\}<button type="button" class="btn preset-tonal ml-auto" on:click=\{minimizeTransactionDialog\}>Minimize<\/button>\{\/if\}<\/header>/);
  assert.match(app, /<footer class="flex justify-end gap-2 border-t border-surface-300-700 px-4 py-3">\s*\{#if transactionPlan \|\| transactionLogs\.length\}<button type="button" class="btn preset-tonal" aria-expanded=\{transactionLogsVisible\}/);
});
