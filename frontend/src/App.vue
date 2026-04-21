<template>
  <div class="app-shell min-h-screen bg-slate-100 text-slate-900 dark:bg-slate-950 dark:text-slate-100 lg:pl-[var(--app-sidebar-width)]">
    <Navbar />

    <main class="min-h-screen px-2.5 pb-3 pt-[calc(var(--app-header-height-mobile)+8px)] sm:px-4 sm:pb-4 md:px-5 md:pb-5 md:pt-[calc(var(--app-header-height)+10px)] xl:px-6 2xl:px-8">
      <div class="mx-auto max-w-[1440px]">
        <div class="page-surface px-2.5 py-2.5 sm:px-3.5 sm:py-4 md:px-4 md:py-4 xl:px-4.5 xl:py-4.5">
          <router-view />
        </div>
      </div>
    </main>

    <footer class="px-3 pb-4 sm:px-4 md:px-6 md:pb-5 xl:px-8">
      <div class="mx-auto max-w-[1440px]">
        <div class="flex flex-col gap-1.5 border-t border-slate-200/80 px-1 pt-3 text-center text-[11px] text-slate-500 dark:border-white/10 dark:text-slate-400 sm:text-xs md:flex-row md:items-center md:justify-between md:gap-2.5 md:text-sm">
          <div>
            © {{ year }}
            <a href="/" class="font-medium text-slate-700 transition hover:text-primary dark:text-slate-200 dark:hover:text-primary">
              {{ seoSetting.seo_title || '初春图床' }}
            </a>
          </div>
          <div class="flex flex-wrap items-center justify-center gap-2 md:justify-end md:gap-2.5">
            <a
              v-if="seoSetting.seo_icp"
              href="http://beian.miit.gov.cn/"
              target="_blank"
              class="inline-flex items-center gap-1.5 rounded-full border border-slate-200 bg-white px-2.5 py-1 transition hover:border-slate-300 hover:text-slate-900 sm:px-3 dark:border-white/10 dark:bg-slate-900 dark:hover:border-white/20 dark:hover:text-white"
            >
              <img class="h-4 w-4" :src="icpImg" alt="ICP" />
              <span>{{ seoSetting.seo_icp }}</span>
            </a>
            <a
              v-if="seoSetting.public_security"
              href="https://beian.mps.gov.cn/#/query/webSearch"
              target="_blank"
              class="inline-flex items-center gap-1.5 rounded-full border border-slate-200 bg-white px-2.5 py-1 transition hover:border-slate-300 hover:text-slate-900 sm:px-3 dark:border-white/10 dark:bg-slate-900 dark:hover:border-white/20 dark:hover:text-white"
            >
              <img class="h-4 w-4" :src="securityImg" alt="公安备案" />
              <span>{{ seoSetting.public_security }}</span>
            </a>
          </div>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import Navbar from '@/components/NavBar.vue'
import icpImg from '@/assets/images/icp.svg'
import securityImg from '@/assets/images/gongan.png'

const seoSetting = ref({
  seo_title: '初春图床',
  seo_description: '',
  seo_keywords: '',
  seo_icp: '',
  public_security: '',
  seo_icon: ''
})

const year = new Date().getFullYear()

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
