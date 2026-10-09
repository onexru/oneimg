/** Text/attribute escaping for the remaining trusted static HTML templates. */
export const escapeHtml = value => String(value ?? '')
  .replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
  .replaceAll('"', '&quot;').replaceAll("'", '&#039;');

/** Only HTTP(S) resources; never accept javascript:, data: or credentials. */
export const safeResourceUrl = (value, base = globalThis.location?.origin || 'https://localhost') => {
  try {
    const url = new URL(String(value ?? ''), base);
    return ['http:', 'https:'].includes(url.protocol) && !url.username && !url.password ? url.href : '';
  } catch { return ''; }
};
