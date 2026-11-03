<template>
  <div class="app-shell flex min-h-screen flex-col bg-slate-50 text-slate-900 dark:bg-slate-950 dark:text-slate-100">
    <Navbar />

    <!-- 内容区偏移由 NavBar 写入的 --app-content-inset 控制：
         登录后让出侧栏宽度，未登录时侧栏不存在，偏移自动归零。
         main 用 flex-1 吃掉剩余高度，页面内容不足时由它补齐，
         这样短页面（登录/注册/标签）不会在底部留下空白再触发滚动条。 -->
    <main class="flex min-h-0 min-w-0 flex-1 flex-col px-3 pb-4 pt-[calc(var(--app-header-height-mobile)+20px)] sm:px-5 md:px-6 lg:ml-[var(--app-content-inset)]">
      <!-- w-full 不能省：这是 flex 列方向的子项，没有它只会按内容宽度收缩，
           登录卡片会被压成 260px（父级只剩内容宽），max-w 再好也撑不起来。 -->
      <div class="mx-auto flex w-full min-w-0 flex-1 flex-col">
        <router-view />
      </div>
    </main>

    <!-- 页脚：不用卡片，回到一条横向信息带。
         与内容区共用边距和侧栏偏移，左边线与主内容对齐。 -->
    <footer ref="footerRef" class="app-footer-wrap">
      <div class="app-footer">
        <div class="app-footer-brand">
          <router-link to="/" class="app-footer-name">
            {{ seoSetting.seo_title || 'OneImg' }}
          </router-link>
          <span class="app-footer-copy">© {{ year }}</span>
        </div>

        <div class="app-footer-legal">
          <a
            v-if="seoSetting.seo_icp"
            href="http://beian.miit.gov.cn/"
            target="_blank" rel="noopener noreferrer nofollow"
            class="app-footer-beian"
          >
            <img class="h-3.5 w-3.5" :src="icpImg" alt="" />
            <span>{{ seoSetting.seo_icp }}</span>
          </a>
          <a
            v-if="seoSetting.public_security"
            href="https://beian.mps.gov.cn/#/query/webSearch"
            target="_blank" rel="noopener noreferrer nofollow"
            class="app-footer-beian"
          >
            <img class="h-3.5 w-3.5" :src="securityImg" alt="" />
            <span>{{ seoSetting.public_security }}</span>
          </a>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import Navbar from '@/components/NavBar.vue'
import icpImg from '@/assets/images/icp.svg'
import securityImg from '@/assets/images/gongan.png'

const seoSetting = ref({
  seo_title: 'OneImg',
  seo_description: '',
  seo_keywords: '',
  seo_icp: '',
  public_security: '',
  seo_icon: ''
})

const footerRef = ref(null)

const year = new Date().getFullYear()

// 页脚高度随备案号折行、断点、字号变化，硬编码会让登录页算出错误的最小高度，
// 进而凭空多出一条滚动条。这里实测后写进 CSS 变量，登录/注册页据此计算剩余空间。
let footerObserver = null
const syncFooterHeight = () => {
  const el = footerRef.value
  if (!el) return
  const height = Math.ceil(el.getBoundingClientRect().height)
  document.documentElement.style.setProperty('--app-footer-height', `${height}px`)
}

onMounted(() => {
  syncFooterHeight()
  if (typeof ResizeObserver !== 'undefined' && footerRef.value) {
    footerObserver = new ResizeObserver(syncFooterHeight)
    footerObserver.observe(footerRef.value)
  } else {
    window.addEventListener('resize', syncFooterHeight)
  }
})

onUnmounted(() => {
  footerObserver?.disconnect()
  window.removeEventListener('resize', syncFooterHeight)
})

// 备案号/站名加载完成后页脚高度会变，重新量一次
watch(() => [seoSetting.value.seo_icp, seoSetting.value.public_security, seoSetting.value.seo_title], () => {
  nextTick(syncFooterHeight)
})

const handleSeoUpdate = (data) => {
  if (!data || typeof data !== 'object') return
  seoSetting.value = { ...seoSetting.value, ...data }

  const setMetaTag = (name, content) => {
    let tag = document.querySelector(`meta[name="${name}"]`)
    if (!tag) {
      tag = document.createElement('meta')
      tag.setAttribute('name', name)
      document.head.appendChild(tag)
    }
    if (content && content.trim()) {
      tag.setAttribute('content', content.trim())
    } else {
      tag.removeAttribute('content')
    }
  }

  setMetaTag('description', seoSetting.value.seo_description)
  setMetaTag('keywords', seoSetting.value.seo_keywords)
}

onMounted(() => {
  window.seoBus?.onUpdate(handleSeoUpdate)
  if (window.seoStting) {
    seoSetting.value = window.seoStting
  }
})

onUnmounted(() => {
  if (window.seoBus?.callbacks) {
    window.seoBus.callbacks = window.seoBus.callbacks.filter((cb) => cb !== handleSeoUpdate)
  }
})
</script>
