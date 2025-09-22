import { createApp } from 'vue'
import App from './App.vue'
import router from './router/router.js'
import message from './utils/message.js'
import themeManager from './utils/theme.js'

// 主题管理器已经在导入时自动初始化

const app = createApp(App)

// 配置全局属性
app.config.globalProperties.$message = message
app.config.globalProperties.$theme = themeManager

// 提供全局注入
app.provide('$message', message)
app.provide('$theme', themeManager)

app.use(router)
app.mount('#app')