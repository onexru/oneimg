import PopupModal from './popupModal.js';
/** Each view/controller owns and releases its popups on unmount, including unopened dialogs. */
export function createDialogScope() {
  const instances = new Set();
  const Dialog = class extends PopupModal {
    constructor(options = {}) {
      const onClose = options.onClose;
      super({ ...options, onClose: modal => { instances.delete(modal); onClose?.(modal); } });
      instances.add(this);
    }
  };
  return { Dialog, dispose: () => { for (const modal of [...instances]) modal.destroy(); instances.clear(); } };
}
