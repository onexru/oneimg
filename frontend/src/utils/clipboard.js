export async function copyToClipboard(text) {
  try {
    if (globalThis.navigator?.clipboard && globalThis.isSecureContext) {
      await navigator.clipboard.writeText(String(text)); return true;
    }
  } catch { /* Retry via the browser fallback. */ }
  const previous = document.activeElement;
  const input = document.createElement('textarea');
  input.value = String(text);
  Object.assign(input.style, { position: 'fixed', opacity: '0' });
  document.body.appendChild(input); input.select();
  let copied = false;
  try { copied = document.execCommand('copy'); } catch { /* unsupported */ }
  input.remove();
  if (previous?.isConnected) previous.focus();
  return copied;
}
