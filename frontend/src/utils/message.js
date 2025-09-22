// 消息提示工具类
class MessageService {
  constructor() {
    this.messageComponent = null
  }

  // 设置消息组件实例
  setMessageComponent(component) {
    this.messageComponent = component
  }

  // 显示消息的通用方法
  show(content, type = 'info', duration = 3000) {
    if (this.messageComponent) {
      return this.messageComponent.addMessage(content, type, duration)
    } else {
      // 如果组件还没有初始化，使用事件方式
      const event = new CustomEvent('show-message', {
        detail: { content, type, duration }
      })
      window.dispatchEvent(event)
    }
  }

  // 成功消息
  success(content, duration = 3000) {
    return this.show(content, 'success', duration)
  }

  // 错误消息
  error(content, duration = 3000) {
    return this.show(content, 'error', duration)
  }

  // 警告消息
  warning(content, duration = 3000) {
    return this.show(content, 'warning', duration)
  }

  // 信息消息
  info(content, duration = 3000) {
    return this.show(content, 'info', duration)
  }

  // 移除指定消息
  remove(id) {
    if (this.messageComponent) {
      this.messageComponent.removeMessage(id)
    }
  }

  // 清空所有消息
  clear() {
    if (this.messageComponent) {
      this.messageComponent.clearMessages()
    }
  }
}

// 创建全局实例
const message = new MessageService()

// 导出实例和类
export default message
export { MessageService }

// 为了兼容性，也可以挂载到window对象上
if (typeof window !== 'undefined') {
  window.$message = message
}