import { createApp } from 'vue'
import App from './App.vue'
import router from './router/router.js'
import './utils/message.js'
import './utils/popupModal.js';
import './utils/spotlight.bundle.js';
import './assets/main.css'

// 主题管理器已经在导入时自动初始化
const app = createApp(App)

app.use(router)
app.mount('#app')