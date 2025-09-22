// 主题管理工具
class ThemeManager {
  constructor() {
    this.theme = this.getInitialTheme()
    this.applyTheme(this.theme)
  }

  // 获取初始主题
  getInitialTheme() {
    // 优先从localStorage获取用户设置
    const savedTheme = localStorage.getItem('theme')
    if (savedTheme && ['light', 'dark', 'auto'].includes(savedTheme)) {
      return savedTheme
    }

    // 默认跟随浏览器偏好
    return 'auto'
  }

  // 获取系统主题偏好
  getSystemTheme() {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }

  // 获取当前实际主题
  getCurrentTheme() {
    if (this.theme === 'auto') {
      return this.getSystemTheme()
    }
    return this.theme
  }

  // 应用主题
  applyTheme(theme) {
    const actualTheme = theme === 'auto' ? this.getSystemTheme() : theme
    
    // 移除之前的主题类
    document.documentElement.classList.remove('light-theme', 'dark-theme')
    
    // 添加新的主题类
    document.documentElement.classList.add(`${actualTheme}-theme`)
    
    // 设置data属性，方便CSS选择器使用
    document.documentElement.setAttribute('data-theme', actualTheme)
    
    this.theme = theme
    localStorage.setItem('theme', theme)
    
    // 触发主题变化事件
    window.dispatchEvent(new CustomEvent('themechange', {
      detail: { theme: actualTheme }
    }))
  }

  // 切换主题
  toggleTheme() {
    const themes = ['light', 'dark', 'auto']
    const currentIndex = themes.indexOf(this.theme)
    const nextTheme = themes[(currentIndex + 1) % themes.length]
    this.setTheme(nextTheme)
  }

  // 设置主题
  setTheme(theme) {
    if (['light', 'dark', 'auto'].includes(theme)) {
      this.applyTheme(theme)
    }
  }

  // 监听系统主题变化
  watchSystemTheme() {
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    mediaQuery.addEventListener('change', () => {
      if (this.theme === 'auto') {
        this.applyTheme('auto')
      }
    })
  }
}

// 创建全局实例
const themeManager = new ThemeManager()

// 监听系统主题变化
themeManager.watchSystemTheme()

// 添加Vue组件需要的方法
themeManager.onThemeChange = (callback) => {
  window.addEventListener('themechange', (event) => {
    callback(event.detail.theme)
  })
}

// 简化的切换方法（只在light和dark之间切换）
themeManager.toggle = () => {
  const currentTheme = themeManager.getCurrentTheme()
  const newTheme = currentTheme === 'dark' ? 'light' : 'dark'
  themeManager.setTheme(newTheme)
}

export { themeManager }
export default themeManager