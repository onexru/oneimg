<template>
  <!-- 顶部导航栏 -->
  <header class="bg-light-100/80 dark:bg-dark-300/80 backdrop-blur-md border-b border-light-200 dark:border-dark-100 py-3 fixed top-0 left-0 right-0 z-40 transition-all duration-300 md:ml-[255px]">
    <div class="container mx-auto px-4">
      <div class="flex justify-between items-center">
        <!-- 左侧Logo和菜单按钮 -->
        <div class="flex items-center gap-3">
          <button 
            ref="sidebarToggleRef"
            class="md:hidden w-10 h-10 rounded-md bg-light-200 dark:bg-dark-100 text-secondary hover:bg-light-300 dark:hover:bg-dark-200 transition-all duration-200 flex items-center justify-center"
          >
            <i class="ri-align-justify"></i>
          </button>
          <div class="flex items-center gap-2 font-semibold text-xl">
            <div class="w-10 h-10 rounded-md bg-gradient-to-br from-primary to-primary-dark flex items-center justify-center text-white font-bold">{{getFirstWord(seoTitle)}}</div>
            <span>{{ seoTitle }}</span>
          </div>
        </div>
        
        <!-- 右侧操作区 -->
        <div class="flex items-center gap-4">
          <button 
            ref="themeToggleRef"
            class="w-10 h-10 rounded-md bg-light-200 dark:bg-dark-100 text-secondary hover:bg-light-300 dark:hover:bg-dark-200 hover:text-primary transition-all duration-200 flex items-center justify-center"
          >
            <i class="ri-moon-clear-line dark:hidden"></i>
            <i class="ri-sun-line dark:inline-block hidden"></i>
          </button>

          <button 
            v-if="isLogin"
            @click="handleLogout"
            class="w-10 h-10 rounded-md bg-light-200 dark:bg-dark-100 text-secondary hover:bg-light-300 dark:hover:bg-dark-200 hover:text-primary transition-all duration-200 flex items-center justify-center"
          >
            <i class="ri-logout-circle-r-line"></i>
          </button>
        </div>
      </div>
    </div>
  </header>

  <!-- 侧边栏 -->
  <div 
    ref="sidebarRef"
    class="fixed top-0 left-0 h-full w-64 bg-light-100 transition-all dark:bg-dark-300 border-r border-light-200 dark:border-dark-100 z-50 transition-transform duration-300 sidebar-closed md:sidebar-open"
  >
    <div class="p-5 border-b border-light-200 transition-all dark:border-dark-100">
        <h3 class="font-medium text-secondary">导航菜单</h3>
    </div>
    <nav class="p-2">
        <ul class="space-y-1">
          <li v-for="item in navItems" :key="item.path">
            <router-link
              :to="item.path"
              :class="[
                'flex items-center px-3 py-3 rounded-md transition-all duration-200',
                isRouteActive(item.path) ? 'bg-primary/10 text-primary' : 'hover:bg-light-100 dark:hover:bg-dark-300 text-secondary hover:text-primary transition-all'
              ]"
              @click="handleNavClick"
            >
              <i :class="`ri-${item.icon} w-6 text-center`"></i>
              <span class="ml-3">{{ item.name }}</span>
            </router-link>
          </li>
        </ul>
    </nav>
  </div>

  <!-- 侧边栏遮罩 -->
  <div
    ref="sidebarOverlayRef"
    class="fixed inset-0 bg-black/50 z-40 overlay-hidden transition-opacity duration-300 pt-16"
  ></div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'

const router = useRouter()
const route = useRoute()

// 定义 ref 引用
const themeToggleRef = ref(null)
const sidebarToggleRef = ref(null)
const sidebarRef = ref(null)
const sidebarOverlayRef = ref(null)
const seoTitle = ref('初春图床');
const isLogin = ref(false);
// 导航菜单数据
const navItems = ref([]);
const refreshNavItems = () => {
  const userInfo = JSON.parse(localStorage.getItem('userInfo') || '{}');
  navItems.value.splice(0);
  isLogin.value = !!userInfo.username;

  if (!isLogin.value) {
    navItems.value.push({ path: '/login', icon: 'login-circle-line', name: '登录' });
  } else {
    navItems.value.push(
      { path: '/', icon: 'home-line', name: '首页' },
      { path: '/gallery', icon: 'nft-line', name: '画廊' },
      { path: '/tags', icon: 'bookmark-line', name: 'Tags' },
      { path: '/stats', icon: 'numbers-fill', name: '统计' }
    );
    if (userInfo?.isTourist !== true) {
      navItems.value.push(
        { path: '/buckets', icon: 'database-2-line', name: '存储桶'},
        { path: '/account', icon: 'user-settings-line', name: '账户设置' },
        { path: '/settings', icon: 'settings-line', name: '系统设置' }
      );
    }
  }
};

const isRouteActive = (targetPath) => {
  const exactMatchPaths = ['/', '/login', '/404']
  if (exactMatchPaths.includes(targetPath)) {
    return route.path === targetPath
  }
  return route.path.startsWith(targetPath)
}

// 导航点击事件
const handleNavClick = () => {
  if (window.innerWidth < 768) {
    closeSidebar()
  }
}

// 主题切换功能
const storageKey = 'theme-preference'

const detectUserThemePreference = () => {
  if (typeof localStorage !== 'undefined' && localStorage.getItem(storageKey)) {
    return localStorage.getItem(storageKey)
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

const applyTheme = (theme) => {
  const htmlElement = document.documentElement
  if (theme === 'dark') {
    htmlElement.classList.add('dark')
  } else {
    htmlElement.classList.remove('dark')
  }
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(storageKey, theme)
  }
}

// 侧边栏控制功能
const openSidebar = () => {
  if (sidebarRef.value) {
    sidebarRef.value.classList.remove('sidebar-closed')
    sidebarRef.value.classList.add('sidebar-open')
  }
  if (sidebarOverlayRef.value) {
    if (window.innerWidth < 768) {
      sidebarOverlayRef.value.classList.remove('overlay-hidden')
      sidebarOverlayRef.value.classList.add('overlay-visible')
    }
  }
  if (window.innerWidth < 768) {
    document.body.style.overflow = 'hidden'
  }
}

const closeSidebar = () => {
  if (sidebarRef.value) {
    sidebarRef.value.classList.remove('sidebar-open')
    sidebarRef.value.classList.add('sidebar-closed')
  }
  if (sidebarOverlayRef.value) {
    sidebarOverlayRef.value.classList.remove('overlay-visible')
    sidebarOverlayRef.value.classList.add('overlay-hidden')
  }
  document.body.style.overflow = ''
}

const handleLogout = async () => {
  if (typeof localStorage !== 'undefined') {
    localStorage.removeItem('token')
    localStorage.removeItem('userInfo')
  }
  try{
    await fetch('/api/logout', { method: 'POST' })
    Message.success('登出成功')
    setTimeout(() => {
      refreshNavItems();
      router.push('/login').catch(err => {
        console.log('跳转登录页失败：', err)
      })
    }, 500)
  } catch (error) {
    Message.error('登出失败')
  }
}

// 获取标题第一个字
const getFirstWord = (title) => {
  if (!title) return ''
  return title.split('')[0]
}

const handleSeoUpdate = (data) => {
  if (data?.seo_title) {
    seoTitle.value = data.seo_title;
  }
};

// 组件挂载时初始化
onMounted(() => {
  // 初始化主题
  const initialTheme = detectUserThemePreference()
  applyTheme(initialTheme)

  // 绑定 SEO 更新事件
  window.seoBus?.onUpdate(handleSeoUpdate);
  if (window.seoStting?.seo_title) {
    seoTitle.value = window.seoStting.seo_title;
  }

  // 绑定主题切换事件
  if (themeToggleRef.value) {
    themeToggleRef.value.addEventListener('click', () => {
      const currentTheme = localStorage.getItem(storageKey) || 'light'
      const newTheme = currentTheme === 'dark' ? 'light' : 'dark'
      applyTheme(newTheme)
    })
  }

  // 绑定侧边栏打开事件
  if (sidebarToggleRef.value) {
    sidebarToggleRef.value.addEventListener('click', openSidebar)
  }

  // 绑定侧边栏遮罩关闭事件
  if (sidebarOverlayRef.value) {
    sidebarOverlayRef.value.addEventListener('click', closeSidebar)
  }

  // 窗口大小变化事件
  const handleResize = () => {
    if (window.innerWidth <= 768) {
      closeSidebar()
    }
    if (window.innerWidth >= 768) {
      openSidebar()
    }
  }
  refreshNavItems();
  window.refreshNavItems = refreshNavItems;
  window.addEventListener('resize', handleResize)
})

// 组件卸载时清理
onUnmounted(() => {
  // 移除主题切换事件
  if (themeToggleRef.value) {
    themeToggleRef.value.removeEventListener('click', () => {})
  }

  // 移除侧边栏打开事件
  if (sidebarToggleRef.value) {
    sidebarToggleRef.value.removeEventListener('click', openSidebar)
  }

  // 移除侧边栏遮罩关闭事件
  if (sidebarOverlayRef.value) {
    sidebarOverlayRef.value.removeEventListener('click', closeSidebar)
  }

  // 移除SEO更新事件
  window.seoBus.callbacks = window.seoBus.callbacks.filter(cb => cb !== handleSeoUpdate);

  // 移除窗口 resize 事件
  window.removeEventListener('resize', () => {});

  // 恢复页面滚动
  document.body.style.overflow = '';

  delete window.refreshNavItems;
})

// 初始化侧边栏状态
onMounted(() => {
  // 加载SEO标题
  if (window.seoStting?.seo_title) {
    seoTitle.value = window.seoStting.seo_title;
  }
})
</script>

<style scoped>
/* 侧边栏滚动样式 */
::v-deep(.sidebar-open) {
  overflow-y: auto;
}

::v-deep(.sidebar-open)::-webkit-scrollbar {
  width: 4px;
}
::v-deep(.sidebar-open)::-webkit-scrollbar-thumb {
  background: #cbd5e1;
  border-radius: 2px;
}
::v-deep(.sidebar-open)::-webkit-scrollbar-thumb:hover {
  background: #94a3b8;
}
</style>