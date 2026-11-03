import { applyFeedbackStyles } from './feedbackStyles.js';
import { OVERLAY_LAYERS, activateDialog, nextDialogLayer } from './overlay.js';

class PopupModal {
  constructor(options = {}) {
    this.defaults = {
      id: `modal-${Date.now()}-${Math.random().toString(36).slice(2)}`,
      title: '提示',
      content: '',
      type: 'default',
      width: 'auto',
      showClose: true,
      closeLabel: '关闭弹窗',
      showFooter: true,
      mask: true,
      maskClose: true,
      zIndex: nextDialogLayer(),
      buttons: [
        { text: '取消', type: 'default', callback: (modal) => modal.close() },
        { text: '确认', type: 'primary', callback: null }
      ],
      formFields: [],
      formSubmit: null,
      onOpen: null,
      onClose: null,
      // 关闭动画「开始时」的回调，与 onClose（动画结束）区分开
      onCloseStart: null,
      // 以下为 Vue 宿主模式（PopupModalShell）使用的扩展项
      icon: '',
      subtitle: '',
      confirmText: '',
      contentMaxHeight: ''
    };

    this.config = { ...this.defaults, ...options };
    this.state = { isOpen: false, formData: {} };

    if (this.config.type === 'form' && this.config.formFields.length) {
      this.config.formFields.forEach(field => {
        this.state.formData[field.name] = field.defaultValue || '';
      });
    }

    this.createElements();
    this.bindMaskEvent();
  }

  createElements() {
    // 宽度必须受视口约束：md 是 500px，在 390px 手机屏上会横向溢出。
    // 用内联 min()，不依赖 Tailwind 任意值对嵌套 calc 的解析。
    const widthMap = { sm: 380, md: 500, lg: 700, full: '90vw', auto: 500 };
    const width = widthMap[this.config.width] ?? 500;

    this.mask = document.createElement('div');
    this.mask.id = `${this.config.id}-mask`;
    this.mask.className = 'fixed inset-0 bg-slate-900/40 backdrop-blur-[2px] transition-opacity duration-300 opacity-0 pointer-events-none';
    this.mask.style.zIndex = this.config.zIndex - 1;
    document.body.appendChild(this.mask);

    this.modal = document.createElement('div');
    this.modal.id = this.config.id;
    // flex 纵向 + max-h：内容超长时只有中间区滚动，标题和底部按钮始终可见。
    // 之前是整体 max-h，长表单会把「保存」按钮顶到屏幕外。
    this.modal.className = `fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 scale-95 opacity-0 transition-all duration-300 pointer-events-none flex flex-col max-h-[min(90dvh,calc(100dvh-24px))] rounded-2xl border border-slate-200 bg-white shadow-xl dark:border-white/10 dark:bg-slate-900`;
    this.modal.style.zIndex = this.config.zIndex;
    this.modal.style.width = typeof width === 'number' ? `min(${width}px, calc(100vw - 24px))` : `min(${width}, calc(100vw - 24px))`;

    this.header = document.createElement('div');
    this.header.className = 'px-6 py-4 border-b border-light-200 dark:border-dark-100 flex justify-between items-center gap-3';

    const heading = document.createElement('div');
    heading.className = 'min-w-0';

    this.titleEl = document.createElement('h3');
    this.titleEl.className = 'font-semibold text-lg text-dark-300 dark:text-light-100 truncate';
    this.titleEl.id = `${this.config.id}-title`;
    if (this.config.icon) {
      const icon = document.createElement('i');
      icon.className = `${this.config.icon} mr-1.5 align-[-2px] text-primary`;
      icon.setAttribute('aria-hidden', 'true');
      this.titleEl.appendChild(icon);
    }
    this.titleEl.appendChild(document.createTextNode(this.config.title));
    heading.appendChild(this.titleEl);

    // 副标题：说明「这个弹窗会影响什么」，不是所有弹窗都需要
    if (this.config.subtitle) {
      this.subtitleEl = document.createElement('p');
      this.subtitleEl.className = 'mt-1 text-[13px] leading-5 text-secondary';
      this.subtitleEl.textContent = this.config.subtitle;
      heading.appendChild(this.subtitleEl);
    }
    this.header.appendChild(heading);

    if (this.config.showClose) {
      this.closeBtn = document.createElement('button');
      this.closeBtn.type = 'button';
      this.closeBtn.setAttribute('aria-label', this.config.closeLabel || '关闭弹窗');
      this.closeBtn.className = 'w-8 h-8 shrink-0 flex items-center justify-center rounded-lg text-secondary hover:text-danger transition-colors';
      this.closeBtn.innerHTML = '<i class="ri-close-fill font-bold text-[1.35rem]"></i>';
      this.closeBtn.addEventListener('click', () => this.close());
      this.header.appendChild(this.closeBtn);
    }
    this.modal.appendChild(this.header);

    this.content = document.createElement('div');
    this.content.style.minHeight = '0';
    this.content.style.overflowWrap = 'anywhere';
    this.content.style.overflowY = 'auto';
    // 用 flex:1 吃掉剩余高度，比写死 60vh 更贴合实际可用空间：
    // 矮屏（手机横屏）不会把内容压成一条缝，高屏也不会提前出现滚动条。
    this.content.style.flex = '1 1 auto';
    if (this.config.contentMaxHeight) this.content.style.maxHeight = this.config.contentMaxHeight;

    if (this.config.contentEl) {
      // Vue 宿主模式：内容区交给外部渲染，内边距由内容自己控制
      this.content.className = 'min-h-0';
      this.content.appendChild(this.config.contentEl);
    } else {
      this.content.className = 'px-6 py-5';
      if (this.config.type === 'form') {
        this.renderFormContent();
      } else {
        this.content.innerHTML = this.config.content;
      }
    }
    this.modal.appendChild(this.content);

    if (this.config.showFooter) {
      this.footer = document.createElement('div');
      this.footer.className = 'px-6 py-4 border-t border-light-200 dark:border-dark-100 flex flex-wrap justify-end gap-3';
      if (this.config.footerEl) {
        // 同上：按钮由外部渲染，避免再写一套按钮样式
        this.footer.appendChild(this.config.footerEl);
      } else {
        this.renderButtons();
      }
      this.modal.appendChild(this.footer);
    }

    document.body.appendChild(this.modal);
  }

  bindMaskEvent() {
    if (this.config.mask && this.config.maskClose) {
      this.maskClickHandler = () => {
        if (this.state.isOpen) this.close();
      };
      this.mask.addEventListener('click', this.maskClickHandler);
    }
  }

  renderFormContent() {
    this.content.innerHTML = '';
    const form = document.createElement('form');
    form.className = 'space-y-4';
    
    form.method = 'post';
    form.action = location.pathname;
    
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      e.stopPropagation();
      e.stopImmediatePropagation();
      this.handleFormSubmit();
      return false;
    });

    this.config.formFields.forEach(field => {
      const fieldGroup = document.createElement('div');
      fieldGroup.className = 'space-y-2';

      const label = document.createElement('label');
      label.className = 'block text-sm font-medium text-dark-300 dark:text-light-100';
      label.textContent = field.label;
      if (field.required) label.innerHTML += '<span class="text-danger ml-1">*</span>';
      fieldGroup.appendChild(label);

      let input;
      switch (field.type) {
        case 'textarea':
          input = document.createElement('textarea');
          input.className = 'w-full px-3 py-2 border border-light-200 dark:border-dark-100 rounded-md bg-light-100 dark:bg-dark-300 focus:outline-none focus:ring-1 focus:ring-primary focus:border-primary';
          input.rows = field.rows || 3;
          input.placeholder = field.placeholder || '';
          break;
        
        case 'select':
          input = document.createElement('select');
          input.className = 'w-full px-3 py-2 border border-light-200 dark:border-dark-100 rounded-md bg-light-100 dark:bg-dark-300 focus:outline-none focus:ring-1 focus:ring-primary focus:border-primary';
          if (field.options && field.options.length) {
            field.options.forEach(opt => {
              const option = document.createElement('option');
              option.value = opt.value;
              option.textContent = opt.label;
              option.disabled = opt.disabled || false;
              if (opt.value === this.state.formData[field.name]) option.selected = true;
              input.appendChild(option);
            });
          }
          break;
        
        default:
          input = document.createElement('input');
          input.type = field.type || 'text';
          input.className = 'w-full px-3 py-2 border border-light-200 dark:border-dark-100 rounded-md bg-light-100 dark:bg-dark-300 focus:outline-none focus:ring-1 focus:ring-primary focus:border-primary';
          input.placeholder = field.placeholder || '';
          break;
      }

      input.name = field.name;
      input.id = `${this.titleEl.id}-${field.name}`;
      label.htmlFor = input.id;
      input.value = this.state.formData[field.name] || '';
      if (field.required) input.required = true;
      if (field.disabled) input.disabled = true;
      // 输入长度限制交给浏览器原生 maxlength，避免用户在超长内容上白填一遍
      if (field.maxlength) input.maxLength = field.maxlength;

      input.addEventListener('change', (e) => {
        this.state.formData[field.name] = e.target.value;
        if (field.onChange) field.onChange(this, e.target.value);
      });

      input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && field.type !== 'textarea') {
          e.preventDefault();
          const confirmBtn = this.footer.querySelector('[class*="bg-primary"]');
          if (confirmBtn) confirmBtn.click();
        }
      });

      fieldGroup.appendChild(input);

      if (field.tip) {
        const tip = document.createElement('p');
        tip.className = 'text-xs text-secondary';
        tip.classList.add('whitespace-pre-line', 'break-words');
        tip.textContent = field.tip;
        fieldGroup.appendChild(tip);
      }

      form.appendChild(fieldGroup);
    });

    this.content.appendChild(form);
  }

  renderButtons() {
    this.footer.innerHTML = '';
    this.config.buttons.forEach((btn) => {
      const button = document.createElement('button');
      
      button.type = 'button';
      
      const btnStyles = {
        default: 'px-4 py-2 border border-light-200 dark:border-dark-100 rounded-md text-dark-300 dark:text-light-100 bg-white dark:bg-dark-200 hover:bg-light-100 dark:hover:bg-dark-300 transition-colors',
        primary: 'px-4 py-2 rounded-md text-white bg-primary hover:bg-primary-dark transition-colors',
        danger: 'px-4 py-2 rounded-md text-white bg-danger hover:bg-danger/90 transition-colors'
      };
      button.className = btnStyles[btn.type] || btnStyles.default;
      button.textContent = btn.text;

      button.addEventListener('click', () => {
        if (typeof btn.callback === 'function') {
          btn.callback(this, this.state.formData);
        } else {
          this.close();
        }
      });

      this.footer.appendChild(button);
    });
  }

  handleFormSubmit() {
    if (typeof this.config.formSubmit === 'function') {
      this.config.formSubmit(this, this.state.formData);
    } else {
      this.close();
    }
  }

  open() {
    if (this.state.isOpen) return;
    // 复用实例时，上次关闭的 onClose 可能还没执行就被取消了；
    // 这里补发一次，保证「每次 open 都有对应的 close 回调」这个约定不被打破。
    if (this.closeModalTimer) {
      clearTimeout(this.closeModalTimer);
      if (typeof this.config.onClose === 'function') this.config.onClose(this);
    }
    clearTimeout(this.closeMaskTimer);
    this.modal.style.zIndex = Math.max(nextDialogLayer(), Number(this.config.zIndex) || OVERLAY_LAYERS.dialog);
    this.mask.style.zIndex = Number(this.modal.style.zIndex) - 1;
    if (!this.modal.isConnected) document.body.appendChild(this.modal);
    if (!this.mask.isConnected) document.body.appendChild(this.mask);
    if (this.config.mask) {
      this.mask.classList.remove('pointer-events-none');
      setTimeout(() => {
        this.mask.classList.remove('opacity-0');
        this.mask.classList.add('opacity-100');
      }, 10);
    }
    
    this.modal.inert = false;
    this.modal.removeAttribute('aria-hidden');
    this.modal.classList.remove('pointer-events-none');
    setTimeout(() => {
      this.modal.classList.remove('scale-95', 'opacity-0');
      this.modal.classList.add('scale-100', 'opacity-100');
    }, 10);
    
    this.state.isOpen = true;
    
    if (typeof this.config.onOpen === 'function') this.config.onOpen(this);
    this.deactivateDialog = activateDialog(this.modal, { labelId: this.titleEl.id, close: () => this.close() });
  }

  close() {
    if (!this.state.isOpen) return;
    
    this.deactivateDialog?.();
    this.deactivateDialog = null;
    this.modal.inert = true;
    this.modal.removeAttribute('role');
    this.modal.removeAttribute('aria-modal');
    this.modal.setAttribute('aria-hidden', 'true');
    if (this.config.mask) {
      this.mask.classList.remove('opacity-100');
      this.mask.classList.add('opacity-0');
      this.closeMaskTimer = setTimeout(() => {
        this.mask.classList.add('pointer-events-none');
        this.mask.remove();
      }, 300);
    }
    
    this.modal.classList.remove('scale-100', 'opacity-100');
    this.modal.classList.add('scale-95', 'opacity-0');
    this.closeModalTimer = setTimeout(() => {
      this.modal.classList.add('pointer-events-none');
      this.modal.remove();
      // 通知外部「已关闭」必须等动画结束。
      // 之前是在这里同步回调，父组件立刻 v-if=false，Vue 卸载时把内容节点带走，
      // 而 300ms 的淡出还在跑 —— 用户看到一个空壳在慢慢消失，就是那个「闪一下」。
      if (typeof this.config.onClose === 'function') this.config.onClose(this);
    }, 300);

    this.state.isOpen = false;

    // 需要立即响应的场景（比如清空当前预览图引用）用这个，不参与动画
    if (typeof this.config.onCloseStart === 'function') this.config.onCloseStart(this);
    this.modal.classList.add('pointer-events-none');
    this.mask.classList.add('pointer-events-none');
  }

  update(options) {
    if (options.title) {
      this.config.title = options.title;
      this.titleEl.textContent = options.title;
    }
    if (options.subtitle !== undefined && this.subtitleEl) {
      this.config.subtitle = options.subtitle;
      this.subtitleEl.textContent = options.subtitle;
    }
    if (options.content && this.config.type !== 'form' && !this.config.contentEl) {
      this.config.content = options.content;
      this.content.innerHTML = options.content;
    }
    if (options.buttons) {
      this.config.buttons = options.buttons;
      if (this.footer) this.renderButtons();
    }
    if (options.formFields && this.config.type === 'form') {
      this.config.formFields = options.formFields;
      this.renderFormContent();
    }
  }

  appendFormFields(newFields = [], keepFieldNames = []) {
    if (this.config.type !== 'form') return;
    
    const keepFields = this.config.formFields.filter(f => keepFieldNames.includes(f.name));
    
    newFields.forEach(field => {
      if (this.state.formData[field.name] === undefined || this.state.formData[field.name] === '') {
        this.state.formData[field.name] = field.defaultValue ?? field.value ?? '';
      }
    });

    const filledNewFields = newFields.map(field => ({
      ...field,
      defaultValue: this.state.formData[field.name] ?? field.defaultValue ?? field.value ?? ''
    }));

    this.update({ formFields: [...keepFields, ...filledNewFields] });
  }

  destroy() {
    if (this.maskClickHandler) {
      this.mask.removeEventListener('click', this.maskClickHandler);
    }
    // 已被销毁就不要再走一遍关闭流程，否则 onClose 会在组件卸载后再 emit 一次
    if (this.config.onClose) this.config.onClose = null;
    this.close();
    setTimeout(() => {
      if (this.mask && this.mask.parentNode) this.mask.parentNode.removeChild(this.mask);
      if (this.modal && this.modal.parentNode) this.modal.parentNode.removeChild(this.modal);
    }, 300);
  }
}

window.PopupModal = PopupModal;

window.showAlert = function(content, title = '提示', callback) {
  const modal = new PopupModal({
    title, content, type: 'default',
    buttons: [{
      text: '确定', type: 'primary',
      callback: (modal) => { modal.close(); if (typeof callback === 'function') callback(); }
    }]
  });
  modal.open();
  return modal;
};

window.showConfirm = function(content, title = '确认', confirmCallback, cancelCallback) {
  const modal = new PopupModal({
    title, content, type: 'confirm',
    buttons: [
      { text: '取消', type: 'default', callback: (modal) => { modal.close(); if (typeof cancelCallback === 'function') cancelCallback(); } },
      { text: '确认', type: 'primary', callback: (modal) => { modal.close(); if (typeof confirmCallback === 'function') confirmCallback(); } }
    ]
  });
  modal.open();
  return modal;
};

window.showFormModal = function(options) {
  const modal = new PopupModal({ type: 'form', ...options });
  modal.open();
  return modal;
};

export default PopupModal;