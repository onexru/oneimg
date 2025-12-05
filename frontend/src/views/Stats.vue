<template>
    <div class="text-gray-800 dark:text-gray-200">
        <!-- 主要内容 -->
        <div class="stats-content container mx-auto px-4 py-8">
            <div class="stats-header mb-8 text-center">
                <h1 class="stats-title text-3xl font-bold mb-2">系统统计</h1>
                <p class="stats-subtitle text-gray-600 dark:text-gray-400">查看图床系统的使用情况和统计数据</p>
            </div>
            
            <!-- 加载状态 -->
            <div v-if="loading" class="loading-container flex flex-col items-center justify-center py-20">
                <div class="spinner w-10 h-10 border-4 border-gray-200 dark:border-gray-700 border-t-primary dark:border-t-primary rounded-full animate-spin mb-4"></div>
                <p class="text-gray-600 dark:text-gray-400">加载统计数据中...</p>
            </div>
            
            <!-- 统计卡片 -->
            <div v-else class="stats-grid grid grid-cols-1 lg:grid-cols-3 gap-4 mb-10">
                <!-- 总图片数 -->
                <div class="stat-card bg-white dark:bg-gray-800 rounded-xl shadow-md p-6 hover:shadow-lg transition-shadow duration-300 flex flex-col items-center text-center">
                    <div class="stat-icon text-4xl mb-4">
                        <i class="ri-image-circle-line"></i>
                    </div>
                    <div class="stat-content">
                        <h3 class="stat-number text-2xl font-bold mb-1">{{ formatNumber(stats.total_images) }}</h3>
                        <p class="stat-label text-gray-600 dark:text-gray-400">总图片数</p>
                    </div>
                </div>
                
                <!-- 总存储空间 -->
                <div class="stat-card bg-white dark:bg-gray-800 rounded-xl shadow-md p-6 hover:shadow-lg transition-shadow duration-300 flex flex-col items-center text-center">
                    <div class="stat-icon text-4xl mb-4">
                        <i class="ri-folder-3-line"></i>
                    </div>
                    <div class="stat-content">
                        <h3 class="stat-number text-2xl font-bold mb-1">{{ formatFileSize(stats.total_size) }}</h3>
                        <p class="stat-label text-gray-600 dark:text-gray-400">总存储空间</p>
                    </div>
                </div>
                
                <!-- 本月上传 -->
                <div class="stat-card bg-white dark:bg-gray-800 rounded-xl shadow-md p-6 hover:shadow-lg transition-shadow duration-300 flex flex-col items-center text-center">
                    <div class="stat-icon text-4xl mb-4">
                        <i class="ri-calendar-line"></i>
                    </div>
                    <div class="stat-content">
                        <h3 class="stat-number text-2xl font-bold mb-1">{{ formatNumber(stats.month_uploads) }}</h3>
                        <p class="stat-label text-gray-600 dark:text-gray-400">本月上传</p>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- 通知消息 - 固定在右下角 -->
        <div 
            v-if="notification.show" 
            class="notification fixed bottom-4 right-4 px-6 py-3 rounded-lg shadow-lg z-50 transition-all duration-300 transform translate-y-0 opacity-100"
            :class="[
                notification.type === 'success' ? 'bg-green-500 text-white' : 'bg-red-500 text-white',
            ]"
        >
            {{ notification.message }}
        </div>
    </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'

// 响应式数据
const loading = ref(false)
const stats = ref({
    total_images: 0,
    total_size: 0,
    today_uploads: 0,
    month_uploads: 0,
    average_size: 0,
    max_size: 0,
    upload_trend: []
})

// 通知消息
const notification = ref({
    show: false,
    message: '',
    type: 'success'
})

// 计算上传趋势的最大计数（优化性能）
const maxTrendCount = computed(() => {
    if (!stats.value.upload_trend.length) return 0
    return Math.max(...stats.value.upload_trend.map(day => day.count))
})

// 加载统计数据
const loadStats = async () => {
    loading.value = true
    
    try {
        const response = await fetch('/api/stats/dashboard', {
            headers: {
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            }
        })
        
        if (!response.ok) {
            // 未授权处理
            if (response.status === 401) {
                localStorage.removeItem('authToken')
                window.location.href = '/login'
                showNotification('登录已过期，请重新登录', 'error')
                return
            }
            throw new Error('加载统计数据失败')
        }
        
        const result = await response.json()
        stats.value = { ...stats.value, ...(result.data || {}) }
    } catch (error) {
        console.error('加载统计数据错误:', error)
        showNotification('加载统计数据失败: ' + error.message, 'error')
    } finally {
        loading.value = false
    }
}

// 工具函数
/** 格式化数字为千分位 */
const formatNumber = (num) => {
    return num ? num.toLocaleString('zh-CN') : '0'
}

/** 格式化文件大小 */
const formatFileSize = (bytes) => {
    if (!bytes) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

/** 显示通知 */
const showNotification = (message, type = 'success') => {
    notification.value = { show: true, message, type }
    setTimeout(() => notification.value.show = false, 3000)
}

// 生命周期
onMounted(() => {
    loadStats()
})
</script>