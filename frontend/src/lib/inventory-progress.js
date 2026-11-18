/** @param {{ scanned: number, total: number, done: boolean, cancelled: boolean }} progress */
export function progressLabel(progress) {
  if (progress.cancelled) return 'Metadata refresh cancelled';
  if (progress.done) return 'Metadata refresh complete';
  const percent = progress.total ? Math.round((progress.scanned / progress.total) * 100) : 0;
  return `Scanning metadata: ${percent}%`;
}
