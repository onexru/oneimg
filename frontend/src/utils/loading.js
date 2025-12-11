/**
 * 全局加载动画类
 * 支持静态调用、全局配置、多实例管理、自定义样式等
 */
class Loading {
  // 默认配置
  static defaults = {
    text: '加载中...', // 加载提示文本
    className: 'custom-loading', // 自定义类名
    mask: true, // 是否显示遮罩层
    color: '#1677ff', // 加载动画主色调
    fullscreen: true, // 是否全屏显示
    zIndex: 9999, // 层级
    container: document.body, // 默认挂载容器
    onShow: null, // 显示回调
    onHide: null // 隐藏回调
  };

  // 存储当前所有加载实例
  static instances = [];

  // 样式定义
  static styles = {
    // 基础容器样式
    base: {
      position: 'fixed',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      pointerEvents: 'none',
      opacity: '0',
      transition: 'opacity 0.2s ease',
      boxSizing: 'border-box'
    },
    // 全屏样式
    fullscreen: {
      top: '0',
      left: '0',
      right: '0',
      bottom: '0'
    },
    // 显示状态
    show: {
      opacity: '1'
    },
    // 遮罩层样式
    mask: {
      position: 'absolute',
      top: '0',
      left: '0',
      right: '0',
      bottom: '0',
      background: 'rgba(255, 255, 255, 0.8)',
      backdropFilter: 'blur(2px)',
      pointerEvents: 'auto'
    },
    // 暗黑模式遮罩
    darkMask: {
      background: 'rgba(0, 0, 0, 0.8)'
    },
    // 加载动画容器
    spinnerContainer: {
      position: 'relative',
      zIndex: '1',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: '12px',
      pointerEvents: 'none'
    },
    // 加载动画样式
    spinner: {
      width: '40px',
      height: '40px',
      border: '4px solid rgba(0, 0, 0, 0.1)',
      borderRadius: '50%',
      animation: 'loading-spin 1s linear infinite'
    },
    // 暗黑模式加载动画边框
    darkSpinner: {
      border: '4px solid rgba(255, 255, 255, 0.1)'
    },
    // 文本样式
    text: {
      fontSize: '14px',
      color: '#666',
      whiteSpace: 'nowrap',
      pointerEvents: 'none'
    },
    // 暗黑模式文本
    darkText: {
      color: '#ccc'
    }
  };

  /**
   * 重新计算同位置Loading的堆叠位置（补充缺失的方法）
   * @param {string} position - 位置类型（如top-right/bottom-center）
   */
  static reCalculatePositions(position) {
    // 仅处理非全屏的Loading实例（全屏不需要堆叠）
    const samePositionInstances = this.instances.filter(
      item => item.dom.dataset.position === position && item.config.fullscreen === false
    );

    if (samePositionInstances.length === 0) return;

    let totalHeight = 0;
    const [vertical, horizontal] = position.split('-');
    const offset = this.defaults.offset;

    // 重新计算每个实例的位置
    samePositionInstances.forEach((instance, index) => {
      const dom = instance.dom;
      if (!dom) return;

      // 水平位置保持不变
      if (horizontal === 'center') {
        dom.style.left = '50%';
        dom.style.transform = 'translateX(-50%) translateY(0)';
      } else {
        dom.style[horizontal] = `${offset}px`;
        dom.style.transform = 'translateY(0)';
      }

      // 垂直位置重新堆叠（间距12px）
      if (index === 0) {
        totalHeight = 0;
      } else {
        totalHeight += samePositionInstances[index - 1].dom.offsetHeight + 12;
      }

      dom.style[vertical] = `${offset + totalHeight}px`;
    });
  }

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
    if (!el) return;
    Object.keys(styles).forEach(key => {
        const cssKey = key.replace(/[A-Z]/g, match => `-${match.toLowerCase()}`);
        el.style[cssKey] = styles[key];
        el.style.setProperty(cssKey, styles[key]);
    });
  }

  /**
   * 添加全局动画样式
   * @private
   */
  static addGlobalStyle() {
    const styleId = 'loading-global-style';
    if (document.getElementById(styleId)) return;

    const style = document.createElement('style');
    style.id = styleId;
    style.textContent = `
      @keyframes loading-spin {
        0% { transform: rotate(0deg); }
        100% { transform: rotate(360deg); }
      }
    `;
    document.head.appendChild(style);
  }

  /**
   * 显示加载动画
   * @param {Object|string} options - 配置项或直接传入加载文本
   * @returns {Object} 加载实例（包含hide/destroy方法）
   */
  static show(options) {
    // 处理参数：如果是字符串，直接作为text
    const config = typeof options === 'string' 
      ? { ...this.defaults, text: options }
      : { ...this.defaults, ...options };

    // 添加全局样式
    this.addGlobalStyle();

    // 创建加载DOM
    const loadingDom = this.createLoadingDom(config);
    
    // 添加到容器
    config.container.appendChild(loadingDom);

    // 非全屏时设置容器为相对定位
    if (!config.fullscreen) {
      config.container.style.position = 'relative';
      loadingDom.style.position = 'absolute';
    }

    // 显示动画（延迟添加样式触发过渡）
    setTimeout(() => {
      this.applyStyles(loadingDom, this.styles.show);
      
      // 执行显示回调
      if (typeof config.onShow === 'function') {
        config.onShow();
      }
    }, 10);

    // 存储实例
    const instance = {
      dom: loadingDom,
      config,
      hide: () => this.hide(loadingDom),
      destroy: () => this.destroy(loadingDom),
      updateText: (text) => this.updateText(loadingDom, text),
      updateColor: (color) => this.updateColor(loadingDom, color)
    };
    
    // 移除同容器已存在的加载实例
    this.instances = this.instances.filter(item => {
      if (item.config.container === config.container && item.config.fullscreen === config.fullscreen) {
        item.hide();
        return false;
      }
      return true;
    });
    
    this.instances.push(instance);

    return instance;
  }

  /**
   * 创建加载动画DOM元素
   * @param {Object} config - 配置项
   * @returns {HTMLElement} 加载DOM
   */
  static createLoadingDom(config) {
    const isDark = this.isDarkMode();

    // 主容器
    const loadingDom = document.createElement('div');
    loadingDom.dataset.className = config.className;
    loadingDom.dataset.fullscreen = config.fullscreen;
    loadingDom.style.zIndex = config.zIndex;

    // 应用基础样式
    this.applyStyles(loadingDom, this.styles.base);
    
    // 全屏样式
    if (config.fullscreen) {
      this.applyStyles(loadingDom, this.styles.fullscreen);
    }

    // 创建遮罩层
    if (config.mask) {
      const maskEl = document.createElement('div');
      maskEl.className = `${config.className}-mask`;
      this.applyStyles(maskEl, this.styles.mask);
      
      // 暗黑模式遮罩
      if (isDark) {
        this.applyStyles(maskEl, this.styles.darkMask);
      }
      
      loadingDom.appendChild(maskEl);
    }

    // 创建加载动画容器
    const spinnerContainer = document.createElement('div');
    this.applyStyles(spinnerContainer, this.styles.spinnerContainer);

    // 创建加载动画
    const spinnerEl = document.createElement('div');
    spinnerEl.className = `${config.className}-spinner`;
    this.applyStyles(spinnerEl, this.styles.spinner);
    
    // 设置动画颜色
    spinnerEl.style.borderTopColor = config.color;
    
    // 暗黑模式加载动画
    if (isDark) {
      this.applyStyles(spinnerEl, this.styles.darkSpinner);
    }

    // 创建文本
    const textEl = document.createElement('div');
    textEl.className = `${config.className}-text`;
    textEl.dataset.for = config.className;
    this.applyStyles(textEl, this.styles.text);
    
    // 暗黑模式文本
    if (isDark) {
      this.applyStyles(textEl, this.styles.darkText);
    }
    
    textEl.textContent = config.text;

    // 组装DOM
    spinnerContainer.appendChild(spinnerEl);
    spinnerContainer.appendChild(textEl);
    loadingDom.appendChild(spinnerContainer);

    return loadingDom;
  }

  /**
     * 隐藏加载动画（修复版）
     * @param {HTMLElement} dom - 加载DOM
     * @param {number} [delay=0] - 延迟隐藏时间(ms)
     * @returns {Promise} 隐藏完成的Promise
     */
    static hide(dom, delay = 0) {
        return new Promise((resolve) => {
            // 前置校验：DOM不存在直接返回
            if (!dom || !dom.parentNode) {
            console.warn('加载动画DOM已不存在');
            resolve();
            return;
            }

            setTimeout(() => {
            // 1. 移除实例缓存
            const instanceIndex = this.instances.findIndex(item => item.dom === dom);
            if (instanceIndex !== -1) {
                const instance = this.instances[instanceIndex];
                if (instance.timer) clearTimeout(instance.timer);
                this.instances.splice(instanceIndex, 1);
            }

            // 2. 强制修改样式（不用applyStyles，直接操作）
            dom.style.opacity = '0';
            dom.style.transform = 'translateY(-10px)';
            dom.style.pointerEvents = 'none';

            // 处理居中位置的transform
            const [, horizontal] = dom.dataset.position?.split('-') || [];
            if (horizontal === 'center') {
                dom.style.transform = 'translateX(-50%) translateY(-10px)';
            }

            // 3. 动画结束后强制移除DOM
            setTimeout(() => {
                try {
                    dom.remove(); // 直接移除DOM
                } catch (err) {
                    if (dom.parentNode) {
                        dom.parentNode.removeChild(dom);
                    }
                }

                // 执行回调
                const config = this.instances.find(item => item.dom === dom)?.config;
                if (config && typeof config.onHide === 'function') {
                    config.onHide();
                }

                // 仅对非全屏实例重新计算位置（避免报错）
                if (this.reCalculatePositions && dom.dataset.position && !dom.dataset.fullscreen) {
                    this.reCalculatePositions(dom.dataset.position);
                }

                resolve();
            }, 200);
            }, delay);
        });
    }

  /**
   * 销毁加载实例
   * @param {HTMLElement} dom - 加载DOM
   */
  static destroy(dom) {
    // 立即隐藏并清理
    this.hide(dom, 0).then(() => {
      // 清理DOM引用
      dom = null;
    });
  }

  /**
   * 更新加载文本
   * @param {HTMLElement} dom - 加载DOM
   * @param {string} text - 新的加载文本
   */
  static updateText(dom, text) {
    if (!dom || !text) return;
    
    const textEl = dom.querySelector(`.${dom.dataset.className}-text`);
    if (textEl) {
      textEl.textContent = text;
      
      // 更新实例配置
      const instance = this.instances.find(item => item.dom === dom);
      if (instance) {
        instance.config.text = text;
      }
    }
  }

  /**
   * 更新加载动画颜色
   * @param {HTMLElement} dom - 加载DOM
   * @param {string} color - 新的颜色值
   */
  static updateColor(dom, color) {
    if (!dom || !color) return;
    
    const spinnerEl = dom.querySelector(`.${dom.dataset.className}-spinner`);
    if (spinnerEl) {
      spinnerEl.style.borderTopColor = color;
      
      // 更新实例配置
      const instance = this.instances.find(item => item.dom === dom);
      if (instance) {
        instance.config.color = color;
      }
    }
  }

  /**
   * 隐藏所有加载动画
   * @returns {Promise} 所有加载动画隐藏完成的Promise
   */
  static async hideAll() {
    const hidePromises = this.instances.map(instance => {
      return this.hide(instance.dom);
    });
    
    await Promise.all(hidePromises);
      this.instances = [];
  }

  /**
   * 获取指定容器的加载实例
   * @param {HTMLElement} container - 容器元素
   * @returns {Object|null} 加载实例
   */
  static getInstance(container = document.body) {
    return this.instances.find(instance => {
      return instance.config.container === container;
    }) || null;
  }
}

// 快捷方法：创建全屏加载
Loading.fullscreen = function(options) {
  return this.show({
    ...(typeof options === 'string' ? { text: options } : options),
    fullscreen: true
  });
};

// 快捷方法：创建局部加载
Loading.local = function(container, options) {
  return this.show({
    ...(typeof options === 'string' ? { text: options } : options),
    fullscreen: false,
    container: container
  });
};

// 暴露到全局
window.Loading = Loading;

export default Loading;
