<template>
  <nav class="navbar">
    <div class="navbar-content">
      <!-- Logo -->
      <div class="navbar-brand">
        <router-link to="/" class="brand-link">
          OneImg
        </router-link>
      </div>
      
      <!-- 桌面端导航链接 -->
      <ul class="nav-links desktop-nav">
        <li><router-link to="/" class="nav-link" @click="handleNavClick('/')">首页</router-link></li>
        <li><router-link to="/gallery" class="nav-link" @click="handleNavClick('/gallery')">画廊</router-link></li>
        <li><router-link to="/stats" class="nav-link" @click="handleNavClick('/stats')">统计</router-link></li>
        <li><router-link to="/settings" class="nav-link" @click="handleNavClick('/settings')">设置</router-link></li>
      </ul>
      
      <!-- 主题切换和用户菜单 -->
      <div class="navbar-actions">
        <!-- 移动端菜单按钮 -->
        <button @click="toggleMobileMenu" class="mobile-menu-btn">
          <span class="hamburger" :class="{ active: showMobileMenu }">
            <span></span>
            <span></span>
            <span></span>
          </span>
        </button>
        
        <!-- 主题切换按钮 -->
        <div class="theme-toggle-wrapper">
          <button @click="toggleTheme" class="theme-toggle" :title="themeTooltip">
            <span class="theme-icon">{{ themeIcon }}</span>
          </button>
        </div>
        
        <!-- 用户菜单 -->
        <div class="user-menu">
          <div class="user-info" @click="toggleUserMenu">
            <span class="username">{{ username || '用户' }}</span>
            <span class="dropdown-arrow">▼</span>
          </div>
          
          <!-- 下拉菜单 -->
          <div v-if="showUserMenu" class="dropdown-menu">
            <button @click="handleLogout" class="logout-btn">退出登录</button>
          </div>
        </div>
      </div>
    </div>
    
    <!-- 移动端导航菜单 -->
    <div v-if="showMobileMenu" class="mobile-nav" @click="closeMobileMenu">
      <div class="mobile-nav-content" @click.stop>
        <ul class="mobile-nav-links">
          <li><router-link to="/" class="mobile-nav-link" @click="handleMobileNavClick('/')">首页</router-link></li>
          <li><router-link to="/gallery" class="mobile-nav-link" @click="handleMobileNavClick('/gallery')">画廊</router-link></li>
          <li><router-link to="/stats" class="mobile-nav-link" @click="handleMobileNavClick('/stats')">统计</router-link></li>
          <li><router-link to="/settings" class="mobile-nav-link" @click="handleMobileNavClick('/settings')">设置</router-link></li>
        </ul>
      </div>
    </div>
  </nav>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import themeManager from '@/utils/theme.js'

const router = useRouter()
const username = ref('')
const showUserMenu = ref(false)
const showMobileMenu = ref(false)
const currentTheme = ref(themeManager.getCurrentTheme())

// 主题相关的计算属性
const themeIcon = computed(() => {
  return currentTheme.value === 'dark' ? '☀️' : '🌙'
})

const themeTooltip = computed(() => {
  return currentTheme.value === 'dark' ? '切换到浅色主题' : '切换到深色主题'
})

// 获取用户信息
const getUserInfo = () => {
  // 从localStorage或其他地方获取用户信息
  const userInfo = localStorage.getItem('userInfo')
  if (userInfo) {
    try {
      const user = JSON.parse(userInfo)
      username.value = user.username
    } catch (e) {
      console.error('解析用户信息失败:', e)
    }
  }
}

// 切换用户菜单
const toggleUserMenu = () => {
  showUserMenu.value = !showUserMenu.value
}

// 切换主题
const toggleTheme = () => {
  themeManager.toggle()
  currentTheme.value = themeManager.getCurrentTheme()
}

// 切换移动端菜单
const toggleMobileMenu = () => {
  showMobileMenu.value = !showMobileMenu.value
  // 关闭用户菜单
  showUserMenu.value = false
}

// 关闭移动端菜单
const closeMobileMenu = () => {
  showMobileMenu.value = false
}

// 处理移动端导航点击
const handleMobileNavClick = (path) => {
  handleNavClick(path)
  closeMobileMenu()
}

// 处理导航点击
const handleNavClick = (path) => {
  console.log('导航点击:', path)
  console.log('当前路由:', router.currentRoute.value.path)
  
  // 关闭用户菜单
  showUserMenu.value = false
  
  // 强制导航到指定路径
  if (router.currentRoute.value.path !== path) {
    router.push(path).then(() => {
      console.log('导航成功:', path)
    }).catch(err => {
      console.error('导航失败:', err)
    })
  }
}

// 处理退出登录
const handleLogout = async () => {
  try {
    // 调用后端退出接口
    await fetch('/api/logout', {
      method: 'POST'
    })
  } catch (error) {
    console.error('退出登录失败:', error)
  } finally {
    // 跳转到登录页
    router.push('/login')
  }
}

// 点击外部关闭菜单
const handleClickOutside = (event) => {
  if (!event.target.closest('.user-menu')) {
    showUserMenu.value = false
  }
  if (!event.target.closest('.navbar-actions') && !event.target.closest('.mobile-nav')) {
    showMobileMenu.value = false
  }
}

onMounted(() => {
  getUserInfo()
  document.addEventListener('click', handleClickOutside)
  
  // 监听主题变化
  themeManager.onThemeChange((theme) => {
    currentTheme.value = theme
  })
})
</script>

<style lang="scss" scoped>
.navbar {
  background: var(--bg-color);
  color: var(--text-color);
  padding: 1rem 0;
  box-shadow: 0 2px 10px var(--shadow-color);
  position: sticky;
  top: 0;
  z-index: 1000;
  border-bottom: 1px solid var(--border-color);
}

.navbar-content {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 20px;
}

.navbar-brand {
  .brand-link {
    color: var(--text-color);
    text-decoration: none;
    font-size: 1.5rem;
    font-weight: bold;
    display: flex;
    align-items: center;
    gap: 8px;
    transition: opacity 0.2s ease;
    
    &:hover {
      opacity: 0.9;
    }
  }
}

.nav-links {
  list-style: none;
  display: flex;
  gap: 2rem;
  margin: 0;
  padding: 0;
  
  .nav-link {
    color: var(--text-color);
    text-decoration: none;
    padding: 8px 16px;
    border-radius: 6px;
    transition: all 0.2s ease;
    
    &:hover {
      background: rgba(255, 255, 255, 0.15);
      transform: translateY(-1px);
    }
    
    &.router-link-active {
      background: rgba(255, 255, 255, 0.25);
      font-weight: 500;
    }
  }
}

.navbar-actions {
  display: flex;
  align-items: center;
  gap: 1.5rem;
  position: relative;
}

.theme-toggle-wrapper {
  position: relative;
  z-index: 1;
}

.mobile-menu-btn {
  display: none;
  background: none;
  border: none;
  cursor: pointer;
  padding: 8px;
  
  .hamburger {
    display: flex;
    flex-direction: column;
    width: 24px;
    height: 18px;
    position: relative;
    
    span {
      display: block;
      height: 2px;
      width: 100%;
      background: var(--text-color);
      border-radius: 1px;
      transition: all 0.3s ease;
      
      &:nth-child(1) {
        transform-origin: top left;
      }
      
      &:nth-child(2) {
        margin: 6px 0;
      }
      
      &:nth-child(3) {
        transform-origin: bottom left;
      }
    }
    
    &.active {
      span:nth-child(1) {
        transform: rotate(45deg) translate(2px, -2px);
      }
      
      span:nth-child(2) {
        opacity: 0;
        transform: translateX(-20px);
      }
      
      span:nth-child(3) {
        transform: rotate(-45deg) translate(2px, 2px);
      }
    }
  }
}

.mobile-nav {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 999;
  display: flex;
  justify-content: flex-end;
  animation: fadeIn 0.3s ease;
  
  .mobile-nav-content {
    background: var(--card-bg);
    width: 280px;
    height: 100%;
    padding: 80px 0 20px;
    box-shadow: -4px 0 20px var(--shadow-color);
    animation: slideInRight 0.3s ease;
    
    .mobile-nav-links {
      list-style: none;
      padding: 0;
      margin: 0;
      
      li {
        border-bottom: 1px solid var(--border-color);
        
        &:last-child {
          border-bottom: none;
        }
      }
      
      .mobile-nav-link {
        display: block;
        color: var(--text-color);
        text-decoration: none;
        padding: 16px 24px;
        font-size: 1.1rem;
        transition: all 0.2s ease;
        
        &:hover {
          background: var(--hover-bg);
          padding-left: 32px;
        }
        
        &.router-link-active {
          background: var(--accent-color);
          color: white;
          font-weight: 500;
        }
      }
    }
  }
}

.theme-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border: none;
  background: rgba(255, 255, 255, 0.15);
  border-radius: 50%;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
  z-index: 1;
  
  &:hover {
    background: rgba(255, 255, 255, 0.25);
    transform: scale(1.05);
  }
  
  .theme-icon {
    font-size: 1.2rem;
    transition: transform 0.2s ease;
  }
  
  &:active .theme-icon {
    transform: scale(0.95);
  }
}

.user-menu {
  position: relative;
  z-index: 2;
  user-select: none;
  
  .user-info {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    background: rgba(255, 255, 255, 0.15);
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s ease;
    color: var(--text-color);
    
    &:hover {
      background: rgba(255, 255, 255, 0.25);
    }
    
    .username {
      font-weight: 500;
      white-space: nowrap;
    }
    
    .dropdown-arrow {
      font-size: 0.8rem;
      transition: transform 0.2s ease;
    }
  }
  
  .dropdown-menu {
    position: absolute;
    top: 100%;
    right: 0;
    margin-top: 8px;
    background: var(--card-bg);
    border: 1px solid var(--border-color);
    border-radius: 6px;
    box-shadow: 0 4px 20px var(--shadow-color);
    min-width: 120px;
    overflow: hidden;
    animation: slideDown 0.2s ease;
    z-index: 1001;
    
    .logout-btn {
      width: 100%;
      padding: 12px 16px;
      border: none;
      background: none;
      color: var(--text-color);
      text-align: left;
      cursor: pointer;
      transition: background 0.2s ease;
      
      &:hover {
        background: var(--hover-bg);
      }
    }
  }
}

@keyframes slideDown {
  from {
    opacity: 0;
    transform: translateY(-10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes fadeIn {
  from {
    opacity: 0;
  }
  to {
    opacity: 1;
  }
}

@keyframes slideInRight {
  from {
    transform: translateX(100%);
  }
  to {
    transform: translateX(0);
  }
}

@media (max-width: 768px) {
  .navbar-content {
    padding: 0 15px;
  }
  
  .nav-links {
    gap: 1rem;
    
    .nav-link {
      padding: 6px 12px;
      font-size: 0.9rem;
    }
  }
  
  .navbar-brand .brand-link {
    font-size: 1.3rem;
  }
}

@media (max-width: 768px) {
  .mobile-menu-btn {
    display: block;
    order: -1;
  }
  
  .desktop-nav {
    display: none;
  }
  
  .navbar-actions {
    gap: 1rem;
  }
  
  .user-menu .user-info {
    padding: 6px 10px;
    
    .username {
      display: none;
    }
    
    .dropdown-arrow {
      margin-left: 0;
    }
  }
}

@media (max-width: 480px) {
  .navbar-content {
    padding: 0 12px;
  }
  
  .navbar-actions {
    gap: 0.75rem;
    
    .theme-toggle {
      width: 36px;
      height: 36px;
      
      .theme-icon {
        font-size: 1rem;
      }
    }
  }
  
  .navbar-brand .brand-link {
    font-size: 1.2rem;
  }
  
  .mobile-nav .mobile-nav-content {
    width: 100%;
    padding-top: 70px;
  }
}
</style>
    