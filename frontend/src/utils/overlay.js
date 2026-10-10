/** Shared overlay layers, scroll locking, focus and keyboard lifecycle. */
export const OVERLAY_LAYERS = Object.freeze({ dialog: 10000, loading: 20000, toast: 200000 });
const locks = new Set();
const dialogs = [];
let originalOverflow = '';
let originalPadding = '';
export function lockScroll(owner) {
  if (locks.has(owner)) return;
  if (!locks.size) {
    originalOverflow = document.body.style.overflow;
    originalPadding = document.body.style.paddingRight;
    const gap = Math.max(0, window.innerWidth - document.documentElement.clientWidth);
    if (gap) document.body.style.paddingRight = `${gap}px`;
    document.body.style.overflow = 'hidden';
  }
  locks.add(owner);
}
export function unlockScroll(owner) {
  if (!locks.delete(owner) || locks.size) return;
  document.body.style.overflow = originalOverflow;
  document.body.style.paddingRight = originalPadding;
}
export function nextDialogLayer() {
  return Math.max(OVERLAY_LAYERS.dialog - 2, ...dialogs.map(({ layer }) => layer)) + 2;
}
export function activateDialog(element, { close, labelId, initialFocus, layer = Number(element.style.zIndex) || nextDialogLayer() } = {}) {
  const previous = document.activeElement;
  const entry = { element, layer };
  dialogs.push(entry);
  lockScroll(entry);
  element.setAttribute('role', 'dialog');
  element.setAttribute('aria-modal', 'true');
  if (labelId) element.setAttribute('aria-labelledby', labelId);
  element.tabIndex = -1;
  const focusables = () => [...element.querySelectorAll('button:not([disabled]), a[href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex="0"]')]
    .filter(node => node.getClientRects().length && !node.closest('[inert]'));
  const keydown = event => {
    if (dialogs.at(-1) !== entry) return;
    if (event.key === 'Escape' && close) { event.preventDefault(); close(); }
    if (event.key === 'Tab') {
      const nodes = focusables();
      const first = nodes[0] || element;
      const last = nodes.at(-1) || element;
      if (!element.contains(document.activeElement) || (!event.shiftKey && document.activeElement === last) || (event.shiftKey && document.activeElement === first) || !nodes.length) {
        event.preventDefault(); (event.shiftKey ? last : first).focus();
      }
    }
  };
  document.addEventListener('keydown', keydown);
  const timer = setTimeout(() => {
    if (dialogs.at(-1) === entry) {
      const preferred = initialFocus ? element.querySelector(initialFocus) : null;
      (preferred && focusables().includes(preferred) ? preferred : focusables()[0] || element).focus();
    }
  }, 20);
  return () => {
    clearTimeout(timer);
    const top = dialogs.at(-1) === entry;
    const index = dialogs.indexOf(entry);
    if (index < 0) return;
    dialogs.splice(index, 1);
    document.removeEventListener('keydown', keydown);
    unlockScroll(entry);
    if (top) {
      const remaining = dialogs.at(-1)?.element;
      if (remaining && (!previous?.isConnected || !remaining.contains(previous))) remaining.focus();
      else if (previous?.isConnected) previous.focus();
    }
  };
}
