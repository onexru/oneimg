<template>
    <div class="home">
        <!-- 导航栏 -->
        <NavBar />
        
        <!-- 主要内容区域 -->
        <div class="main-content">
            <!-- 上传区域 -->
            <div class="upload-section">
                <h2 class="section-title">图片上传</h2>
                
                <!-- 拖拽上传区域 -->
                <div 
                    class="upload-area"
                    :class="{ 
                        'drag-over': isDragOver,
                        'uploading': isUploading 
                    }"
                    @drop="handleDrop"
                    @dragover="handleDragOver"
                    @dragenter="handleDragEnter"
                    @dragleave="handleDragLeave"
                    @click="triggerFileInput"
                >
                    <div v-if="!isUploading" class="upload-content">
                        <div class="upload-icon">📁</div>
                        <h3>拖拽图片到此处上传</h3>
                        <p>或者点击选择文件</p>
                        <p class="paste-tip">💡 提示：您也可以直接按 Ctrl+V 粘贴剪贴板中的图片</p>
                        <div class="supported-formats">
                            支持格式：JPG, PNG, GIF, WebP, SVG
                        </div>
                    </div>
                    
                    <!-- 上传进度 -->
                    <div v-else class="upload-progress">
                        <div class="spinner"></div>
                        <p>正在上传 {{ uploadingCount }} 个文件...</p>
                        <div class="progress-bar">
                            <div class="progress-fill" :style="{ width: uploadProgress + '%' }"></div>
                        </div>
                    </div>
                </div>
                
                <!-- 隐藏的文件输入 -->
                <input 
                    ref="fileInput"
                    type="file"
                    multiple
                    accept="image/*"
                    @change="handleFileSelect"
                    style="display: none"
                />
                
                <!-- 上传结果 -->
                <div v-if="uploadResults.length > 0" class="upload-results">
                    <h3>上传结果</h3>
                    <div class="results-grid">
                        <div 
                            v-for="(result, index) in uploadResults" 
                            :key="index"
                            class="result-item"
                            :class="{ 'success': result.success, 'error': !result.success }"
                        >
                            <div v-if="result.success" class="success-item">
                                <img :src="result.url" :alt="result.filename" class="result-image" />
                                <div class="result-info">
                                    <p class="filename">{{ result.filename }}</p>
                                    <p class="file-size">{{ formatFileSize(result.file_size) }}</p>
                                    <p class="dimensions">{{ result.width }} × {{ result.height }}</p>
                                    <div class="result-actions">
                                        <button @click="copyUrl(result.url)" class="copy-btn">复制链接</button>
                                        <button @click="previewImage(result)" class="preview-btn">预览</button>
                                    </div>
                                </div>
                            </div>
                            <div v-else class="error-item">
                                <div class="error-icon">❌</div>
                                <p class="error-message">{{ result.message }}</p>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            
            <!-- 最近上传的图片 -->
            <div class="recent-section">
                <h2 class="section-title">最近上传</h2>
                <div v-if="recentImages.length > 0" class="recent-grid">
                    <div 
                        v-for="image in recentImages" 
                        :key="image.id"
                        class="recent-item"
                        @click="previewImage(image)"
                    >
                        <img :src="image.url" :alt="image.filename" class="recent-image" />
                        <div class="recent-info">
                            <p class="recent-filename">{{ image.filename }}</p>
                            <p class="recent-date">{{ formatDate(image.created_at) }}</p>
                        </div>
                    </div>
                </div>
                <div v-else class="no-images">
                    <p>暂无上传的图片</p>
                </div>
            </div>
        </div>
        
        <!-- 图片预览模态框 -->
        <div v-if="previewModal.show" class="preview-modal" @click="closePreview">
            <div class="preview-content" @click.stop>
                <button class="close-btn" @click="closePreview">×</button>
                <img :src="previewModal.image.url" :alt="previewModal.image.filename" class="preview-img" />
                <div class="preview-info">
                    <h3>{{ previewModal.image.filename }}</h3>
                    <p>尺寸: {{ previewModal.image.width }} × {{ previewModal.image.height }}</p>
                    <p>大小: {{ formatFileSize(previewModal.image.file_size) }}</p>
                    <p>上传时间: {{ formatDate(previewModal.image.created_at) }}</p>
                    <div class="preview-actions">
                        <button @click="copyUrl(previewModal.image.url)" class="copy-btn">复制链接</button>
                        <button @click="downloadImage(previewModal.image)" class="download-btn">下载</button>
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
import { ref, onMounted, onUnmounted } from 'vue'
import NavBar from '@/components/NavBar.vue'

// 响应式数据
const isDragOver = ref(false)
const isUploading = ref(false)
const uploadingCount = ref(0)
const uploadProgress = ref(0)
const uploadResults = ref([])
const recentImages = ref([])
const fileInput = ref(null)

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

// 拖拽处理
const handleDragOver = (e) => {
    e.preventDefault()
    isDragOver.value = true
}

const handleDragEnter = (e) => {
    e.preventDefault()
    isDragOver.value = true
}

const handleDragLeave = (e) => {
    e.preventDefault()
    // 只有当离开整个拖拽区域时才设置为false
    if (!e.currentTarget.contains(e.relatedTarget)) {
        isDragOver.value = false
    }
}

const handleDrop = (e) => {
    e.preventDefault()
    isDragOver.value = false
    
    const files = Array.from(e.dataTransfer.files)
    const imageFiles = files.filter(file => file.type.startsWith('image/'))
    
    if (imageFiles.length > 0) {
        uploadFiles(imageFiles)
    } else {
        showNotification('请拖拽图片文件', 'error')
    }
}

// 文件选择处理
const triggerFileInput = () => {
    if (!isUploading.value) {
        fileInput.value.click()
    }
}

const handleFileSelect = (e) => {
    const files = Array.from(e.target.files)
    if (files.length > 0) {
        uploadFiles(files)
    }
    // 清空input值，允许重复选择同一文件
    e.target.value = ''
}

// 剪贴板粘贴处理
const handlePaste = async (e) => {
    const items = e.clipboardData.items
    const imageFiles = []
    
    for (let item of items) {
        if (item.type.startsWith('image/')) {
            const file = item.getAsFile()
            if (file) {
                // 给粘贴的文件一个默认名称
                const timestamp = new Date().getTime()
                const extension = item.type.split('/')[1] || 'png'
                const newFile = new File([file], `paste-${timestamp}.${extension}`, {
                    type: item.type
                })
                imageFiles.push(newFile)
            }
        }
    }
    
    if (imageFiles.length > 0) {
        e.preventDefault()
        uploadFiles(imageFiles)
        showNotification(`从剪贴板粘贴了 ${imageFiles.length} 个图片`, 'success')
    }
}

// 文件上传
const uploadFiles = async (files) => {
    if (isUploading.value) return
    
    isUploading.value = true
    uploadingCount.value = files.length
    uploadProgress.value = 0
    uploadResults.value = []
    
    const formData = new FormData()
    files.forEach(file => {
        formData.append('images[]', file)
    })
    
    try {
        // 模拟进度更新
        const progressInterval = setInterval(() => {
            if (uploadProgress.value < 90) {
                uploadProgress.value += Math.random() * 10
            }
        }, 200)
        
        const response = await fetch('/api/upload/images', {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            },
            body: formData
        })
        
        clearInterval(progressInterval)
        uploadProgress.value = 100
        
        const result = await response.json()
        
        if (response.ok) {
            uploadResults.value = result.data
            showNotification(result.message, 'success')
            // 刷新最近上传的图片
            await loadRecentImages()
        } else {
            throw new Error(result.message || '上传失败')
        }
    } catch (error) {
        console.error('上传错误:', error)
        showNotification('上传失败: ' + error.message, 'error')
    } finally {
        isUploading.value = false
        uploadingCount.value = 0
        uploadProgress.value = 0
    }
}

// 加载最近上传的图片
const loadRecentImages = async () => {
    try {
        const response = await fetch('/api/images?limit=12', {
            headers: {
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            }
        })
        
        if (response.ok) {
            const result = await response.json()
            recentImages.value = result.data.images || []
        }
    } catch (error) {
        console.error('加载图片失败:', error)
    }
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

const previewImage = (image) => {
    previewModal.value = {
        show: true,
        image: image
    }
}

const closePreview = () => {
    previewModal.value.show = false
}

const downloadImage = (image) => {
    const link = document.createElement('a')
    link.href = image.url
    link.download = image.filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
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
    // 添加全局粘贴事件监听
    document.addEventListener('paste', handlePaste)
    // 加载最近上传的图片
    loadRecentImages()
})

onUnmounted(() => {
    // 移除全局粘贴事件监听
    document.removeEventListener('paste', handlePaste)
})
</script>

<style lang="scss">
@use "@/styles/style.scss" as *;
.home {
    min-height: 100vh;
    // background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.main-content {
    max-width: 1200px;
    margin: 0 auto;
    padding: 20px;
}

.section-title {
    font-size: 1.8rem;
    color: var(--text-color);
    margin-bottom: 20px;
    text-align: center;
}

.upload-section {
    background: var(--card-bg);
    border-radius: 16px;
    padding: 30px;
    margin-bottom: 40px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
}

.upload-area {
    border: 3px dashed #ddd;
    border-radius: 12px;
    padding: 60px 20px;
    text-align: center;
    cursor: pointer;
    transition: all 0.3s ease;
    // background: #fafafa;
    
    &:hover {
        border-color: #667eea;
        // background: #f0f4ff;
    }
    
    &.drag-over {
        border-color: #667eea;
        // background: #e8f2ff;
        transform: scale(1.02);
    }
    
    &.uploading {
        cursor: not-allowed;
        opacity: 0.8;
    }
}

.upload-content {
    .upload-icon {
        font-size: 4rem;
        margin-bottom: 20px;
    }
    
    h3 {
        font-size: 1.5rem;
        color: var(--text-color);
        margin-bottom: 10px;
    }
    
    p {
        color: #666;
        margin-bottom: 10px;
    }
    
    .paste-tip {
        color: #667eea;
        font-weight: 500;
        margin-top: 20px;
    }
    
    .supported-formats {
        margin-top: 20px;
        padding: 10px;
        // background: #e8f2ff;
        border-radius: 6px;
        color: #667eea;
        font-size: 0.9rem;
    }
}

.upload-progress {
    .spinner {
        width: 40px;
        height: 40px;
        border: 4px solid #f3f3f3;
        border-top: 4px solid #667eea;
        border-radius: 50%;
        animation: spin 1s linear infinite;
        margin: 0 auto 20px;
    }
    
    p {
        color: #333;
        margin-bottom: 20px;
    }
    
    .progress-bar {
        width: 100%;
        height: 8px;
        // background: #f0f0f0;
        border-radius: 4px;
        overflow: hidden;
        
        .progress-fill {
            height: 100%;
            background: linear-gradient(90deg, #667eea, #764ba2);
            transition: width 0.3s ease;
        }
    }
}

.upload-results {
    margin-top: 30px;
    
    h3 {
        color: #333;
        margin-bottom: 20px;
    }
}

.results-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    gap: 20px;
}

.result-item {
    border-radius: 8px;
    overflow: hidden;
    
    &.success {
        border: 2px solid #4caf50;
    }
    
    &.error {
        border: 2px solid #f44336;
        // background: #ffebee;
        padding: 20px;
        text-align: center;
    }
}

.success-item {
    display: flex;
    // background: white;
    
    .result-image {
        width: 100px;
        height: 100px;
        object-fit: cover;
        flex-shrink: 0;
    }
    
    .result-info {
        padding: 15px;
        flex: 1;
        
        .filename {
            font-weight: 500;
            margin-bottom: 5px;
            word-break: break-all;
        }
        
        .file-size, .dimensions {
            font-size: 0.9rem;
            color: #666;
            margin-bottom: 5px;
        }
        
        .result-actions {
            margin-top: 10px;
            display: flex;
            gap: 10px;
            
            button {
                padding: 5px 10px;
                border: none;
                border-radius: 4px;
                cursor: pointer;
                font-size: 0.8rem;
                
                &.copy-btn {
                    background: #667eea;
                    color: white;
                }
                
                &.preview-btn {
                    // background: #f0f0f0;
                    color: #333;
                }
                
                &:hover {
                    opacity: 0.8;
                }
            }
        }
    }
}

.error-item {
    .error-icon {
        font-size: 2rem;
        margin-bottom: 10px;
    }
    
    .error-message {
        color: #d32f2f;
        font-weight: 500;
    }
}

.recent-section {
    // background: rgba(255, 255, 255, 0.95);
    border-radius: 16px;
    padding: 30px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
}

.recent-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 20px;
}

.recent-item {
    background: var(--card-bg);
    border-radius: 8px;
    overflow: hidden;
    cursor: pointer;
    transition: transform 0.2s ease;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    
    &:hover {
        transform: translateY(-2px);
        box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
    }
    
    .recent-image {
        width: 100%;
        height: 150px;
        object-fit: cover;
    }
    
    .recent-info {
        padding: 15px;
        
        .recent-filename {
            font-weight: 500;
            margin-bottom: 5px;
            word-break: break-all;
        }
        
        .recent-date {
            font-size: 0.9rem;
            color: #666;
        }
    }
}

.no-images {
    text-align: center;
    color: #666;
    padding: 40px;
}

.preview-modal {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.8);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1000;
    
    .preview-content {
        // background: white;
        border-radius: 12px;
        max-width: 90vw;
        max-height: 90vh;
        overflow: auto;
        position: relative;
        
        .close-btn {
            position: absolute;
            top: 10px;
            right: 10px;
            background: rgba(0, 0, 0, 0.5);
            color: white;
            border: none;
            border-radius: 50%;
            width: 40px;
            height: 40px;
            font-size: 1.5rem;
            cursor: pointer;
            z-index: 1001;
        }
        
        .preview-img {
            max-width: 100%;
            max-height: 70vh;
            display: block;
        }
        
        .preview-info {
            padding: 20px;
            background-color: var(--card-bg);
            
            h3 {
                margin-bottom: 10px;
            }
            
            p {
                margin-bottom: 5px;
                color: #666;
            }
            
            .preview-actions {
                margin-top: 15px;
                display: flex;
                gap: 10px;
                
                button {
                    padding: 8px 16px;
                    border: none;
                    border-radius: 6px;
                    cursor: pointer;
                    
                    &.copy-btn {
                        background: #667eea;
                        color: white;
                    }
                    
                    &.download-btn {
                        background: #4caf50;
                        color: white;
                    }
                    
                    &:hover {
                        opacity: 0.8;
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
    .main-content {
        padding: 10px;
    }
    
    .upload-area {
        padding: 40px 15px;
    }
    
    .results-grid,
    .recent-grid {
        grid-template-columns: 1fr;
    }
    
    .success-item {
        flex-direction: column;
        
        .result-image {
            width: 100%;
            height: 200px;
        }
    }
}
</style>