<template>
    <div class="stats">
        <!-- 导航栏 -->
        <NavBar />
        
        <!-- 主要内容 -->
        <div class="stats-content">
            <div class="stats-header">
                <h1 class="stats-title">系统统计</h1>
                <p class="stats-subtitle">查看图床系统的使用情况和统计数据</p>
            </div>
            
            <!-- 加载状态 -->
            <div v-if="loading" class="loading-container">
                <div class="spinner"></div>
                <p>加载统计数据中...</p>
            </div>
            
            <!-- 统计卡片 -->
            <div v-else class="stats-grid">
                <!-- 总图片数 -->
                <div class="stat-card">
                    <div class="stat-icon">📷</div>
                    <div class="stat-content">
                        <h3 class="stat-number">{{ formatNumber(stats.total_images) }}</h3>
                        <p class="stat-label">总图片数</p>
                    </div>
                </div>
                
                <!-- 总存储空间 -->
                <div class="stat-card">
                    <div class="stat-icon">💾</div>
                    <div class="stat-content">
                        <h3 class="stat-number">{{ formatFileSize(stats.total_size) }}</h3>
                        <p class="stat-label">总存储空间</p>
                    </div>
                </div>
                
                <!-- 今日上传 -->
                <div class="stat-card">
                    <div class="stat-icon">📈</div>
                    <div class="stat-content">
                        <h3 class="stat-number">{{ formatNumber(stats.today_uploads) }}</h3>
                        <p class="stat-label">今日上传</p>
                    </div>
                </div>
                
                <!-- 本月上传 -->
                <div class="stat-card">
                    <div class="stat-icon">📊</div>
                    <div class="stat-content">
                        <h3 class="stat-number">{{ formatNumber(stats.month_uploads) }}</h3>
                        <p class="stat-label">本月上传</p>
                    </div>
                </div>
                
                <!-- 平均文件大小 -->
                <div class="stat-card">
                    <div class="stat-icon">📏</div>
                    <div class="stat-content">
                        <h3 class="stat-number">{{ formatFileSize(stats.average_size) }}</h3>
                        <p class="stat-label">平均文件大小</p>
                    </div>
                </div>
                
                <!-- 最大文件大小 -->
                <div class="stat-card">
                    <div class="stat-icon">🔝</div>
                    <div class="stat-content">
                        <h3 class="stat-number">{{ formatFileSize(stats.max_size) }}</h3>
                        <p class="stat-label">最大文件大小</p>
                    </div>
                </div>
            </div>
            
            <!-- 详细统计 -->
            <div v-if="!loading" class="detailed-stats">
                <!-- 上传趋势 -->
                <div class="detail-section">
                    <h2 class="section-title">最近7天上传趋势</h2>
                    <div class="trend-chart">
                        <div 
                            v-for="(day, index) in stats.upload_trend" 
                            :key="index"
                            class="trend-bar"
                        >
                            <div 
                                class="trend-fill"
                                :style="{ 
                                    height: getTrendHeight(day.count, stats.upload_trend) + '%'
                                }"
                            ></div>
                            <span class="trend-count">{{ day.count }}</span>
                            <span class="trend-date">{{ formatTrendDate(day.date) }}</span>
                        </div>
                    </div>
                </div>
                
                <!-- 存储使用情况 -->
                <div class="detail-section">
                    <h2 class="section-title">存储使用情况</h2>
                    <div class="storage-info">
                        <div class="storage-item">
                            <span class="storage-label">已使用空间:</span>
                            <span class="storage-value">{{ formatFileSize(stats.total_size) }}</span>
                        </div>
                        <div class="storage-item">
                            <span class="storage-label">图片数量:</span>
                            <span class="storage-value">{{ formatNumber(stats.total_images) }} 个</span>
                        </div>
                        <div class="storage-item">
                            <span class="storage-label">平均大小:</span>
                            <span class="storage-value">{{ formatFileSize(stats.average_size) }}</span>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        
        <!-- 通知消息 -->
        <div v-if="notification.show" class="notification" :class="notification.type">
            {{ notification.message }}
        </div>
    </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import NavBar from '@/components/NavBar.vue'

// 响应式数据
const loading = ref(false)
const stats = ref({
    total_images: 0,
    total_size: 0,
    today_uploads: 0,
    month_uploads: 0,
    average_size: 0,
    max_size: 0,
    type_distribution: [],
    upload_trend: []
})

// 通知消息
const notification = ref({
    show: false,
    message: '',
    type: 'success'
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
        
        if (response.ok) {
            const result = await response.json()
            stats.value = result.data || stats.value
        } else {
            throw new Error('加载统计数据失败')
        }
    } catch (error) {
        console.error('加载统计数据错误:', error)
        showNotification('加载统计数据失败: ' + error.message, 'error')
    } finally {
        loading.value = false
    }
}

// 工具函数
const formatNumber = (num) => {
    if (!num) return '0'
    return num.toLocaleString('zh-CN')
}

const formatFileSize = (bytes) => {
    if (!bytes) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const getTypeName = (mimeType) => {
    const typeMap = {
        'image/jpeg': 'JPEG',
        'image/jpg': 'JPG',
        'image/png': 'PNG',
        'image/gif': 'GIF',
        'image/webp': 'WebP',
        'image/svg+xml': 'SVG',
        'image/bmp': 'BMP',
        'image/tiff': 'TIFF'
    }
    return typeMap[mimeType] || mimeType.split('/')[1]?.toUpperCase() || '未知'
}

const getTypeColor = (mimeType) => {
    const colorMap = {
        'image/jpeg': '#ff6b6b',
        'image/jpg': '#ff6b6b',
        'image/png': '#4ecdc4',
        'image/gif': '#45b7d1',
        'image/webp': '#96ceb4',
        'image/svg+xml': '#feca57',
        'image/bmp': '#ff9ff3',
        'image/tiff': '#54a0ff'
    }
    return colorMap[mimeType] || '#ddd'
}

const getTrendHeight = (count, trendData) => {
    if (!trendData || trendData.length === 0) return 0
    const maxCount = Math.max(...trendData.map(d => d.count))
    return maxCount > 0 ? (count / maxCount) * 100 : 0
}

const formatTrendDate = (dateString) => {
    const date = new Date(dateString)
    return date.toLocaleDateString('zh-CN', { month: 'short', day: 'numeric' })
}

const showNotification = (message, type = 'success') => {
    notification.value = {
        show: true,
        message,
        type
    }
    
    setTimeout(() => {
        notification.value.show = false
    }, 3000)
}

// 生命周期
onMounted(() => {
    loadStats()
})
</script>

<style lang="scss" scoped>
.stats-content {
    max-width: 1200px;
    margin: 0 auto;
    padding: 20px;
}

.stats-header {
    text-align: center;
    margin-bottom: 40px;
    
    .stats-title {
        font-size: 2.5rem;
        color: white;
        margin-bottom: 10px;
    }
    
    .stats-subtitle {
        font-size: 1.1rem;
        color: rgba(255, 255, 255, 0.8);
    }
}

.loading-container {
    text-align: center;
    padding: 60px 20px;
    color: white;
    
    .spinner {
        width: 50px;
        height: 50px;
        border: 4px solid rgba(255, 255, 255, 0.3);
        border-top: 4px solid white;
        border-radius: 50%;
        animation: spin 1s linear infinite;
        margin: 0 auto 20px;
    }
}

.stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 25px;
    margin-bottom: 40px;
}

.stat-card {
    background: var(--card-bg);
    border-radius: 16px;
    padding: 30px;
    display: flex;
    align-items: center;
    gap: 20px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
    transition: all 0.3s ease;
    
    &:hover {
        transform: translateY(-5px);
        box-shadow: 0 12px 40px rgba(0, 0, 0, 0.15);
    }
    
    .stat-icon {
        font-size: 3rem;
        width: 80px;
        height: 80px;
        display: flex;
        align-items: center;
        justify-content: center;
        background: linear-gradient(135deg, #667eea, #764ba2);
        border-radius: 50%;
        color: white;
        flex-shrink: 0;
    }
    
    .stat-content {
        flex: 1;
        
        .stat-number {
            font-size: 2rem;
            font-weight: bold;
            color: var(--text-color);
            margin-bottom: 5px;
        }
        
        .stat-label {
            font-size: 1rem;
            color: #666;
            margin: 0;
        }
    }
}

.detailed-stats {
    display: grid;
    gap: 30px;
}

.type-stats {
    display: grid;
    gap: 15px;
}

.type-item {
    display: grid;
    grid-template-columns: 1fr 2fr auto;
    gap: 15px;
    align-items: center;
    
    .type-info {
        display: flex;
        flex-direction: column;
        gap: 5px;
        
        .type-name {
            font-weight: 500;
            color: #333;
        }
        
        .type-count {
            font-size: 0.9rem;
            color: #666;
        }
    }
    
    .type-bar {
        height: 20px;
        background: #f0f0f0;
        border-radius: 10px;
        overflow: hidden;
        
        .type-fill {
            height: 100%;
            border-radius: 10px;
            transition: width 0.3s ease;
        }
    }
    
    .type-percentage {
        font-weight: 500;
        color: #333;
        min-width: 50px;
        text-align: right;
    }
}

.trend-chart {
    display: flex;
    justify-content: space-between;
    align-items: end;
    height: 200px;
    padding: 20px 0;
    gap: 10px;
}

.trend-bar {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    height: 100%;
    
    .trend-fill {
        width: 100%;
        max-width: 40px;
        background: linear-gradient(135deg, #667eea, #764ba2);
        border-radius: 4px 4px 0 0;
        transition: height 0.3s ease;
        margin-top: auto;
    }
    
    .trend-count {
        font-size: 0.9rem;
        font-weight: 500;
        color: var(--text-color);
        margin-top: 8px;
    }
    
    .trend-date {
        font-size: 0.8rem;
        color: #666;
        margin-top: 5px;
    }
}

.storage-info {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 20px;
}

.storage-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 15px 20px;
    background: var(--card-bg);
    border-radius: 8px;
    
    .storage-label {
        font-weight: 500;
        color: var(--text-color);
    }
    
    .storage-value {
        font-weight: 600;
        color: #667eea;
    }
}

.notification {
    position: fixed;
    top: 20px;
    right: 20px;
    padding: 15px 20px;
    border-radius: 6px;
    color: white;
    font-weight: 500;
    z-index: 1000;
    animation: slideIn 0.3s ease;
    
    &.success {
        background: #4caf50;
    }
    
    &.error {
        background: #f44336;
    }
}

@keyframes spin {
    0% { transform: rotate(0deg); }
    100% { transform: rotate(360deg); }
}

@keyframes slideIn {
    from {
        transform: translateX(100%);
        opacity: 0;
    }
    to {
        transform: translateX(0);
        opacity: 1;
    }
}

@media (max-width: 768px) {
    .stats-grid {
        grid-template-columns: 1fr;
        gap: 20px;
    }
    
    .stat-card {
        padding: 20px;
        
        .stat-icon {
            font-size: 2.5rem;
            width: 60px;
            height: 60px;
        }
        
        .stat-content .stat-number {
            font-size: 1.5rem;
        }
    }
    
    .type-item {
        grid-template-columns: 1fr;
        gap: 10px;
        
        .type-percentage {
            text-align: left;
        }
    }
    
    .trend-chart {
        height: 150px;
        gap: 5px;
    }
    
    .storage-info {
        grid-template-columns: 1fr;
    }
}
</style>