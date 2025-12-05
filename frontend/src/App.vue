<template>
  <div id="app">
      <!-- 背景网格 -->
    <div class="fixed inset-0 bg-grid opacity-70 dark:opacity-50"></div>
    
    <!-- 装饰性背景元素 -->
    <div class="fixed top-20 -left-20 w-64 h-64 bg-primary/10 dark:bg-primary/20 rounded-full decorative-blur animate-pulse-slow"></div>
    <div class="fixed bottom-20 -right-20 w-80 h-80 bg-primary-dark/10 dark:bg-primary-dark/20 rounded-full decorative-blur animate-pulse-slow" style="animation-delay: 1s;"></div>
    
    <Navbar />
    
    <!-- 侧边栏 -->
    <aside id="sidebar" class="fixed top-16 left-0 h-[calc(100vh-4rem)] w-64 bg-white dark:bg-dark-200 border-r border-light-200 dark:border-dark-100 shadow-md dark:shadow-dark-md z-40 sidebar-closed transition-all duration-300 overflow-y-auto">
        <div class="p-4 border-b border-light-200 dark:border-dark-100">
            <h3 class="font-medium text-secondary">导航菜单</h3>
        </div>
        <nav class="p-2">
            <ul id="sidebar-menu" class="space-y-1"></ul>
        </nav>
    </aside>

    <!-- 侧边栏遮罩层 -->
    <div id="sidebarOverlay" class="fixed inset-0 bg-black/20 dark:bg-black/40 backdrop-blur-sm z-30 overlay-hidden transition-all duration-300"></div>
    
    <!-- 主内容区 -->
    <main class="flex-grow pt-24 pb-16 px-4 relative z-10">
        <router-view></router-view>
    </main>
  </div>
</template>

<script setup>
import { ref, onMounted, inject } from 'vue';
import Navbar from "@/components/NavBar.vue";


onMounted(() => {
  
})
</script>