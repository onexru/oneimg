/** Incremental rendering keeps first paint small without changing selection/model IDs. */
export function nextRenderCount(current, total, chunk = 24) {
  const length = Math.max(0, Math.floor(Number(total) || 0));
  const step = Math.max(1, Math.floor(Number(chunk) || 24));
  return Math.min(length, Math.max(0, Math.floor(Number(current) || 0)) + step);
}
