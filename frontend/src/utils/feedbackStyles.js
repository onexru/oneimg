/** Shared mechanics only; each utility's distinct colors/layout remain unchanged. */
export const FEEDBACK_STYLE = Object.freeze({
  box: Object.freeze({ boxSizing: 'border-box' }),
  hidden: Object.freeze({ opacity: '0' }),
  inset: Object.freeze({ top: '0', left: '0', right: '0', bottom: '0' }),
  centered: Object.freeze({ display: 'flex', alignItems: 'center', justifyContent: 'center' }),
});
export function applyFeedbackStyles(element, styles) {
  if (!element) return;
  for (const [key, value] of Object.entries(styles)) element.style.setProperty(key.replace(/[A-Z]/g, char => `-${char.toLowerCase()}`), value);
}
