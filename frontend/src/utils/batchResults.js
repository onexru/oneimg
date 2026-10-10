/** Legacy upload API may return successful images even with a partial/error HTTP status. */
export function uploadOutcome(result, expected = 1) {
  const data = result?.data;
  const images = Array.isArray(data) ? data : Array.isArray(data?.images) ? data.images
    : Array.isArray(data?.successes) ? data.successes : Array.isArray(data?.uploaded) ? data.uploaded : [];
  const files = Array.isArray(data?.files) ? data.files : [];
  const successful = files.length ? files.filter(file => file.success === true) : images;
  const errors = files.length ? files.filter(file => file.success !== true)
    : Array.isArray(result?.errors) ? result.errors : Array.isArray(data?.errors) ? data.errors : [];
  return { images: successful, errors, failed: Math.max(errors.length, Number(data?.failed_count) || 0, expected - successful.length, 0) };
}

export async function settleDeletions(ids, remove) {
  const outcomes = await Promise.allSettled(ids.map(id => remove(id)));
  return { succeeded: ids.filter((_, i) => outcomes[i].status === 'fulfilled' && outcomes[i].value === true),
    failed: ids.filter((_, i) => outcomes[i].status !== 'fulfilled' || outcomes[i].value !== true) };
}

export function clampUsage(value) {
  const n = Number(value);
  return Number.isFinite(n) ? Math.min(100, Math.max(0, n)) : 0;
}
