class PopupModal {
  constructor(options = {}) {
    this.defaults = {
      id: `modal-${Date.now()}`,
      title: '提示',
      content: '',
      type: 'default',
      width: 'auto',
      showClose: true,
      mask: true,
      maskClose: true,
      zIndex: 9999,
      buttons: [
        { text: '取消', type: 'default', callback: (modal) => modal.close() },
        { text: '确认', type: 'primary', callback: null }
      ],
      formFields: [],
      formSubmit: null,
      onOpen: null,
      onClose: null
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
    const widthMap = {
      sm: 'w-[300px]', md: 'w-[500px]', lg: 'w-[700px]', full: 'w-[90%]',
      auto: ['w-[calc(100%-20px)]', 'min-w-[320px]', 'max-w-[500px]']
    };

    this.mask = document.createElement('div');
    this.mask.id = `${this.config.id}-mask`;
    this.mask.className = 'fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm transition-opacity duration-300 opacity-0 pointer-events-none';
    this.mask.style.zIndex = this.config.zIndex - 1;
    document.body.appendChild(this.mask);

    this.modal = document.createElement('div');
    this.modal.id = this.config.id;
    this.modal.className = 'fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 scale-95 opacity-0 transition-all duration-300 pointer-events-none rounded-xl bg-white dark:bg-dark-200 shadow-lg dark:shadow-dark-lg overflow-hidden';
    this.modal.style.zIndex = this.config.zIndex;
    this.modal.classList.add(...(widthMap[this.config.width] || widthMap.auto));

    this.header = document.createElement('div');
    this.header.className = 'px-6 py-4 border-b border-light-200 dark:border-dark-100 flex justify-between items-center';
    
    this.titleEl = document.createElement('h3');
    this.titleEl.className = 'font-semibold text-lg text-dark-300 dark:text-light-100 w-[50%] truncate';
    this.titleEl.innerHTML = this.config.title;
    this.header.appendChild(this.titleEl);

    if (this.config.showClose) {
      this.closeBtn = document.createElement('button');
      this.closeBtn.type = 'button';
      this.closeBtn.className = 'w-8 h-8 flex items-center justify-center text-secondary hover:text-danger transition-colors';
      this.closeBtn.innerHTML = '<i class="ri-close-fill font-bold text-[1.35rem]"></i>';
      this.closeBtn.addEventListener('click', () => this.close());
      this.header.appendChild(this.closeBtn);
    }
    this.modal.appendChild(this.header);

    this.content = document.createElement('div');
    this.content.className = 'px-6 py-5 max-h-[60vh] overflow-y-auto';
    
    if (this.config.type === 'form') {
      this.renderFormContent();
    } else {
      this.content.innerHTML = this.config.content;
    }
    this.modal.appendChild(this.content);

    this.footer = document.createElement('div');
    this.footer.className = 'px-6 py-4 border-t border-light-200 dark:border-dark-100 flex justify-end gap-3';
    this.renderButtons();
    this.modal.appendChild(this.footer);

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
    form.action = 'javascript:void(0)';
    
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
      input.value = this.state.formData[field.name] || '';
      if (field.required) input.required = true;
      if (field.disabled) input.disabled = true;

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
        tip.innerHTML = field.tip;
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
    
    if (this.config.mask) {
      this.mask.classList.remove('pointer-events-none');
      setTimeout(() => {
        this.mask.classList.remove('opacity-0');
        this.mask.classList.add('opacity-100');
      }, 10);
    }
    
    this.modal.classList.remove('pointer-events-none');
    setTimeout(() => {
      this.modal.classList.remove('scale-95', 'opacity-0');
      this.modal.classList.add('scale-100', 'opacity-100');
    }, 10);
    
    this.state.isOpen = true;
    
    if (typeof this.config.onOpen === 'function') this.config.onOpen(this);
    document.body.style.overflow = 'hidden';
  }

  close() {
    if (!this.state.isOpen) return;
    
    if (this.config.mask) {
      this.mask.classList.remove('opacity-100');
      this.mask.classList.add('opacity-0');
      setTimeout(() => {
        this.mask.classList.add('pointer-events-none');
        this.mask.remove();
      }, 300);
    }
    
    this.modal.classList.remove('scale-100', 'opacity-100');
    this.modal.classList.add('scale-95', 'opacity-0');
    setTimeout(() => {
      this.modal.classList.add('pointer-events-none');
      this.modal.remove();
    }, 300);

    this.state.isOpen = false;
    
    if (typeof this.config.onClose === 'function') this.config.onClose(this);
    document.body.style.overflow = '';
  }

  update(options) {
    if (options.title) {
      this.config.title = options.title;
      this.titleEl.textContent = options.title;
    }
    if (options.content && this.config.type !== 'form') {
      this.config.content = options.content;
      this.content.innerHTML = options.content;
    }
    if (options.buttons) {
      this.config.buttons = options.buttons;
      this.renderButtons();
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