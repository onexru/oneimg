/**
 * 前端入口：挂载 Vue 应用与全局工具。
 */
import { createApp } from 'vue'
import App from './App.vue'
import router from './router/router.js'
import './utils/message.js'
import './utils/popupModal.js'
import './utils/spotlight.bundle.js'
import './utils/loading.js'
import './utils/guestFingerprint.js'
import './assets/main.css'

const app = createApp(App)
app.use(router)
app.mount('#app')