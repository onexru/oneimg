<template>
  <teleport to="body">
    <div class="message-container">
      <transition-group name="message" tag="div">
        <div
          v-for="message in messages"
          :key="message.id"
          :class="['message', `message--${message.type}`]"
        >
          <div class="message-icon">
            <span v-if="message.type === 'success'">✓</span>
            <span v-else-if="message.type === 'error'">✕</span>
            <span v-else-if="message.type === 'warning'">⚠</span>
            <span v-else>ℹ</span>
          </div>
          <div class="message-content">{{ message.content }}</div>
          <button 
            class="message-close" 
            @click="removeMessage(message.id)"
          >
            ×
          </button>
        </div>
      </transition-group>
    </div>
  </teleport>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'

// 消息列表
const messages = ref([])

// 消息ID计数器
let messageId = 0

// 添加消息
const addMessage = (content, type = 'info', duration = 3000) => {
  const id = ++messageId
  const message = {
    id,
    content,
    type,
    duration
  }
  
  messages.value.push(message)
  
  // 自动移除消息
  if (duration > 0) {
    setTimeout(() => {
      removeMessage(id)
    }, duration)
  }
  
  return id
}

// 移除消息
const removeMessage = (id) => {
  const index = messages.value.findIndex(msg => msg.id === id)
  if (index > -1) {
    messages.value.splice(index, 1)
  }
}

// 清空所有消息
const clearMessages = () => {
  messages.value = []
}

// 暴露方法给外部使用
defineExpose({
  addMessage,
  removeMessage,
  clearMessages,
  success: (content, duration) => addMessage(content, 'success', duration),
  error: (content, duration) => addMessage(content, 'error', duration),
  warning: (content, duration) => addMessage(content, 'warning', duration),
  info: (content, duration) => addMessage(content, 'info', duration)
})

// 监听全局消息事件
const handleGlobalMessage = (event) => {
  const { content, type, duration } = event.detail
  addMessage(content, type, duration)
}

onMounted(() => {
  window.addEventListener('show-message', handleGlobalMessage)
})

onUnmounted(() => {
  window.removeEventListener('show-message', handleGlobalMessage)
})
</script>

<style lang="scss" scoped>
.message-container {
  position: fixed;
  top: 20px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 9999;
  pointer-events: none;
  width: 100%;
  max-width: 500px;
  padding: 0 20px;
}

.message {
  display: flex;
  align-items: center;
  background: white;
  border-radius: 8px;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
  margin-bottom: 12px;
  padding: 16px 20px;
  pointer-events: auto;
  min-height: 56px;
  border-left: 4px solid;
  
  &--success {
    border-left-color: #52c41a;
    
    .message-icon {
      color: #52c41a;
      background: rgba(82, 196, 26, 0.1);
    }
  }
  
  &--error {
    border-left-color: #ff4d4f;
    
    .message-icon {
      color: #ff4d4f;
      background: rgba(255, 77, 79, 0.1);
    }
  }
  
  &--warning {
    border-left-color: #faad14;
    
    .message-icon {
      color: #faad14;
      background: rgba(250, 173, 20, 0.1);
    }
  }
  
  &--info {
    border-left-color: #1890ff;
    
    .message-icon {
      color: #1890ff;
      background: rgba(24, 144, 255, 0.1);
    }
  }
}

.message-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  margin-right: 12px;
  font-size: 14px;
  font-weight: bold;
  flex-shrink: 0;
}

.message-content {
  flex: 1;
  color: #333;
  font-size: 14px;
  line-height: 1.4;
  word-break: break-word;
}

.message-close {
  background: none;
  border: none;
  color: #999;
  cursor: pointer;
  font-size: 18px;
  line-height: 1;
  margin-left: 12px;
  padding: 0;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  transition: all 0.2s ease;
  flex-shrink: 0;
  
  &:hover {
    background: #f5f5f5;
    color: #666;
  }
}

// 动画效果
.message-enter-active {
  transition: all 0.3s ease;
}

.message-leave-active {
  transition: all 0.3s ease;
}

.message-enter-from {
  opacity: 0;
  transform: translateY(-20px);
}

.message-leave-to {
  opacity: 0;
  transform: translateY(-20px);
}

.message-move {
  transition: transform 0.3s ease;
}

// 响应式设计
@media (max-width: 768px) {
  .message-container {
    padding: 0 15px;
    top: 15px;
  }
  
  .message {
    padding: 12px 16px;
    margin-bottom: 8px;
    
    .message-content {
      font-size: 13px;
    }
  }
}
</style>