<template>
  <header class="fixed inset-x-0 top-0 z-40 border-b border-slate-200/80 bg-white/95 backdrop-blur-sm dark:border-white/10 dark:bg-slate-950/95 lg:left-[var(--app-sidebar-width)]">
    <div class="mx-auto flex h-[var(--app-header-height-mobile)] max-w-[1440px] items-center justify-between gap-2 px-2.5 sm:px-4 md:h-[var(--app-header-height)] md:px-5 xl:px-6 2xl:px-8">
      <div class="flex min-w-0 flex-1 items-center gap-2 sm:gap-2.5">
        <button
          type="button"
          class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-slate-200 bg-white text-slate-700 transition hover:border-slate-300 hover:text-slate-900 dark:border-white/10 dark:bg-slate-900 dark:text-slate-200 dark:hover:border-white/20 dark:hover:text-white lg:hidden"
          @click="toggleSidebar"
        >
          <i class="ri-menu-3-line text-lg"></i>
        </button>
        <div class="flex min-w-0 items-center gap-2 sm:gap-2.5">
          <div class="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-slate-900 text-sm font-bold text-white shadow-sm dark:bg-white dark:text-slate-900 sm:h-9 sm:w-9 sm:text-base">
              {{ getFirstWord(seoTitle) }}
          </div>
          <div class="min-w-0">
            <h1 class="truncate text-[13px] font-semibold text-slate-900 dark:text-slate-100 sm:text-[15px] md:text-base xl:text-lg">{{ seoTitle }}</h1>
          </div>
        </div>
      </div>

      <div class="flex shrink-0 items-center gap-1.5 md:gap-2">
        <button
          type="button"
          class="inline-flex h-8.5 w-8.5 items-center justify-center rounded-xl border border-slate-200 bg-white text-sm font-medium text-slate-600 transition hover:border-slate-300 hover:text-slate-900 dark:border-white/10 dark:bg-slate-900 dark:text-slate-200 dark:hover:border-white/20 dark:hover:text-white md:h-9 md:w-auto md:gap-1.5 px-3 py-1.5"
          @click="toggleTheme"
        >
          <i :class="isDark ? 'ri-sun-line' : 'ri-moon-clear-line'"></i>
          <span class="hidden md:inline">{{ isDark ? '浅色' : '深色' }}</span>
        </button>

        <button
          v-if="isLogin"
          type="button"
          class="inline-flex h-8.5 w-8.5 items-center justify-center rounded-xl border border-red-200 bg-white text-sm font-medium text-red-600 transition hover:border-red-300 hover:text-red-900 dark:border-white/10 dark:bg-red-900 dark:text-red-200 dark:hover:border-red/20 dark:hover:text-red md:h-9 md:w-auto md:gap-1.5 px-3 py-1.5"
          @click="handleLogout"
        >
          <i class="ri-logout-circle-r-line"></i>
        </button>
      </div>
    </div>
  </header>

  <aside class="fixed inset-y-0 left-0 z-50 w-[min(88vw,var(--app-sidebar-width))] border-r border-slate-200/80 bg-slate-100 px-2 pb-2 pt-2 transition-transform duration-300 dark:border-white/10 dark:bg-slate-950 sm:px-3 sm:pb-3 sm:pt-3 lg:w-[var(--app-sidebar-width)]" :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'">
    <div class="flex h-full flex-col overflow-hidden rounded-[18px] border border-slate-200/80 bg-white px-3 py-3 sm:rounded-[22px] sm:px-3.5 sm:py-4 dark:border-white/10 dark:bg-slate-900">
      <div class="flex items-center justify-between border-b border-slate-200/70 pb-3 dark:border-white/10 sm:pb-3.5">
        <div>
          <p class="text-xs font-medium uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">Navigation</p>
          <h2 class="mt-1.5 text-base font-semibold text-slate-900 dark:text-slate-100 sm:text-lg">工作区</h2>
        </div>
        <button
          type="button"
          class="inline-flex h-9 w-9 items-center justify-center rounded-xl border border-slate-200 bg-white text-slate-600 transition hover:text-slate-900 dark:border-white/10 dark:bg-slate-900 dark:text-slate-300 dark:hover:text-white lg:hidden"
          @click="closeSidebar"
        >
          <i class="ri-close-line"></i>
        </button>
      </div>

      <nav class="mt-3 flex-1 overflow-y-auto pr-1 sm:mt-4">
        <ul class="space-y-1.5">
          <li v-for="item in navItems" :key="item.path">
            <router-link
              :to="item.path"
              class="group flex items-center gap-2.5 rounded-[16px] px-3 py-2.5 text-sm font-medium transition sm:px-3.5 sm:py-2.5"
              :class="isRouteActive(item.path) ? 'bg-slate-900 text-white dark:bg-white dark:text-slate-900' : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-white/5 dark:hover:text-white'"
              @click="handleNavClick"
            >
              <span class="inline-flex h-8 w-8 items-center justify-center rounded-[14px] text-base sm:h-9 sm:w-9 sm:rounded-[16px]"
                :class="isRouteActive(item.path) ? 'bg-white/15 text-white dark:bg-slate-900/10 dark:text-slate-900' : 'bg-slate-100 text-slate-500 group-hover:bg-white group-hover:text-slate-900 dark:bg-white/5 dark:text-slate-400 dark:group-hover:bg-slate-800 dark:group-hover:text-white'">
                <i :class="`ri-${item.icon}`"></i>
              </span>
              <span class="flex-1">{{ item.name }}</span>
              <i class="ri-arrow-right-s-line text-base opacity-40"></i>
            </router-link>
          </li>
        </ul>
      </nav>
    </div>
  </aside>

  <transition name="fade">
    <div v-if="sidebarOpen" class="fixed inset-0 z-40 bg-slate-950/45 lg:hidden" @click="closeSidebar"></div>
  </transition>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import Message from '@/utils/message.js'

const router = useRouter()
const route = useRoute()

const seoTitle = ref('初春图床')
const isLogin = ref(false)
const isDark = ref(false)
const sidebarOpen = ref(false)
const navItems = ref([])
const storageKey = 'theme-preference'

const refreshNavItems = () => {
  const userInfo = JSON.parse(localStorage.getItem('userInfo') || '{}')
  navItems.value = []
  isLogin.value = !!userInfo.username

  if (!isLogin.value) {
    navItems.value.push({ path: '/login', icon: 'login-circle-line', name: '登录' })
    return
  }

  navItems.value.push(
    { path: '/', icon: 'home-5-line', name: '控制台' },
    { path: '/gallery', icon: 'gallery-view-2', name: '图库管理' },
    { path: '/tags', icon: 'price-tag-3-line', name: '标签管理' },
    { path: '/stats', icon: 'bar-chart-grouped-line', name: '数据统计' }
  )

  if (userInfo?.isTourist !== true) {
    navItems.value.push(
      { path: '/buckets', icon: 'database-2-line', name: '存储管理' },
      { path: '/account', icon: 'shield-user-line', name: '账户设置' },
      { path: '/settings', icon: 'settings-4-line', name: '系统设置' }
    )
  }
}

const isRouteActive = (targetPath) => {
  const exactMatchPaths = ['/', '/login']
  if (exactMatchPaths.includes(targetPath)) {
    return route.path === targetPath
  }
  return route.path.startsWith(targetPath)
}

const getFirstWord = (title) => {
  if (!title) return '图'
  return title.trim().slice(0, 1)
}

const detectUserThemePreference = () => {
  if (typeof localStorage !== 'undefined' && localStorage.getItem(storageKey)) {
    return localStorage.getItem(storageKey)
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

const applyTheme = (theme) => {
  const htmlElement = document.documentElement
  isDark.value = theme === 'dark'
  if (isDark.value) {
    htmlElement.classList.add('dark')
  } else {
    htmlElement.classList.remove('dark')
  }
  localStorage.setItem(storageKey, theme)
}

const toggleTheme = () => {
  applyTheme(isDark.value ? 'light' : 'dark')
}

const openSidebar = () => {
  sidebarOpen.value = true
  document.body.style.overflow = 'hidden'
}

const closeSidebar = () => {
  sidebarOpen.value = false
  document.body.style.overflow = ''
}

const toggleSidebar = () => {
  if (sidebarOpen.value) {
    closeSidebar()
  } else {
    openSidebar()
  }
}

const handleNavClick = () => {
  if (window.innerWidth < 768) {
    closeSidebar()
  }
}

const handleLogout = async () => {
  localStorage.removeItem('token')
  localStorage.removeItem('userInfo')
  try {
    await fetch('/api/logout', { method: 'POST' })
    Message.success('登出成功')
    refreshNavItems()
    router.push('/login').catch(() => {})
  } catch (error) {
    Message.error('登出失败')
  }
}

const handleSeoUpdate = (data) => {
  if (data?.seo_title) {
    seoTitle.value = data.seo_title
  }
}

const handleResize = () => {
  if (window.innerWidth >= 1024) {
    sidebarOpen.value = false
    document.body.style.overflow = ''
  }
}

onMounted(() => {
  applyTheme(detectUserThemePreference())
  refreshNavItems()
  window.refreshNavItems = refreshNavItems
  window.seoBus?.onUpdate(handleSeoUpdate)
  if (window.seoStting?.seo_title) {
    seoTitle.value = window.seoStting.seo_title
  }
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  if (window.seoBus?.callbacks) {
    window.seoBus.callbacks = window.seoBus.callbacks.filter((cb) => cb !== handleSeoUpdate)
  }
  window.removeEventListener('resize', handleResize)
  document.body.style.overflow = ''
  delete window.refreshNavItems
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
