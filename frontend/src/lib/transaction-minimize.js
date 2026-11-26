/**
 * @param {object | null} plan
 * @param {boolean} loading
 * @param {(() => Promise<void>) | null} action
 * @param {'success' | 'failure' | ''} outcome
 */
export function isTransactionMinimizable(plan, loading, action, outcome) {
  return Boolean(plan) || (loading && !action) || Boolean(outcome);
}

/**
 * @param {'success' | 'failure' | ''} outcome
 * @param {string} error
 * @param {boolean} loading
 * @param {string} streamingRequest
 * @param {object | null} plan
 */
export function minimizedStatusFor(outcome, error, loading, streamingRequest, plan) {
  if (outcome === 'success') return 'Transaction complete.';
  if (outcome === 'failure' || error) return 'Transaction failed.';
  if (loading) return streamingRequest ? 'Applying transaction…' : 'Preparing transaction preview…';
  return plan ? 'Ready to confirm.' : 'Preparing transaction preview.';
}
