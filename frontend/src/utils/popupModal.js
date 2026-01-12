class PopupModal {
  constructor(options = {}) {
    // 默认配置
    this.defaults = {
      // 基础配置
      id: `modal-${Date.now()}`,
      title: '提示',
      content: '',
      type: 'default', // default/confirm/form
      width: 'auto', // sm(300px)/md(500px)/lg(700px)/full(90%)
      showClose: true,
      mask: true,
      maskClose: true, // 默认允许遮罩关闭
      zIndex: 9999,

      // 按钮配置（默认2个按钮：取消/确认）
      buttons: [
        {
          text: '取消',
          type: 'default',
          callback: (modal) => modal.close()
        },
        {
          text: '确认',
          type: 'primary',
          callback: null
        }
      ],

      // 表单配置（type=form时生效）
      formFields: [],
      formSubmit: null,

      // 生命周期
      onOpen: null,
      onClose: null
    };

    // 合并配置（确保maskClose默认值生效）
    this.config = { ...this.defaults, ...options };
    
    // 状态管理
    this.state = {
      isOpen: false,
      formData: {}
    };

    // 初始化表单数据
    if (this.config.type === 'form' && this.config.formFields.length) {
      this.config.formFields.forEach(field => {
        this.state.formData[field.name] = field.defaultValue || '';
      });
    }

    // 创建DOM元素
    this.createElements();
    // 单独绑定遮罩事件（确保优先级）
    this.bindMaskEvent();
  }

  /**
   * 创建弹出框DOM结构
   */
  createElements() {
    // 宽度映射
    const widthMap = {
      sm: 'w-[300px]',
      md: 'w-[500px]',
      lg: 'w-[700px]',
      full: 'w-[90%]',
      auto: ['w-[calc(100%-20px)]','min-w-[320px]','max-w-[500px]']
    };

    // 1. 创建遮罩层
    this.mask = document.createElement('div');
    this.mask.id = `${this.config.id}-mask`;
    // 修复：遮罩初始状态添加pointer-events-none，避免遮挡页面
    this.mask.className = `fixed inset-0 bg-black/50 dark:bg-black/70 backdrop-blur-sm transition-opacity duration-300 opacity-0 pointer-events-none`;
    this.mask.style.zIndex = this.config.zIndex - 1;
    document.body.appendChild(this.mask);

    // 2. 创建弹出框容器
    this.modal = document.createElement('div');
    this.modal.id = this.config.id;
    this.modal.className = `fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 scale-95 opacity-0 transition-all duration-300 pointer-events-none rounded-xl bg-white dark:bg-dark-200 shadow-lg dark:shadow-dark-lg overflow-hidden`;
    this.modal.style.zIndex = this.config.zIndex;
    this.modal.classList.add(...(widthMap[this.config.width] || widthMap.auto));

    // 3. 头部（标题+关闭按钮）
    this.header = document.createElement('div');
    this.header.className = 'px-6 py-4 border-b border-light-200 dark:border-dark-100 flex justify-between items-center';
    
    // 标题
    this.titleEl = document.createElement('h3');
    this.titleEl.className = 'font-semibold text-lg text-dark-300 dark:text-light-100 w-[50%] truncate';
    this.titleEl.innerHTML = this.config.title;
    this.header.appendChild(this.titleEl);

    // 关闭按钮
    if (this.config.showClose) {
      this.closeBtn = document.createElement('button');
      this.closeBtn.className = 'w-8 h-8 flex items-center justify-center text-secondary hover:text-danger transition-colors';
      this.closeBtn.innerHTML = '<i class="ri-close-fill font-bold text-[1.35rem]"></i>';
      this.closeBtn.addEventListener('click', () => this.close());
      this.header.appendChild(this.closeBtn);
    }
    this.modal.appendChild(this.header);

    // 4. 内容区
    this.content = document.createElement('div');
    this.content.className = 'px-6 py-5 max-h-[60vh] overflow-y-auto';
    
    // 根据类型渲染内容
    if (this.config.type === 'form') {
      this.renderFormContent();
    } else {
      this.content.innerHTML = this.config.content;
    }
    this.modal.appendChild(this.content);

    // 5. 底部（按钮区）
    this.footer = document.createElement('div');
    this.footer.className = 'px-6 py-4 border-t border-light-200 dark:border-dark-100 flex justify-end gap-3';
    this.renderButtons();
    this.modal.appendChild(this.footer);

    // 添加到页面
    document.body.appendChild(this.modal);
  }

  /**
   * 单独绑定遮罩事件
   */
  bindMaskEvent() {
    // 确保mask和maskClose都为true时才绑定
    if (this.config.mask && this.config.maskClose) {
      // 使用箭头函数确保this指向正确
      this.maskClickHandler = () => {
        // 只有弹窗处于打开状态时才执行关闭
        if (this.state.isOpen) {
          this.close();
        }
      };
      // 绑定事件（使用addEventListener确保可移除）
      this.mask.addEventListener('click', this.maskClickHandler);
    }
  }

  /**
   * 渲染表单内容（type=form时）
   */
  renderFormContent() {
    this.content.innerHTML = '';
    const form = document.createElement('form');
    form.className = 'space-y-4';
    form.addEventListener('submit', (e) => {
      e.preventDefault();
      this.handleFormSubmit();
    });

    // 渲染表单项
    this.config.formFields.forEach(field => {
      const fieldGroup = document.createElement('div');
      fieldGroup.className = 'space-y-2';

      // 标签
      const label = document.createElement('label');
      label.className = 'block text-sm font-medium text-dark-300 dark:text-light-100';
      label.textContent = field.label;
      if (field.required) label.innerHTML += '<span class="text-danger ml-1">*</span>';
      fieldGroup.appendChild(label);

      // 输入框（支持多种类型）
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
          
          // 添加选项
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
        
        default: // input类型（text/number/email等）
          input = document.createElement('input');
          input.type = field.type || 'text';
          input.className = 'w-full px-3 py-2 border border-light-200 dark:border-dark-100 rounded-md bg-light-100 dark:bg-dark-300 focus:outline-none focus:ring-1 focus:ring-primary focus:border-primary';
          input.placeholder = field.placeholder || '';
          break;
      }

      // 基础属性
      input.name = field.name;
      input.value = this.state.formData[field.name] || '';
      if (field.required) input.required = true;
      if (field.disabled) input.disabled = true;

      // 绑定值变化事件
      input.addEventListener('change', (e) => {
        this.state.formData[field.name] = e.target.value;
        // 返回事件
        if (field.onChange) {
          field.onChange(this, e.target.value);
        }
      });

      fieldGroup.appendChild(input);

      // 提示文本
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

  /**
   * 渲染底部按钮
   */
  renderButtons() {
    this.footer.innerHTML = '';
    this.config.buttons.forEach((btn, index) => {
      const button = document.createElement('button');
      
      // 按钮样式（根据类型）
      const btnStyles = {
        default: 'px-4 py-2 border border-light-200 dark:border-dark-100 rounded-md text-dark-300 dark:text-light-100 bg-white dark:bg-dark-200 hover:bg-light-100 dark:hover:bg-dark-300 transition-colors',
        primary: 'px-4 py-2 rounded-md text-white bg-primary hover:bg-primary-dark transition-colors',
        danger: 'px-4 py-2 rounded-md text-white bg-danger hover:bg-danger/90 transition-colors'
      };
      button.className = btnStyles[btn.type] || btnStyles.default;
      button.textContent = btn.text;

      // 绑定点击事件
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

  /**
   * 处理表单提交（type=form时）
   */
  handleFormSubmit() {
    if (typeof this.config.formSubmit === 'function') {
      this.config.formSubmit(this, this.state.formData);
    } else {
      this.close();
    }
  }

  /**
   * 打开弹出框
   */
  open() {
    if (this.state.isOpen) return;
    
    // 显示遮罩（先移除pointer-events-none，再显示）
    if (this.config.mask) {
      this.mask.classList.remove('pointer-events-none');
      setTimeout(() => {
        this.mask.classList.remove('opacity-0');
        this.mask.classList.add('opacity-100');
      }, 10);
    }
    
    // 显示弹窗
    this.modal.classList.remove('pointer-events-none');
    setTimeout(() => {
      this.modal.classList.remove('scale-95', 'opacity-0');
      this.modal.classList.add('scale-100', 'opacity-100');
    }, 10);
    
    // 更新状态
    this.state.isOpen = true;
    
    // 执行打开回调
    if (typeof this.config.onOpen === 'function') {
      this.config.onOpen(this);
    }

    // 禁止页面滚动
    document.body.style.overflow = 'hidden';
  }

  /**
   * 关闭弹出框
   */
  close() {
    if (!this.state.isOpen) return;
    
    // 隐藏遮罩（先隐藏，再添加pointer-events-none）
    if (this.config.mask) {
      this.mask.classList.remove('opacity-100');
      this.mask.classList.add('opacity-0');
      setTimeout(() => {
        this.mask.classList.add('pointer-events-none');
        this.mask.remove();
      }, 300);
    }
    
    // 隐藏弹窗
    this.modal.classList.remove('scale-100', 'opacity-100');
    this.modal.classList.add('scale-95', 'opacity-0');
    setTimeout(() => {
      this.modal.classList.add('pointer-events-none');
      this.modal.remove();
    }, 300);

    // 更新状态
    this.state.isOpen = false;
    
    // 执行关闭回调
    if (typeof this.config.onClose === 'function') {
      this.config.onClose(this);
    }

    // 恢复页面滚动
    document.body.style.overflow = '';
  }

  /**
   * 更新弹出框内容
   * @param {Object} options - 要更新的配置（title/content/buttons等）
   */
  update(options) {
    if (options.title) {
      this.config.title = options.title;
      this.titleEl.textContent = options.title;
    }

    if (options.content) {
      this.config.content = options.content;
      if (this.config.type !== 'form') {
        this.content.innerHTML = options.content;
      }
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

  /**
   * 追加表单字段
   * @param {Array} newFields - 要追加的新字段数组
   * @param {Array} keepFieldNames - 要保留的原有字段名数组
   */
  appendFormFields(newFields = [], keepFieldNames = []) {
    // 仅在表单类型下生效
    if (this.config.type !== 'form') {
      return;
    }

    // 过滤要保留的原有字段
    const keepFields = this.config.formFields.filter(field => {
      return keepFieldNames.includes(field.name);
    });

    // 回填已输入的数据到新字段
    const filledNewFields = newFields.map(field => ({
      ...field,
      defaultValue: this.state.formData[field.name] || field.value || ''
    }));

    // 拼接新的字段数组
    const finalFields = [...keepFields, ...filledNewFields];

    // 更新表单配置并重新渲染
    this.update({ formFields: finalFields });

    // 同步更新表单数据
    filledNewFields.forEach(field => {
      if (!this.state.formData[field.name]) {
        this.state.formData[field.name] = field.value || '';
      }
    });
  }

  /**
   * 销毁弹出框（从DOM中移除）
   */
  destroy() {
    // 移除遮罩事件，避免内存泄漏
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

// 暴露到全局，方便直接调用
window.PopupModal = PopupModal;

// 快捷方法：普通提示框
window.showAlert = function(content, title = '提示', callback) {
  const modal = new PopupModal({
    title,
    content,
    type: 'default',
    buttons: [
      {
        text: '确定',
        type: 'primary',
        callback: (modal) => {
          modal.close();
          if (typeof callback === 'function') callback();
        }
      }
    ]
  });
  modal.open();
  return modal;
};

// 快捷方法：确认提示框
window.showConfirm = function(content, title = '确认', confirmCallback, cancelCallback) {
  const modal = new PopupModal({
    title,
    content,
    type: 'confirm',
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => {
          modal.close();
          if (typeof cancelCallback === 'function') cancelCallback();
        }
      },
      {
        text: '确认',
        type: 'primary',
        callback: (modal) => {
          modal.close();
          if (typeof confirmCallback === 'function') confirmCallback();
        }
      }
    ]
  });
  modal.open();
  return modal;
};

// 快捷方法：表单弹出框
window.showFormModal = function(options) {
  const modal = new PopupModal({
    type: 'form',
    ...options
  });
  modal.open();
  return modal;
};

export default PopupModal;

window.PopupModal = PopupModal;