<template>
  <!-- ===== 顶栏：通栏毛玻璃，66px，与侧栏解耦 ===== -->
  <header
    class="fixed inset-x-0 top-0 z-40 h-[var(--app-header-height-mobile)] border-b border-slate-200 bg-white/90 backdrop-blur-[14px] dark:border-slate-800 dark:bg-slate-900/90 md:h-[var(--app-header-height)]"
  >
    <div class="mx-auto flex h-full w-full items-center gap-3 px-3 md:gap-6 md:px-6">
      <button
        v-if="isLogin"
        type="button"
        class="inline-flex h-[38px] w-[38px] flex-none items-center justify-center rounded-lg border border-slate-200 text-slate-900 transition-colors hover:bg-slate-100 dark:border-slate-700 dark:text-slate-100 dark:hover:bg-slate-800 lg:hidden"
        aria-label="打开导航" :aria-expanded="sidebarOpen" @click="toggleSidebar"
      >
        <i class="ri-menu-2-line text-base"></i>
      </button>

      <router-link to="/" class="flex flex-none items-center gap-2.5" :aria-label="seoTitle">
        <img v-if="siteIcon" :src="siteIcon" alt="" width="34" height="34" class="h-[34px] w-[34px] flex-none rounded-lg object-cover" />
        <span v-else class="grid h-[34px] w-[34px] flex-none place-items-center rounded-lg bg-primary text-sm font-bold text-white">{{ initial }}</span>
        <span class="hidden leading-tight sm:block">
          <strong class="block max-w-[180px] truncate text-[15px] font-bold text-slate-900 dark:text-white">{{ seoTitle }}</strong>
          <small class="block text-[11px] text-slate-400 dark:text-slate-500">图片托管 · 图库管理</small>
        </span>
      </router-link>

      <!-- 顶栏搜索：直达图库筛选。未登录时没有图库可搜，整块不渲染。
           ml-auto 把搜索与右侧控件推到最右。 -->
      <form v-if="isLogin" class="relative ml-auto hidden w-[168px] flex-none xl:w-[210px] lg:block" role="search" @submit.prevent="submitSearch">
        <i class="ri-search-line pointer-events-none absolute left-4 top-1/2 -translate-y-1/2 text-sm text-slate-500 dark:text-slate-400"></i>
        <input
          v-model="searchQuery" type="search" aria-label="按文件名搜索图片" placeholder="搜索图片…"
          class="h-[38px] w-full rounded-full border border-slate-200 bg-slate-50 py-0 pl-[42px] pr-[18px] text-sm text-slate-900 outline-none transition-colors placeholder:text-slate-400 focus:border-primary/40 focus:bg-white focus:shadow-[0_0_0_4px_rgb(37_99_235_/_0.08)] dark:border-slate-700 dark:bg-slate-800/60 dark:text-slate-100 dark:placeholder:text-slate-500 dark:focus:bg-slate-900"
        />
      </form>

      <!-- 窄屏、未登录都没有搜索框，靠这里的 ml-auto 把控件推到最右 -->
      <div class="ml-auto flex flex-none items-center gap-1.5 lg:ml-0" :class="{ 'lg:ml-auto': !isLogin }">
        <button
          type="button"
          class="theme-toggle grid h-[38px] w-[38px] place-items-center rounded-full text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900 dark:text-slate-400 dark:hover:bg-slate-800 dark:hover:text-white"
          :aria-label="isDark ? '切换浅色主题' : '切换深色主题'" :title="isDark ? '切换浅色主题' : '切换深色主题'"
          @click="toggleTheme"
        >
          <!-- 两个图标叠在同一格里做交叉旋转：月亮转出去、太阳转进来。
               只换 class 会让图标直接跳变，和整页的揭示动画不搭。 -->
          <span class="theme-icon-stack">
            <i class="ri-moon-line theme-icon" :class="isDark ? 'theme-icon-out' : ''" aria-hidden="true"></i>
            <i class="ri-sun-line theme-icon" :class="isDark ? '' : 'theme-icon-out'" aria-hidden="true"></i>
          </span>
        </button>

        <!-- 已登录才有识别身份与退出的必要；未登录不放登录按钮，
             因为登录页本身就是入口，顶栏再放一个只是重复。 -->
        <div v-if="isLogin" ref="userMenuRef" class="relative">
          <button
            type="button"
            class="flex h-[38px] items-center gap-2 rounded-full border border-slate-200 py-0 pl-1 pr-2.5 transition-colors hover:bg-slate-100 dark:border-slate-700 dark:hover:bg-slate-800"
            aria-haspopup="menu" :aria-expanded="userMenuOpen" @click="userMenuOpen = !userMenuOpen"
          >
            <span class="grid h-7 w-7 flex-none place-items-center rounded-full bg-slate-900 text-[11px] font-semibold text-white dark:bg-white dark:text-slate-900">{{ userInitial }}</span>
            <span class="hidden max-w-[90px] truncate text-sm text-slate-700 dark:text-slate-200 md:inline">{{ userName }}</span>
            <i class="ri-arrow-down-s-line text-sm text-slate-400"></i>
          </button>

          <!-- 下拉菜单：进出各有一段时间。
               缓出（leave）比缓入慢一档、并带轻微上移与缩放回缩，
               避免鼠标移开菜单「啪」地消失；缓入用 180ms 的短促弹出，
               点开要立刻有反馈。
               Transition 只包列表本身，不包外层定位 div，否则 <Transition>
               会因根节点不是条件渲染而拿不到 leave 钩子。 -->
          <Transition name="menu-pop">
            <div
              v-if="userMenuOpen" role="menu"
              class="absolute right-0 top-full z-50 mt-2 w-56 origin-top-right overflow-hidden rounded-xl border border-slate-200 bg-white py-1.5 shadow-[0_12px_32px_rgb(15_23_42_/_0.12)] dark:border-slate-700 dark:bg-slate-900"
            >
              <div class="border-b border-slate-100 px-3 pb-2.5 pt-1.5 dark:border-slate-800">
                <p class="truncate text-sm font-semibold text-slate-900 dark:text-white">{{ userName }}</p>
                <p class="mt-0.5 text-[11px] text-slate-400 dark:text-slate-500">{{ roleLabel }}</p>
              </div>
              <router-link
                v-for="item in userMenuItems" :key="item.path" :to="item.path" role="menuitem"
                class="flex items-center gap-2.5 px-3 py-2 text-sm text-slate-700 transition-colors hover:bg-slate-100 dark:text-slate-200 dark:hover:bg-slate-800"
                @click="userMenuOpen = false"
              >
                <i :class="`ri-${item.icon}`" class="w-4 text-center text-[13px] text-slate-400"></i>{{ item.name }}
              </router-link>
              <button
                type="button" role="menuitem"
                class="flex w-full items-center gap-2.5 border-t border-slate-100 px-3 py-2 text-sm text-red-600 transition-colors hover:bg-red-50 dark:border-slate-800 dark:text-red-400 dark:hover:bg-red-950/40"
                @click="handleLogout"
              >
                <i class="ri-logout-box-r-line w-4 text-center text-[13px]"></i>退出登录
              </button>
            </div>
          </Transition>
        </div>
      </div>
    </div>
  </header>

  <!-- ===== 移动端抽屉遮罩 ===== -->
  <transition name="fade">
    <div v-if="sidebarOpen" class="fixed inset-0 z-[105] bg-slate-900/40 lg:hidden" @click="closeSidebar"></div>
  </transition>

  <!-- ===== 侧边栏：贴合顶栏下方，分组目录。
       未登录时整块不渲染：此时没有任何可导航页面，留着只会是个空壳。 ===== -->
  <aside
    v-if="isLogin"
    class="fixed bottom-0 left-0 top-0 z-[110] w-[280px] max-w-[80vw] transition-transform duration-300 ease-out lg:top-[var(--app-header-height)] lg:z-20 lg:w-[var(--app-sidebar-width)] lg:max-w-none"
    :class="sidebarOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'"
    aria-label="侧边导航"
  >
    <div class="flex h-full flex-col overflow-y-auto border-r border-slate-200 bg-white shadow-[0_12px_32px_rgb(15_23_42_/_0.06)] [scrollbar-width:none] dark:border-slate-800 dark:bg-slate-900 [&::-webkit-scrollbar]:hidden">
      <!-- 抽屉头部：站点标识 + 关闭 -->
      <div class="flex items-center justify-between border-b border-slate-200 px-4 py-3 dark:border-slate-800 lg:hidden">
        <div class="flex min-w-0 items-center gap-2.5">
          <span class="grid h-8 w-8 flex-none place-items-center rounded-lg bg-primary text-xs font-bold text-white">{{ initial }}</span>
          <strong class="truncate text-sm font-bold text-slate-900 dark:text-white">{{ seoTitle }}</strong>
        </div>
        <button
          type="button"
          class="grid h-8 w-8 place-items-center rounded-lg text-slate-500 transition-colors hover:bg-slate-100 dark:text-slate-400 dark:hover:bg-slate-800"
          aria-label="关闭导航" @click="closeSidebar"
        >
          <i class="ri-close-line"></i>
        </button>
      </div>

      <div v-for="group in navGroups" :key="group.title" class="border-b border-slate-200 p-4 last:border-b-0 dark:border-slate-800">
        <div class="mb-2.5 text-[15px] font-black text-slate-900 dark:text-slate-100">{{ group.title }}</div>
        <div class="grid gap-[5px]">
          <router-link
            v-for="item in group.items" :key="item.path" :to="item.path"
            class="flex items-center justify-between gap-2.5 rounded-[11px] px-2.5 py-[9px] text-sm no-underline transition-all"
            :class="isRouteActive(item.path)
              ? 'bg-primary/[0.08] font-semibold text-primary dark:text-primary-light'
              : 'text-slate-700 hover:bg-primary/[0.08] hover:text-primary dark:text-slate-300 dark:hover:text-primary-light'"
            :aria-current="isRouteActive(item.path) ? 'page' : undefined"
            @click="handleNavClick"
          >
            <span class="flex min-w-0 items-center gap-2 overflow-hidden">
              <i
                class="w-4 flex-none text-center text-[13px] transition-colors"
                :class="[`ri-${item.icon}`, isRouteActive(item.path) ? 'text-primary dark:text-primary-light' : 'text-slate-400 dark:text-slate-500']"
              ></i>
              <span class="truncate">{{ item.name }}</span>
            </span>
            <small
              v-if="item.badge" class="flex-none text-[11px]"
              :class="isRouteActive(item.path) ? 'text-primary dark:text-primary-light' : 'text-slate-400 dark:text-slate-500'"
            >{{ item.badge }}</small>
          </router-link>
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import Message from '@/utils/message.js'
import { lockScroll, unlockScroll } from '@/utils/overlay.js'

const sidebarScrollLock = {}

const router = useRouter()
const route = useRoute()

const seoTitle = ref('OneImg')
const siteIcon = ref('')
const isLogin = ref(false)
const isDark = ref(false)
const sidebarOpen = ref(false)
const userMenuOpen = ref(false)
const userMenuRef = ref(null)
const searchQuery = ref('')
const userInfo = ref({})
const navGroups = ref([])
const storageKey = 'theme-preference'

const initial = computed(() => (seoTitle.value || '图').trim().slice(0, 1))
const userName = computed(() => userInfo.value.username || '未登录')
const userInitial = computed(() => (userInfo.value.username || '?').trim().slice(0, 1).toUpperCase())
const roleLabel = computed(() => {
  const role = Number(userInfo.value.role)
  return role === 1 ? '管理员' : role === 2 ? '游客' : '普通用户'
})

// 顶栏不再放一级导航，页面切换统一走左侧边栏。
const userMenuItems = computed(() => {
  const items = []
  // 与侧栏保持一致：账户设置在上，系统设置在下
  if (isLogin.value && Number(userInfo.value.role) !== 2) items.push({ path: '/account', icon: 'shield-user-line', name: '账户设置' })
  if (canSeeSettings.value) items.push({ path: '/settings', icon: 'settings-4-line', name: '系统设置' })
  return items
})

const canSeeSettings = computed(() => (
  Number(userInfo.value.id) === 1 || !!userInfo.value.permission?.codes?.includes('setting:list')
))

// 侧栏只在登录后存在，内容区让位宽度随之切换（见 App.vue 的 --app-content-inset）。
watch(isLogin, (loggedIn) => {
  document.documentElement.style.setProperty('--app-content-inset', loggedIn ? 'var(--app-sidebar-width)' : '0px')
}, { immediate: true })

const buildNav = () => {
  let info = {}
  try { info = JSON.parse(localStorage.getItem('userInfo') || '{}') } catch { info = {} }
  userInfo.value = info
  isLogin.value = !!info.username

  if (!isLogin.value) {
    // 未登录没有可导航项：顶栏只留品牌，侧栏整块不渲染。
    navGroups.value = []
    return
  }

  const isAdmin = Number(info.role) === 1
  const groups = [{
    title: '工作区',
    items: [
      { path: '/', icon: 'home-5-line', name: '控制台' },
      { path: '/gallery', icon: 'image-2-line', name: '我的图库' },
      { path: '/tags', icon: 'price-tag-3-line', name: '标签管理' },
      { path: '/stats', icon: 'bar-chart-grouped-line', name: '数据统计' },
    ],
  }]

  if (isAdmin) {
    groups.push({
      title: '管理',
      items: [
        { path: '/admin/gallery', icon: 'apps-2-line', name: '总图库' },
        { path: '/buckets', icon: 'database-2-line', name: '存储管理' },
        { path: '/users', icon: 'user-line', name: '用户管理' },
      ],
    })
  }

  const accountItems = []
  // 账户设置在前、系统设置在后：先管「我的账号」，再进「全站配置」。
  if (Number(info.role) !== 2) accountItems.push({ path: '/account', icon: 'shield-user-line', name: '账户设置' })
  if (canSeeSettings.value) accountItems.push({ path: '/settings', icon: 'settings-4-line', name: '系统设置' })
  if (accountItems.length) groups.push({ title: '账户', items: accountItems })

  navGroups.value = groups
}

const isRouteActive = (targetPath) => {
  const exactMatchPaths = ['/', '/login']
  if (exactMatchPaths.includes(targetPath)) return route.path === targetPath
  return route.path.startsWith(targetPath)
}

const detectUserThemePreference = () => {
  if (typeof localStorage !== 'undefined' && localStorage.getItem(storageKey)) return localStorage.getItem(storageKey)
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

const applyTheme = (theme) => {
  const htmlElement = document.documentElement
  isDark.value = theme === 'dark'
  htmlElement.classList.toggle('dark', isDark.value)
  localStorage.setItem(storageKey, theme)
}

// 主题切换动画。
// body 早就有 transition-colors，但那只能让背景色自己渐变 —— 深色/浅色切换时
// 页面上几十个卡片的边框、文字色都是瞬间跳变，看起来像「闪一下」。
// 这里用 View Transitions API 做一次从按钮位置扩散的圆形揭示：
//   1. 给 html 加一圈「新主题色」的径向遮罩，圆心落在点击的按钮上；
//   2. 旧画面做快照、新画面做快照，新画面裁在圆内；
//   3. 圆半径从 0 扩到对角线长度，旧主题就被从点击处「推开」。
// 不支持该 API 的浏览器（Firefox 旧版、Safari < 18）直接跳过动画，
// 退回原来的瞬间切换 —— 功能不受影响。
const prefersReducedMotion = () => {
  try { return window.matchMedia('(prefers-reduced-motion: reduce)').matches } catch { return false }
}

// 正在播放揭示动画时忽略新的切换请求：
// startViewTransition 并发调用会让后一次直接接管（前一次的 finished 提前 resolve），
// 表现为「点了两下只闪一下」，且 data-theme-reveal 可能被后一次提前清掉。
// 单次动画 0.52s，忽略期间用户看到的是按钮图标已在转，不会有「点了没反应」的错觉。
let themeTransitionActive = false

const animateThemeChange = (theme, originEl) => {
  // 尊重系统的「减少动态效果」：此时不做揭示动画
  if (prefersReducedMotion() || typeof document.startViewTransition !== 'function') {
    applyTheme(theme)
    return
  }
  if (themeTransitionActive) return

  // 圆心取点击的按钮中心；拿不到元素就退回视口右上角（主题按钮所在方位）
  const rect = originEl?.getBoundingClientRect?.()
  const x = rect ? rect.left + rect.width / 2 : window.innerWidth - 80
  const y = rect ? rect.top + rect.height / 2 : 40
  const radius = Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y))

  // 揭示层的底色必须与新主题一致，否则扩散圆里露出的仍是旧色
  document.documentElement.style.setProperty('--theme-reveal-x', `${x}px`)
  document.documentElement.style.setProperty('--theme-reveal-y', `${y}px`)
  document.documentElement.style.setProperty('--theme-reveal-r', `${radius}px`)
  document.documentElement.dataset.themeReveal = theme

  const transition = document.startViewTransition(() => applyTheme(theme))
  themeTransitionActive = true
  transition.finished.finally(() => {
    themeTransitionActive = false
    delete document.documentElement.dataset.themeReveal
  })
}

// 主题按钮带出自身位置，供揭示动画定位圆心
const toggleTheme = (event) => {
  const next = isDark.value ? 'light' : 'dark'
  animateThemeChange(next, event?.currentTarget)
}

const openSidebar = () => {
  sidebarOpen.value = true
  lockScroll(sidebarScrollLock)
}

const closeSidebar = () => {
  sidebarOpen.value = false
  unlockScroll(sidebarScrollLock)
}

const toggleSidebar = () => (sidebarOpen.value ? closeSidebar() : openSidebar())

const handleNavClick = () => {
  userMenuOpen.value = false
  if (window.innerWidth < 1024) closeSidebar()
}

const submitSearch = () => {
  // 搜索框只在登录后渲染，这里无需再判登录态。
  const keyword = searchQuery.value.trim()
  router.push({ path: '/gallery', query: keyword ? { search: keyword } : {} }).catch(() => {})
  if (window.innerWidth < 1024) closeSidebar()
}

const handleLogout = async () => {
  userMenuOpen.value = false
  localStorage.removeItem('token')
  localStorage.removeItem('userInfo')
  try {
    await fetch('/api/logout', { method: 'POST' })
    Message.success('登出成功')
  } catch (error) {
    Message.error('登出失败')
  }
  buildNav()
  router.push('/login').catch(() => {})
}

const handleSeoUpdate = (data) => {
  if (data?.seo_title) seoTitle.value = data.seo_title
  if (typeof data?.seo_icon === 'string') siteIcon.value = data.seo_icon
}

const handleDocumentClick = (event) => {
  if (!userMenuOpen.value) return
  if (userMenuRef.value && !userMenuRef.value.contains(event.target)) userMenuOpen.value = false
}

const handleResize = () => {
  if (window.innerWidth >= 1024) {
    sidebarOpen.value = false
    unlockScroll(sidebarScrollLock)
  }
}

// 图库页的搜索词回填到顶栏，两处状态保持一致
watch(() => route.query.search, (value) => {
  searchQuery.value = typeof value === 'string' ? value : ''
})

onMounted(() => {
  applyTheme(detectUserThemePreference())
  buildNav()
  window.refreshNavItems = buildNav
  window.seoBus?.onUpdate(handleSeoUpdate)
  if (window.seoStting) {
    seoTitle.value = window.seoStting.seo_title || seoTitle.value
    siteIcon.value = window.seoStting.seo_icon || ''
  }
  searchQuery.value = typeof route.query.search === 'string' ? route.query.search : ''
  window.addEventListener('resize', handleResize)
  document.addEventListener('click', handleDocumentClick)
})

onUnmounted(() => {
  if (window.seoBus?.callbacks) {
    window.seoBus.callbacks = window.seoBus.callbacks.filter((cb) => cb !== handleSeoUpdate)
  }
  window.removeEventListener('resize', handleResize)
  document.removeEventListener('click', handleDocumentClick)
  unlockScroll(sidebarScrollLock)
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

/* ===== 主题按钮的图标交叉旋转 =====
   两个图标叠在同一格，用 rotate + scale + opacity 做「转出 / 转入」。
   时长与揭示动画的 0.52s 对齐，两者同时结束才不显得脱节。 */
.theme-icon-stack {
  position: relative;
  display: grid;
  height: 18px;
  width: 18px;
  place-items: center;
}

.theme-icon {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  font-size: 18px;
  /* 必须显式写 1。图标用的是字体，字形按「基线」排在行盒里，
     行盒高度是 line-height 而非 font-size —— 继承来的 1.5（27px）会让
     18px 高的容器里塞进 27px 行盒，字形整体下移约 4px（实测 dy=+4.00）。
     line-height:1 让行盒等于字号，grid 居中才是几何居中。 */
  line-height: 1;
  opacity: 1;
  transform: rotate(0deg) scale(1);
  transition: opacity 0.28s ease, transform 0.52s cubic-bezier(0.4, 0, 0.2, 1);
}

/* 转出去的那一个：缩到 0.4 并旋转 70°，同时淡出 */
.theme-icon-out {
  opacity: 0;
  transform: rotate(70deg) scale(0.4);
}

@media (prefers-reduced-motion: reduce) {
  .theme-icon { transition: opacity 0.15s linear; }
  .theme-icon-out { transform: none; }
}

/* ===== 用户菜单进 / 缓出 =====
   进入：180ms 短促弹出（translateY(-6px) + scale(.96) → 原位），
         点开要立刻有反馈，不能让人等。
   离开：240ms 缓出，位移更大一点（-4px）并回缩到 .97，配合快速淡出，
         视觉上像「轻轻收回去」，而不是被一刀切掉。
   transform-origin 在右上角，缩放才不会让菜单往左飘离按钮。 */
.menu-pop-enter-active {
  transition: opacity 0.18s ease-out, transform 0.18s cubic-bezier(0.16, 1, 0.3, 1);
}

.menu-pop-leave-active {
  /* 离开比进入慢，且用 ease-in 让尾部「飘走」而不是戛然停止 */
  transition: opacity 0.24s ease-in, transform 0.24s cubic-bezier(0.4, 0, 0.6, 1);
  /* 避免离场时还挡着下方元素的点击 */
  pointer-events: none;
}

.menu-pop-enter-from {
  opacity: 0;
  transform: translateY(-6px) scale(0.96);
}

.menu-pop-leave-to {
  opacity: 0;
  transform: translateY(-4px) scale(0.97);
}

@media (prefers-reduced-motion: reduce) {
  .menu-pop-enter-active,
  .menu-pop-leave-active {
    transition: opacity 0.12s linear;
  }
  .menu-pop-enter-from,
  .menu-pop-leave-to {
    transform: none;
  }
}
</style>
