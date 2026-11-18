import assert from 'node:assert/strict';
import test from 'node:test';

import { progressLabel } from './inventory-progress.js';

test('formats active, completed, and cancelled scan progress', () => {
  assert.equal(progressLabel({ scanned: 25, total: 100, done: false, cancelled: false }), 'Scanning metadata: 25%');
  assert.equal(progressLabel({ scanned: 100, total: 100, done: true, cancelled: false }), 'Metadata refresh complete');
  assert.equal(progressLabel({ scanned: 40, total: 100, done: true, cancelled: true }), 'Metadata refresh cancelled');
});

test('handles an empty total without dividing by zero', () => {
  assert.equal(progressLabel({ scanned: 0, total: 0, done: false, cancelled: false }), 'Scanning metadata: 0%');
});
