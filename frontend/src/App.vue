<template>
  <div id="app">
    <router-view />
    <!-- 全局消息组件 -->
    <Message ref="messageRef" />
    
    <!-- 主题切换按钮（可选，用于测试） -->
    <button 
      v-if="showThemeToggle" 
      @click="toggleTheme" 
      class="theme-toggle"
      :title="currentTheme === 'dark' ? '切换到浅色模式' : '切换到深色模式'"
    >
      <i :class="currentTheme === 'dark' ? 'fas fa-sun' : 'fas fa-moon'"></i>
    </button>
  </div>
</template>

<script setup>
import { ref, onMounted, inject } from 'vue'
import Message from '@/components/Message.vue'
import message from '@/utils/message.js'

const messageRef = ref(null)
const showThemeToggle = ref(false) // 设为true可显示主题切换按钮
const currentTheme = ref('light')

// 注入主题管理器
const $theme = inject('$theme')

const toggleTheme = () => {
  if ($theme) {
    $theme.toggle()
    currentTheme.value = $theme.getCurrentTheme()
  }
}

onMounted(() => {
  // 将消息组件实例注册到全局消息服务
  if (messageRef.value) {
    message.setMessageComponent(messageRef.value)
  }
  
  // 初始化当前主题状态
  if ($theme) {
    currentTheme.value = $theme.getCurrentTheme()
    
    // 监听主题变化
    $theme.onThemeChange((theme) => {
      currentTheme.value = theme
    })
  }
})
</script>

<style lang="scss">
@use "@/styles/style.scss" as *;
#app {
  min-height: 100vh;
  background-color: var(--bg-color);
  color: var(--text-color);
}

/* 路由过渡动画 */
.router-enter-active,
.router-leave-active {
  transition: opacity 0.3s ease;
}

.router-enter-from,
.router-leave-to {
  opacity: 0;
}
</style>