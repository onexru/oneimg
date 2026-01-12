<template>
  <div id="app" class="relative min-h-screen">
      <!-- 背景网格 -->
    <div class="fixed inset-0 bg-grid opacity-70 dark:opacity-50"></div>
    
    <!-- 装饰性背景元素 -->
    <div class="fixed top-20 -left-20 w-64 h-64 bg-primary/10 dark:bg-primary/20 rounded-full decorative-blur animate-pulse-slow"></div>
    <div class="fixed bottom-20 -right-20 w-80 h-80 bg-primary-dark/10 dark:bg-primary-dark/20 rounded-full decorative-blur animate-pulse-slow" style="animation-delay: 1s;"></div>
    
    <Navbar />
    
    <!-- 主内容区 -->
    <main class="pt-24 pb-16 px-4 relative z-10 md:ml-[255px] transition-all">
        <router-view class="mb-8"></router-view>
    </main>

    <!-- 底部版权信息 -->
    <footer class="absolute bottom-0 left-0 right-0 min-h-16 border-light-200 dark:border-dark-100 shadow-md dark:shadow-dark-md z-40 md:ml-[255px]">
        <div class="px-4 py-2 text-center text-xs text-gray-500 dark:text-gray-400">
            © {{ year }} <a href="/" class="hover:underline">{{ seoSetting.seo_title || '初春图床'}}</a>. All rights reserved.
            <div class="md:flex items-center justify-center mt-1 gap-2">
                <!-- 设置备案信息 -->
                <p v-if="seoSetting.seo_icp" class="mt-1">
                    <img class="inline-block h-6 w-6" :src="icpImg" alt="ICP"/>
                    <a href="http://www.beian.miit.gov.cn/" target="_blank" class="hover:underline">{{seoSetting.seo_icp}}</a>
                </p>
                <!-- 设置公安备案信息 -->
                <p v-if="seoSetting.public_security" class="mt-1">
                    <img class="inline-block h-6 w-6" :src="securityImg" alt="公安部备案"/>
                    <a href="https://beian.mps.gov.cn/#/query/webSearch" target="_blank" class="hover:underline">{{seoSetting.public_security}}</a>
                </p>
            </div>
        </div>
    </footer>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import Navbar from "@/components/NavBar.vue";
import icpImg from '@/assets/images/icp.svg';
import securityImg from '@/assets/images/gongan.png';

const seoSetting = ref({
    seo_title: '初春图床',
    seo_description: '',
    seo_keywords: '',
    seo_icp: '',
    public_security: '',
    seo_icon: ''
});
const year = new Date().getFullYear();

const handleSeoUpdate = (data) => {
  if (!data || typeof data !== 'object') return;

  // 更新组件内SEO数据
  seoSetting.value = { ...seoSetting.value, ...data };

  // 封装元标签设置函数
  const setMetaTag = (name, content) => {
    // 查找现有标签，找不到则创建
    let tag = document.querySelector(`meta[name="${name}"]`);
    if (!tag) {
      tag = document.createElement('meta');
      tag.setAttribute('name', name);
      document.head.appendChild(tag);
    }
    if (content && content.trim()) {
      tag.setAttribute('content', content.trim());
    } else {
      tag.removeAttribute('content');
    }
  };

  setMetaTag('description', seoSetting.value.seo_description);
  setMetaTag('keywords', seoSetting.value.seo_keywords);
};

onMounted(() => {
  window.seoBus?.onUpdate(handleSeoUpdate);
  if (window.seoStting) {
    seoSetting.value = window.seoStting;
  }
});

onUnmounted(() => {
  window.seoBus.callbacks = window.seoBus.callbacks.filter(cb => cb !== handleSeoUpdate);
});
</script>