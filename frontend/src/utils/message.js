class Message {
  // 默认配置
  static defaults = {
    type: 'info', // 通知类型：success/info/warning/error
    message: '', // 通知内容（支持HTML）
    duration: 3000, // 自动关闭时长（毫秒），0表示不自动关闭
    position: 'top-right', // 显示位置：top-left/top-center/top-right/bottom-left/bottom-center/bottom-right
    offset:75, // 距离边界的偏移量（像素）
    zIndex: 99999, // 层级（高于普通元素，低于PopupModal）
    showClose: false, // 是否显示关闭按钮
    onClose: null // 关闭回调函数
  };

  // 存储当前所有通知实例
  static instances = [];

  // 样式定义（所有样式通过内联和动态添加）
  static styles = {
    // 核心样式
    base: {
      position: 'fixed',
      padding: '12px 16px',
      borderRadius: '8px',
      boxShadow: '0 4px 12px rgba(0, 0, 0, 0.1)',
      display: 'flex',
      alignItems: 'center',
      transition: 'all 0.3s ease',
      opacity: '0',
      transform: 'translateY(-10px)',
      maxWidth: '380px',
      minWidth: '160px',
      wordBreak: 'break-word',
      boxSizing: 'border-box'
    },
    // 显示状态
    show: {
      opacity: '1',
      transform: 'translateY(0)'
    },
    // 隐藏状态
    hide: {
      opacity: '0',
      transform: 'translateY(-10px)'
    },
    // 类型样式
    types: {
      success: {
        backgroundColor: '#f0f9eb'
      },
      info: {
        backgroundColor: '#e6f7ff'
      },
      warning: {
        backgroundColor: '#fffbe6'
      },
      error: {
        backgroundColor: '#fff1f0'
      }
    },
    // 暗黑模式类型样式
    darkTypes: {
      success: {
        backgroundColor: '#163320'
      },
      info: {
        backgroundColor: '#0f3443'
      },
      warning: {
        backgroundColor: '#443300'
      },
      error: {
        backgroundColor: '#4b1818'
      }
    },
    // 内容容器样式
    content: {
      display: 'flex',
      alignItems: 'center',
      width: '100%'
    },
    // 图标样式
    icon: {
      fontSize: '16px',
      marginRight: '8px',
      flexShrink: '0'
    },
    // 文本样式
    text: {
      flex: '1',
      fontSize: '14px',
      lineHeight: '1.5',
      color: '#1f2937'
    },
    // 暗黑模式文本样式
    darkText: {
      color: '#f3f4f6'
    },
    // 关闭按钮样式
    closeBtn: {
      background: 'transparent',
      border: 'none',
      color: '#9ca3af',
      cursor: 'pointer',
      fontSize: '14px',
      marginLeft: '8px',
      padding: '2px',
      borderRadius: '50%',
      transition: 'background-color 0.2s ease'
    },
    // 关闭按钮hover样式
    closeBtnHover: {
      backgroundColor: 'rgba(0, 0, 0, 0.05)',
      color: '#1f2937'
    },
    // 暗黑模式关闭按钮hover样式
    darkCloseBtnHover: {
      color: '#f3f4f6'
    }
  };

  // 图标映射（依赖Font Awesome）
  static iconMap = {
    success: '<i class="ri-check-line"></i>',
    info: '<i class="ri-info-i"></i>',
    warning: '<i class="ri-error-warning-line"></i>',
    error: '<i class="ri-close-line"></i>'
  };

  /**
   * 检查是否为暗黑模式
   * @returns {boolean} 是否暗黑模式
   */
  static isDarkMode() {
    return document.documentElement.classList.contains('dark');
  }

  /**
   * 应用样式到元素
   * @param {HTMLElement} el - 目标元素
   * @param {Object} styles - 样式对象
   */
  static applyStyles(el, styles) {
    Object.keys(styles).forEach(key => {
      el.style[key] = styles[key];
    });
  }

  /**
   * 显示通知
   * @param {Object|string} options - 配置项或直接传入通知内容
   * @returns {Object} 通知实例（包含close方法）
   */
  static show(options) {
    // 处理参数：如果是字符串，直接作为message
    const config = typeof options === 'string' 
      ? { ...this.defaults, message: options }
      : { ...this.defaults, ...options };

    // 验证必填参数
    if (!config.message) {
      console.warn('Message 通知内容不能为空');
      return null;
    }

    // 创建通知DOM
    const messageDom = this.createMessageDom(config);
    document.body.appendChild(messageDom);

    // 计算位置（处理多个通知的堆叠）
    this.calculatePosition(messageDom, config);

    // 显示动画
    setTimeout(() => {
      this.applyStyles(messageDom, this.styles.show);
      // 处理居中位置的transform
      const [, horizontal] = config.position.split('-');
      if (horizontal === 'center') {
        messageDom.style.transform = 'translateX(-50%) translateY(0)';
      }
    }, 10);

    // 自动关闭
    let timer = null;
    if (config.duration > 0) {
      timer = setTimeout(() => {
        this.close(messageDom, config);
      }, config.duration);
    }

    // 存储实例
    const instance = {
      dom: messageDom,
      config,
      close: () => this.close(messageDom, config),
      timer
    };
    this.instances.push(instance);

    return instance;
  }

  /**
   * 创建通知DOM元素
   * @param {Object} config - 配置项
   * @returns {HTMLElement} 通知DOM
   */
  static createMessageDom(config) {
    const isDark = this.isDarkMode();

    // 主容器
    const messageDom = document.createElement('div');
    messageDom.dataset.type = config.type;
    messageDom.dataset.position = config.position;
    messageDom.style.zIndex = config.zIndex;

    // 应用基础样式
    this.applyStyles(messageDom, this.styles.base);

    // 应用类型样式（根据是否暗黑模式）
    const typeStyles = isDark 
      ? this.styles.darkTypes[config.type]
      : this.styles.types[config.type];
    this.applyStyles(messageDom, typeStyles);

    // 内容容器
    const content = document.createElement('div');
    this.applyStyles(content, this.styles.content);

    // 图标
    const icon = document.createElement('div');
    this.applyStyles(icon, this.styles.icon);
    // 应用图标颜色（根据类型）
    const iconColor = {
      success: 'color: #52c41a',
      info: 'color: #1890ff',
      warning: 'color: #faad14',
      error: 'color: #ff4d4f'
    };
    icon.style.cssText += iconColor[config.type];
    icon.innerHTML = this.iconMap[config.type];
    content.appendChild(icon);

    // 文本
    const text = document.createElement('div');
    this.applyStyles(text, this.styles.text);
    // 暗黑模式文本样式
    if (isDark) {
      this.applyStyles(text, this.styles.darkText);
    }
    // 处理文本内容（支持HTML和DOM元素）
    if (typeof config.message === 'string') {
      text.innerHTML = config.message;
    } else if (config.message instanceof HTMLElement) {
      text.innerHTML = '';
      text.appendChild(config.message);
    }
    content.appendChild(text);

    // 关闭按钮
    if (config.showClose) {
      const closeBtn = document.createElement('button');
      this.applyStyles(closeBtn, this.styles.closeBtn);
      closeBtn.innerHTML = '<i class="ri-close-line"></i>';

      // 关闭按钮hover事件
      closeBtn.addEventListener('mouseenter', () => {
        const hoverStyles = isDark 
          ? this.styles.darkCloseBtnHover
          : this.styles.closeBtnHover;
        this.applyStyles(closeBtn, hoverStyles);
      });
      closeBtn.addEventListener('mouseleave', () => {
        this.applyStyles(closeBtn, this.styles.closeBtn);
      });

      // 关闭事件
      closeBtn.addEventListener('click', () => {
        this.close(messageDom, config);
      });

      content.appendChild(closeBtn);
    }

    messageDom.appendChild(content);

    // 鼠标悬停暂停自动关闭
    if (config.duration > 0) {
      messageDom.addEventListener('mouseenter', () => {
        const instance = this.instances.find(item => item.dom === messageDom);
        if (instance && instance.timer) {
          clearTimeout(instance.timer);
        }
      });

      messageDom.addEventListener('mouseleave', () => {
        const instance = this.instances.find(item => item.dom === messageDom);
        if (instance) {
          instance.timer = setTimeout(() => {
            this.close(messageDom, config);
          }, config.duration);
        }
      });
    }

    // 响应式调整（最大宽度）
    const mediaQuery = window.matchMedia('(max-width: 768px)');
    const handleResize = () => {
      if (mediaQuery.matches) {
        messageDom.style.maxWidth = 'calc(100% - 32px)';
      } else {
        messageDom.style.maxWidth = '380px';
      }
    };
    handleResize();
    mediaQuery.addEventListener('change', handleResize);

    // 存储媒体查询处理函数，便于后续移除
    messageDom.resizeHandler = handleResize;

    return messageDom;
  }

  /**
   * 计算通知位置（处理堆叠）
   * @param {HTMLElement} dom - 通知DOM
   * @param {Object} config - 配置项
   */
  static calculatePosition(dom, config) {
    const { position, offset } = config;
    const [vertical, horizontal] = position.split('-');

    // 基础位置样式
    dom.style[vertical] = `${offset}px`;

    // 水平位置
    switch (horizontal) {
      case 'left':
        dom.style.left = `${offset}px`;
        dom.style.transform = 'translateY(-10px)';
        break;
      case 'center':
        dom.style.left = '50%';
        dom.style.transform = 'translateX(-50%) translateY(-10px)';
        break;
      case 'right':
        dom.style.right = `${offset}px`;
        dom.style.transform = 'translateY(-10px)';
        break;
    }

    // 处理同位置的堆叠（上方/下方间距12px）
    const samePositionInstances = this.instances.filter(
      item => item.dom.dataset.position === position && item.dom !== dom
    );

    if (samePositionInstances.length > 0) {
      let totalHeight = 0;
      samePositionInstances.forEach(instance => {
        totalHeight += instance.dom.offsetHeight + 12;
      });

      dom.style[vertical] = `${offset + totalHeight}px`;
    }
  }

  /**
   * 关闭通知
   * @param {HTMLElement} dom - 通知DOM
   * @param {Object} config - 配置项
   */
  static close(dom, config) {
    // 移除自动关闭定时器
    const instanceIndex = this.instances.findIndex(item => item.dom === dom);
    if (instanceIndex !== -1) {
      const instance = this.instances[instanceIndex];
      if (instance.timer) clearTimeout(instance.timer);
      this.instances.splice(instanceIndex, 1);
    }

    // 移除响应式监听
    const mediaQuery = window.matchMedia('(max-width: 768px)');
    if (dom.resizeHandler) {
      mediaQuery.removeEventListener('change', dom.resizeHandler);
    }

    // 关闭动画
    this.applyStyles(dom, this.styles.hide);
    // 处理居中位置的transform
    const [, horizontal] = config.position.split('-');
    if (horizontal === 'center') {
      dom.style.transform = 'translateX(-50%) translateY(-10px)';
    }

    // 动画结束后移除DOM
    setTimeout(() => {
      if (dom.parentNode) dom.parentNode.removeChild(dom);
      // 执行关闭回调
      if (typeof config.onClose === 'function') {
        config.onClose();
      }
      // 重新计算同位置其他通知的位置
      this.reCalculatePositions(config.position);
    }, 300);
  }

  /**
   * 重新计算同位置通知的堆叠位置
   * @param {string} position - 位置类型
   */
  static reCalculatePositions(position) {
    const samePositionInstances = this.instances.filter(
      item => item.dom.dataset.position === position
    );

    let totalHeight = 0;
    const [vertical, horizontal] = position.split('-');
    const offset = this.defaults.offset;

    samePositionInstances.forEach((instance, index) => {
      const dom = instance.dom;
      const config = instance.config;

      // 水平位置保持不变
      if (horizontal === 'center') {
        dom.style.left = '50%';
        dom.style.transform = 'translateX(-50%) translateY(0)';
      } else {
        dom.style[horizontal] = `${offset}px`;
        dom.style.transform = 'translateY(0)';
      }

      // 垂直位置重新堆叠
      if (index === 0) {
        totalHeight = 0;
      } else {
        totalHeight += samePositionInstances[index - 1].dom.offsetHeight + 12;
      }

      dom.style[vertical] = `${offset + totalHeight}px`;
    });
  }

  /**
   * 关闭所有通知
   */
  static closeAll() {
    this.instances.forEach(instance => {
      if (instance.timer) clearTimeout(instance.timer);
      // 关闭动画
      this.applyStyles(instance.dom, this.styles.hide);
      const [, horizontal] = instance.config.position.split('-');
      if (horizontal === 'center') {
        instance.dom.style.transform = 'translateX(-50%) translateY(-10px)';
      }
      // 移除响应式监听
      const mediaQuery = window.matchMedia('(max-width: 768px)');
      if (instance.dom.resizeHandler) {
        mediaQuery.removeEventListener('change', instance.dom.resizeHandler);
      }
      // 移除DOM
      setTimeout(() => {
        if (instance.dom.parentNode) instance.dom.parentNode.removeChild(instance.dom);
      }, 300);
    });
    this.instances = [];
  }
}

// 快捷方法：成功通知
Message.success = function(message, options = {}) {
  return this.show({
    type: 'success',
    message,
    ...options
  });
};

// 快捷方法：信息通知
Message.info = function(message, options = {}) {
  return this.show({
    type: 'info',
    message,
    ...options
  });
};

// 快捷方法：警告通知
Message.warning = function(message, options = {}) {
  return this.show({
    type: 'warning',
    message,
    ...options
  });
};

// 快捷方法：错误通知
Message.error = function(message, options = {}) {
  return this.show({
    type: 'error',
    message,
    ...options
  });
};

// 暴露到全局
window.Message = Message;

// 兼容原有代码的showToast用法（可选）
if (!window.showToast) {
  window.showToast = function(message, type = 'success', duration = 2000) {
    Message.show({
      type,
      message,
      duration,
      position: 'top-center'
    });
  };
}

export default Message;