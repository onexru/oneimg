<template>
    <div class="gallery">
        <!-- 导航栏 -->
        <NavBar />
        
        <!-- 主要内容 -->
        <div class="gallery-content">
            <!-- 头部工具栏 -->
            <div class="gallery-header">
                <h1 class="gallery-title">图片画廊</h1>
                
                <!-- 搜索和筛选 -->
                <div class="gallery-controls">
                    <div class="search-box">
                        <input 
                            v-model="searchQuery"
                            type="text" 
                            placeholder="搜索图片..."
                            class="search-input"
                            @input="handleSearch"
                        />
                        <span class="search-icon">🔍</span>
                    </div>
                    
                    <div class="filter-controls">
                        <select v-model="sortBy" @change="loadImages" class="sort-select">
                            <option value="created_at">按时间排序</option>
                            <option value="file_size">按大小排序</option>
                            <option value="filename">按名称排序</option>
                        </select>
                        
                        <select v-model="sortOrder" @change="loadImages" class="sort-select">
                            <option value="desc">降序</option>
                            <option value="asc">升序</option>
                        </select>
                    </div>
                    
                    <!-- 视图切换 -->
                    <div class="view-controls">
                        <button 
                            @click="viewMode = 'grid'"
                            :class="{ active: viewMode === 'grid' }"
                            class="view-btn"
                        >
                            ⊞ 网格
                        </button>
                        <button 
                            @click="viewMode = 'list'"
                            :class="{ active: viewMode === 'list' }"
                            class="view-btn"
                        >
                            ☰ 列表
                        </button>
                    </div>
                </div>
            </div>
            
            <!-- 加载状态 -->
            <div v-if="loading" class="loading-container">
                <div class="spinner"></div>
                <p>加载中...</p>
            </div>
            
            <!-- 图片网格/列表 -->
            <div v-else-if="images.length > 0" class="images-container">
                <!-- 网格视图 -->
                <div v-if="viewMode === 'grid'" class="images-grid">
                    <div 
                        v-for="image in images" 
                        :key="image.id"
                        class="image-card"
                        @click="openPreview(image)"
                    >
                        <div class="image-wrapper">
                            <img 
                                :src="image.url" 
                                :alt="image.filename"
                                class="image-thumbnail"
                                @error="handleImageError"
                            />
                            <div class="image-overlay">
                                <div class="overlay-actions">
                                    <button @click.stop="copyUrl(image.url)" class="action-btn copy-btn">
                                        📋
                                    </button>
                                    <button @click.stop="downloadImage(image)" class="action-btn download-btn">
                                        ⬇️
                                    </button>
                                    <button @click.stop="deleteImage(image)" class="action-btn delete-btn">
                                        🗑️
                                    </button>
                                </div>
                            </div>
                        </div>
                        <div class="image-info">
                            <p class="image-filename">{{ image.filename }}</p>
                            <p class="image-meta">
                                {{ formatFileSize(image.file_size) }} • 
                                {{ image.width }}×{{ image.height }}
                            </p>
                            <p class="image-date">{{ formatDate(image.created_at) }}</p>
                        </div>
                    </div>
                </div>
                
                <!-- 列表视图 -->
                <div v-else class="images-list">
                    <div class="list-header">
                        <div class="col-thumbnail">预览</div>
                        <div class="col-filename">文件名</div>
                        <div class="col-size">大小</div>
                        <div class="col-dimensions">尺寸</div>
                        <div class="col-date">上传时间</div>
                        <div class="col-actions">操作</div>
                    </div>
                    
                    <div 
                        v-for="image in images" 
                        :key="image.id"
                        class="list-item"
                    >
                        <div class="col-thumbnail">
                            <img 
                                :src="image.url" 
                                :alt="image.filename"
                                class="list-thumbnail"
                                @click="openPreview(image)"
                            />
                        </div>
                        <div class="col-filename">
                            <span class="filename-text">{{ image.filename }}</span>
                        </div>
                        <div class="col-size">{{ formatFileSize(image.file_size) }}</div>
                        <div class="col-dimensions">{{ image.width }}×{{ image.height }}</div>
                        <div class="col-date">{{ formatDate(image.created_at) }}</div>
                        <div class="col-actions">
                            <button @click="copyUrl(image.url)" class="list-action-btn" title="复制链接">
                                📋
                            </button>
                            <button @click="downloadImage(image)" class="list-action-btn" title="下载">
                                ⬇️
                            </button>
                            <button @click="deleteImage(image)" class="list-action-btn delete" title="删除">
                                🗑️
                            </button>
                        </div>
                    </div>
                </div>
                
                <!-- 分页 -->
                <div v-if="totalPages > 1" class="pagination">
                    <button 
                        @click="changePage(currentPage - 1)"
                        :disabled="currentPage <= 1"
                        class="page-btn"
                    >
                        ← 上一页
                    </button>
                    
                    <div class="page-numbers">
                        <button 
                            v-for="page in visiblePages"
                            :key="page"
                            @click="changePage(page)"
                            :class="{ active: page === currentPage }"
                            class="page-number"
                        >
                            {{ page }}
                        </button>
                    </div>
                    
                    <button 
                        @click="changePage(currentPage + 1)"
                        :disabled="currentPage >= totalPages"
                        class="page-btn"
                    >
                        下一页 →
                    </button>
                </div>
            </div>
            
            <!-- 空状态 -->
            <div v-else class="empty-state">
                <div class="empty-icon">📷</div>
                <h3>暂无图片</h3>
                <p>还没有上传任何图片，<router-link to="/">去上传一些吧</router-link></p>
            </div>
        </div>
        
        <!-- 图片预览模态框 -->
        <div v-if="previewModal.show" class="preview-modal" @click="closePreview">
            <div class="preview-content" @click.stop>
                <button class="close-btn" @click="closePreview">×</button>
                <img 
                    :src="previewModal.image.url" 
                    :alt="previewModal.image.filename" 
                    class="preview-image"
                />
                <div class="preview-info">
                    <h3>{{ previewModal.image.filename }}</h3>
                    <div class="preview-details">
                        <p><strong>尺寸:</strong> {{ previewModal.image.width }} × {{ previewModal.image.height }}</p>
                        <p><strong>大小:</strong> {{ formatFileSize(previewModal.image.file_size) }}</p>
                        <p><strong>类型:</strong> {{ previewModal.image.mime_type }}</p>
                        <p><strong>上传时间:</strong> {{ formatDate(previewModal.image.created_at) }}</p>
                    </div>
                    <div class="preview-actions">
                        <button @click="copyUrl(previewModal.image.url)" class="preview-action-btn">
                            📋 复制链接
                        </button>
                        <button @click="downloadImage(previewModal.image)" class="preview-action-btn">
                            ⬇️ 下载
                        </button>
                        <button @click="deleteImage(previewModal.image)" class="preview-action-btn delete">
                            🗑️ 删除
                        </button>
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
import { ref, onMounted, computed } from 'vue'
import NavBar from '@/components/NavBar.vue'

// 响应式数据
const images = ref([])
const loading = ref(false)
const searchQuery = ref('')
const sortBy = ref('created_at')
const sortOrder = ref('desc')
const viewMode = ref('grid')
const currentPage = ref(1)
const totalPages = ref(1)
const pageSize = ref(20)

// 预览模态框
const previewModal = ref({
    show: false,
    image: null
})

// 通知消息
const notification = ref({
    show: false,
    message: '',
    type: 'success'
})

// 计算属性
const visiblePages = computed(() => {
    const pages = []
    const start = Math.max(1, currentPage.value - 2)
    const end = Math.min(totalPages.value, currentPage.value + 2)
    
    for (let i = start; i <= end; i++) {
        pages.push(i)
    }
    
    return pages
})

// 加载图片列表
const loadImages = async () => {
    loading.value = true
    
    try {
        const params = new URLSearchParams({
            page: currentPage.value,
            limit: pageSize.value,
            sort_by: sortBy.value,
            sort_order: sortOrder.value
        })
        
        if (searchQuery.value) {
            params.append('search', searchQuery.value)
        }
        
        const response = await fetch(`/api/images?${params}`, {
            headers: {
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            }
        })
        
        if (response.ok) {
            const result = await response.json()
            images.value = result.data.images || []
            totalPages.value = result.data.total_pages || 1
        } else {
            throw new Error('加载图片失败')
        }
    } catch (error) {
        console.error('加载图片错误:', error)
        showNotification('加载图片失败: ' + error.message, 'error')
    } finally {
        loading.value = false
    }
}

// 搜索处理
let searchTimeout = null
const handleSearch = () => {
    clearTimeout(searchTimeout)
    searchTimeout = setTimeout(() => {
        currentPage.value = 1
        loadImages()
    }, 500)
}

// 分页处理
const changePage = (page) => {
    if (page >= 1 && page <= totalPages.value) {
        currentPage.value = page
        loadImages()
        // 滚动到顶部
        window.scrollTo({ top: 0, behavior: 'smooth' })
    }
}

// 图片预览
const openPreview = (image) => {
    previewModal.value = {
        show: true,
        image: image
    }
}

const closePreview = () => {
    previewModal.value.show = false
}

// 复制链接
const copyUrl = async (url) => {
    try {
        const fullUrl = window.location.origin + url
        await navigator.clipboard.writeText(fullUrl)
        showNotification('链接已复制到剪贴板', 'success')
    } catch (error) {
        // 降级方案
        const textArea = document.createElement('textarea')
        textArea.value = window.location.origin + url
        document.body.appendChild(textArea)
        textArea.select()
        document.execCommand('copy')
        document.body.removeChild(textArea)
        showNotification('链接已复制到剪贴板', 'success')
    }
}

// 下载图片
const downloadImage = (image) => {
    const link = document.createElement('a')
    link.href = image.url
    link.download = image.filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
}

// 删除图片
const deleteImage = async (image) => {
    if (!confirm(`确定要删除图片 "${image.filename}" 吗？`)) {
        return
    }
    
    try {
        const response = await fetch(`/api/images/${image.id}`, {
            method: 'DELETE',
            headers: {
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            }
        })
        
        if (response.ok) {
            showNotification('图片删除成功', 'success')
            // 关闭预览模态框（如果打开的话）
            if (previewModal.value.show && previewModal.value.image.id === image.id) {
                closePreview()
            }
            // 重新加载图片列表
            loadImages()
        } else {
            const result = await response.json()
            throw new Error(result.message || '删除失败')
        }
    } catch (error) {
        console.error('删除图片错误:', error)
        showNotification('删除图片失败: ' + error.message, 'error')
    }
}

// 图片加载错误处理
const handleImageError = (event) => {
    event.target.src = 'data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjAwIiBoZWlnaHQ9IjIwMCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48cmVjdCB3aWR0aD0iMTAwJSIgaGVpZ2h0PSIxMDAlIiBmaWxsPSIjZGRkIi8+PHRleHQgeD0iNTAlIiB5PSI1MCUiIGZvbnQtZmFtaWx5PSJBcmlhbCIgZm9udC1zaXplPSIxNCIgZmlsbD0iIzk5OSIgdGV4dC1hbmNob3I9Im1pZGRsZSIgZHk9Ii4zZW0iPuWbvueJh+WKoOi9veWksei0pTwvdGV4dD48L3N2Zz4='
}

// 工具函数
const formatFileSize = (bytes) => {
    if (!bytes) return '0 B'
    const k = 1024
    const sizes = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

const formatDate = (dateString) => {
    if (!dateString) return ''
    const date = new Date(dateString)
    return date.toLocaleString('zh-CN')
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
    loadImages()
})
</script>

<style lang="scss" scoped>
.gallery-content {
    max-width: 1400px;
    margin: 0 auto;
    padding: 20px;
}

.gallery-header {
    background: var(--card-bg);
    border-radius: 16px;
    padding: 30px;
    margin-bottom: 30px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
    
    .gallery-title {
        font-size: 2rem;
        color: var(--text-color);
        margin-bottom: 20px;
        text-align: center;
    }
}

.gallery-controls {
    display: flex;
    flex-wrap: wrap;
    gap: 20px;
    align-items: center;
    justify-content: center;
    
    .search-box {
        position: relative;
        
        .search-input {
            color: var(--text-color);
            background: none;
            padding: 10px 40px 10px 15px;
            border: 2px solid #ddd;
            border-radius: 8px;
            font-size: 1rem;
            width: 250px;
            
            &:focus {
                outline: none;
                border-color: #667eea;
            }
        }
        
        .search-icon {
            position: absolute;
            right: 12px;
            top: 50%;
            transform: translateY(-50%);
            color: #666;
        }
    }
    
    .filter-controls {
        display: flex;
        gap: 10px;
        
        .sort-select {
            color: var(--text-color);
            padding: 10px 15px;
            border: 2px solid #ddd;
            border-radius: 8px;
            font-size: 1rem;
            background: var(--card-bg);
            
            &:focus {
                outline: none;
                border-color: #667eea;
            }
        }
    }
    
    .view-controls {
        display: flex;
        gap: 5px;

        .view-btn {
            color: var(--text-color);
            padding: 10px 15px;
            border: 2px solid #ddd;
            background: var(--card-bg);
            border-radius: 8px;
            cursor: pointer;
            transition: all 0.3s ease;
            
            &:hover {
                border-color: #667eea;
            }
            
            &.active {
                background: #667eea;
                border-color: #667eea;
            }
        }
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

.images-container {
    background: var(--card-bg);
    border-radius: 16px;
    padding: 30px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
}

.images-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 25px;
}

.image-card {
    background: var(--card-bg);
    border-radius: 12px;
    overflow: hidden;
    box-shadow: 0 4px 15px rgba(0, 0, 0, 0.1);
    transition: all 0.3s ease;
    cursor: pointer;
    
    &:hover {
        transform: translateY(-5px);
        box-shadow: 0 8px 25px rgba(0, 0, 0, 0.15);
        
        .image-overlay {
            opacity: 1;
        }
    }
    
    .image-wrapper {
        position: relative;
        height: 200px;
        overflow: hidden;
        
        .image-thumbnail {
            width: 100%;
            height: 100%;
            object-fit: cover;
            transition: transform 0.3s ease;
        }
        
        .image-overlay {
            position: absolute;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0, 0, 0, 0.7);
            display: flex;
            justify-content: center;
            align-items: center;
            opacity: 0;
            transition: opacity 0.3s ease;
            
            .overlay-actions {
                display: flex;
                gap: 10px;
                
                .action-btn {
                    width: 40px;
                    height: 40px;
                    border: none;
                    border-radius: 50%;
                    background: rgba(255, 255, 255, 0.9);
                    cursor: pointer;
                    font-size: 1.2rem;
                    transition: all 0.2s ease;
                    
                    &:hover {
                        background: white;
                        transform: scale(1.1);
                    }
                    
                    &.delete-btn:hover {
                        background: #ff4444;
                        color: white;
                    }
                }
            }
        }
    }
    
    .image-info {
        padding: 15px;
        
        .image-filename {
            font-weight: 500;
            margin-bottom: 5px;
            word-break: break-all;
            color: var(--text-color);
        }
        
        .image-meta {
            font-size: 0.9rem;
            color: var(--text-color);
            margin-bottom: 5px;
        }
        
        .image-date {
            font-size: 0.8rem;
            color: var(--text-color);
        }
    }
}

.images-list {
    .list-header {
        display: grid;
        grid-template-columns: 80px 1fr 100px 100px 150px 120px;
        gap: 15px;
        padding: 15px;
        background: var(--card-bg);;
        border-radius: 8px;
        font-weight: 600;
        color: var(--text-color);
        margin-bottom: 10px;
    }
    
    .list-item {
        display: grid;
        grid-template-columns: 80px 1fr 100px 100px 150px 120px;
        gap: 15px;
        padding: 15px;
        background: var(--card-bg);;
        border-radius: 8px;
        margin-bottom: 10px;
        align-items: center;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
        transition: all 0.2s ease;
        
        &:hover {
            box-shadow: 0 4px 15px rgba(0, 0, 0, 0.1);
        }
        
        .list-thumbnail {
            width: 60px;
            height: 60px;
            object-fit: cover;
            border-radius: 6px;
            cursor: pointer;
        }
        
        .filename-text {
            word-break: break-all;
            color: var(--text-color);
        }
        
        .col-actions {
            display: flex;
            gap: 8px;
            
            .list-action-btn {
                width: 32px;
                height: 32px;
                border: none;
                border-radius: 6px;
                background: #f0f0f0;
                cursor: pointer;
                font-size: 0.9rem;
                transition: all 0.2s ease;
                
                &:hover {
                    background: #e0e0e0;
                }
                
                &.delete:hover {
                    background: #ff4444;
                    color: white;
                }
            }
        }
    }
}

.pagination {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 10px;
    margin-top: 30px;
    
    .page-btn {
        padding: 10px 20px;
        border: 2px solid #ddd;
        background: var(--card-bg);;
        border-radius: 8px;
        cursor: pointer;
        transition: all 0.3s ease;
        
        &:hover:not(:disabled) {
            border-color: #667eea;
            color: #667eea;
        }
        
        &:disabled {
            opacity: 0.5;
            cursor: not-allowed;
        }
    }
    
    .page-numbers {
        display: flex;
        gap: 5px;
        
        .page-number {
            width: 40px;
            height: 40px;
            border: 2px solid #ddd;
            background: var(--card-bg);;
            border-radius: 8px;
            cursor: pointer;
            transition: all 0.3s ease;
            
            &:hover {
                border-color: #667eea;
                color: #667eea;
            }
            
            &.active {
                background: #667eea;
                color: white;
                border-color: #667eea;
            }
        }
    }
}

.empty-state {
    text-align: center;
    padding: 80px 20px;
    color: white;
    
    .empty-icon {
        font-size: 4rem;
        margin-bottom: 20px;
    }
    
    h3 {
        font-size: 1.5rem;
        margin-bottom: 10px;
    }
    
    p {
        font-size: 1rem;
        opacity: 0.8;
        
        a {
            color: #fff;
            text-decoration: underline;
        }
    }
}

.preview-modal {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.9);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1000;
    
    .preview-content {
        background: var(--card-bg);;
        border-radius: 12px;
        max-width: 90vw;
        max-height: 90vh;
        overflow: auto;
        position: relative;
        
        .close-btn {
            position: absolute;
            top: 15px;
            right: 15px;
            background: rgba(0, 0, 0, 0.7);
            color: white;
            border: none;
            border-radius: 50%;
            width: 40px;
            height: 40px;
            font-size: 1.5rem;
            cursor: pointer;
            z-index: 1001;
        }
        
        .preview-image {
            max-width: 100%;
            max-height: 70vh;
            display: block;
        }
        
        .preview-info {
            padding: 20px;
            
            h3 {
                margin-bottom: 15px;
                color: #333;
            }
            
            .preview-details {
                margin-bottom: 20px;
                
                p {
                    margin-bottom: 8px;
                    color: #666;
                }
            }
            
            .preview-actions {
                display: flex;
                gap: 10px;
                flex-wrap: wrap;
                
                .preview-action-btn {
                    padding: 10px 20px;
                    border: none;
                    border-radius: 6px;
                    cursor: pointer;
                    font-size: 0.9rem;
                    transition: all 0.2s ease;
                    
                    &:not(.delete) {
                        background: #667eea;
                        color: white;
                        
                        &:hover {
                            background: #5a6fd8;
                        }
                    }
                    
                    &.delete {
                        background: #ff4444;
                        color: white;
                        
                        &:hover {
                            background: #e63939;
                        }
                    }
                }
            }
        }
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
    .gallery-controls {
        flex-direction: column;
        align-items: stretch;
        
        .search-box .search-input {
            background: none;
            width: 100%;
        }
        
        .filter-controls,
        .view-controls {
            justify-content: center;
        }
    }
    
    .images-grid {
        grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
        gap: 20px;
    }
    
    .list-header,
    .list-item {
        grid-template-columns: 60px 1fr 80px 100px;
        
        .col-size,
        .col-dimensions {
            display: none;
        }
    }
    
    .pagination {
        flex-wrap: wrap;
        
        .page-numbers {
            order: -1;
            width: 100%;
            justify-content: center;
        }
    }
}
</style>