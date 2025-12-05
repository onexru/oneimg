<template>
    <div class="text-gray-800 dark:text-gray-200">
        <!-- 主要内容 -->
        <div class="gallery-content container mx-auto px-4 py-8">
            <!-- 加载状态 -->
            <div v-if="loading" class="loading-container flex flex-col items-center justify-center py-20">
                <div class="spinner w-10 h-10 border-4 border-gray-200 dark:border-gray-700 border-t-primary dark:border-t-primary rounded-full animate-spin mb-4"></div>
                <p class="text-gray-600 dark:text-gray-400">加载中...</p>
            </div>
            
            <!-- 图片网格/列表 -->
            <div v-else-if="images.length > 0" class="images-container">
                <!-- 网格视图 -->
                <div v-if="viewMode === 'grid'" class="images-grid grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
                    <div 
                        v-for="image in images" 
                        :key="image.id"
                        class="image-card bg-white dark:bg-gray-800 rounded-xl shadow-md overflow-hidden hover:shadow-lg transition-all duration-300 cursor-pointer"
                        @click="openPreview(image)"
                    >
                        <div class="image-wrapper relative aspect-video overflow-hidden bg-gray-100 dark:bg-gray-900">
                            <img 
                                :src="image.thumbnail || image.url" 
                                :alt="image.filename"
                                class="image-thumbnail w-full h-full object-cover transition-transform duration-500 hover:scale-105"
                                @error="handleImageError"
                            />
                        </div>
                        <div class="image-info p-3">
                            <p class="image-filename font-medium text-sm truncate whitespace-nowrap overflow-hidden">{{ image.filename }}</p>
                            <p class="image-meta text-xs text-gray-500 dark:text-gray-400 mt-1">
                                {{ formatFileSize(image.file_size) }} • 
                                {{ image.width }}×{{ image.height }}
                            </p>
                            <p class="image-date text-xs text-gray-500 dark:text-gray-400 mt-1">{{ formatDate(image.created_at) }}</p>
                        </div>
                    </div>
                </div>
                
                <!-- 分页 -->
                <div v-if="totalPages > 1" class="pagination flex flex-wrap items-center justify-center gap-2 py-8">
                    <button 
                        @click="changePage(currentPage - 1)"
                        :disabled="currentPage <= 1"
                        class="page-btn px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700 transition-all text-sm"
                        :class="{ 'opacity-50 cursor-not-allowed': currentPage <= 1 }"
                    >
                        上一页
                    </button>
                    
                    <div class="page-numbers flex gap-1">
                        <button 
                            v-for="page in visiblePages"
                            :key="page"
                            @click="changePage(page)"
                            class="w-9 h-9 flex items-center justify-center rounded-lg border transition-all text-sm"
                            :class="[
                                page === currentPage 
                                    ? 'bg-primary text-white border-primary' 
                                    : 'border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700'
                            ]"
                        >
                            {{ page }}
                        </button>
                    </div>
                    
                    <button 
                        @click="changePage(currentPage + 1)"
                        :disabled="currentPage >= totalPages"
                        class="page-btn px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700 transition-all text-sm"
                        :class="{ 'opacity-50 cursor-not-allowed': currentPage >= totalPages }"
                    >
                        下一页
                    </button>
                </div>
            </div>
            
            <!-- 空状态 -->
            <div v-else class="empty-state flex flex-col items-center justify-center py-20 text-center">
                <div class="empty-icon text-6xl mb-4 text-gray-400 dark:text-gray-600">
                    <i class="ri-image-ai-line"></i>
                </div>
                <h3 class="text-xl font-bold mb-2">暂无图片</h3>
                <p class="text-gray-600 dark:text-gray-400 mb-6">还没有上传任何图片，<router-link to="/" class="text-primary hover:underline">去上传一些吧</router-link></p>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, onMounted, computed, onUnmounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'

const getFullUrl = (path) => {
  if (!path) return ''
  if (typeof window !== 'undefined') {
    return window.location.origin + path
  }
  return path
}

// 响应式数据（仅保留必要项）
const images = ref([])
const loading = ref(false)
const viewMode = ref('grid')
const currentPage = ref(1)
const totalPages = ref(1)
const pageSize = ref(20)
const notification = ref({ show: false, message: '', type: 'success' })

// 当前预览的图片
const currentPreviewImage = ref(null)

// 计算属性（分页显示）
const visiblePages = computed(() => {
    const pages = []
    const start = Math.max(1, currentPage.value - 2)
    const end = Math.min(totalPages.value, currentPage.value + 2)
    
    for (let i = start; i <= end; i++) {
        pages.push(i)
    }
    
    return pages
})

// 路由实例
const router = useRouter()

// 加载图片列表（核心功能）
const loadImages = async () => {
    loading.value = true
    
    try {
        const params = new URLSearchParams({
            page: currentPage.value,
            limit: pageSize.value,
            sort_by: 'created_at', // 固定默认排序
            sort_order: 'desc'
        })
        
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
            // 未授权跳转登录页
            if (response.status === 401) {
                localStorage.removeItem('authToken')
                router.push('/login')
                Message.error('登录已过期，请重新登录')
                return
            }
            throw new Error('加载图片失败')
        }
    } catch (error) {
        console.error('加载图片错误:', error)
        Message.error('加载图片失败: ' + error.message)
    } finally {
        loading.value = false
    }
}

// 分页处理
const changePage = (page) => {
    if (page >= 1 && page <= totalPages.value) {
        currentPage.value = page
        loadImages()
        window.scrollTo({ top: 0, behavior: 'smooth' })
    }
}

// 图片预览（核心功能）
const openPreview = (image) => {
    currentPreviewImage.value = image // 存储当前预览图片
    
    // 创建预览弹窗（假设 PopupModal 是全局可用的工具类）
    const customModal = new PopupModal({
        title: '图片预览',
        content: `
            <div class="image-preview-popup w-full max-w-5xl max-h-[85vh] flex flex-col overflow-hidden bg-white dark:bg-dark-200">
                <!-- 顶部操作栏 -->
                <div class="preview-header bg-light-50 pb-2 flex justify-between items-center">
                    <h3 class="text-xs font-medium truncate max-w-[50%]">${image.filename}</h3>
                    <div class="flex gap-1">
                        <!-- 复制按钮 -->
                        <div class="relative z-100">
                            <button 
                                class="px-3 py-1.5 text-xs bg-primary/10 hover:bg-primary/20 whitespace-nowrap text-primary rounded-md transition-colors duration-200 flex items-center gap-1"
                                onclick="event.stopPropagation(); window.togglePreviewCopyMenu()"
                            >
                                <i class="ri-file-copy-line"></i>
                                复制
                                <i class="ri-arrow-down-s-line text-[10px] ml-0.5" id="copyMenuIcon"></i>
                            </button>
                            <!-- 复制下拉框 -->
                            <div 
                                class="absolute right-0 mt-1 w-36 bg-white dark:bg-dark-200 rounded-md shadow-xl z-101 transition-all duration-200 hidden opacity-0 translate-y-[-5px]"
                                id="previewCopyDropdown"
                            >
                                <div class="p-1">
                                    <button 
                                        class="w-full text-left px-3 py-2 text-xs text-gray-800 dark:text-light-100 hover:bg-light-100 dark:hover:bg-dark-300 rounded transition-colors duration-200 flex items-center gap-2"
                                        onclick="event.stopPropagation(); window.copyPreviewImageLink('url')"
                                    >
                                        <i class="ri-link text-xs w-4 text-center"></i>
                                        URL
                                    </button>
                                    <button 
                                        class="w-full text-left px-3 py-2 text-xs text-gray-800 dark:text-light-100 hover:bg-light-100 dark:hover:bg-dark-300 rounded transition-colors duration-200 flex items-center gap-2"
                                        onclick="event.stopPropagation(); window.copyPreviewImageLink('html')"
                                    >
                                        <i class="ri-code-fill text-xs w-4 text-center"></i>
                                        HTML
                                    </button>
                                    <button 
                                        class="w-full text-left px-3 py-2 text-xs text-gray-800 dark:text-light-100 hover:bg-light-100 dark:hover:bg-dark-300 rounded transition-colors duration-200 flex items-center gap-2"
                                        onclick="event.stopPropagation(); window.copyPreviewImageLink('markdown')"
                                    >
                                        <i class="ri-markdown-fill text-xs w-4 text-center"></i>
                                        MD
                                    </button>
                                </div>
                            </div>
                        </div>
                        <!-- 下载按钮 -->
                        <button 
                            class="px-3 py-1.5 text-xs bg-light-100 dark:bg-dark-300 hover:bg-light-200 whitespace-nowrap dark:hover:bg-dark-400 text-secondary rounded-md transition-colors duration-200 flex items-center gap-1"
                            onclick="event.stopPropagation(); window.downloadPreviewImage()"
                        >
                            <i class="ri-download-fill text-xs"></i>
                            下载
                        </button>
                        <!-- 删除按钮 -->
                        <button 
                            class="px-3 py-1.5 text-xs bg-danger/10 hover:bg-danger/20 whitespace-nowrap text-danger rounded-md transition-colors duration-200 flex items-center gap-1"
                            onclick="event.stopPropagation(); window.deletePreviewImage(${image.id})"
                        >
                            <i class="ri-delete-bin-fill text-xs"></i>
                            删除
                        </button>
                    </div>
                </div>
                
                <!-- 预览图片区域 -->
                <div class="max-h-[360px] flex-1 overflow-auto flex items-center justify-center">
                    <a class="spotlight" href="${image.url}" data-description="尺寸: ${image.width || '未知'}×${image.height || '未知'} | 大小: ${formatFileSize(image.file_size || 0)} | 上传日期：${formatDate(image.created_at)}">
                        <img 
                            src="${image.url}"
                            alt="${image.filename}" 
                            class="max-w-full w-fill max-h-[360px] object-contain rounded-lg"
                        />
                    </a>
                </div>
                
                <!-- 底部信息栏 -->
                <div class="pt-2 flex flex-wrap gap-2 text-xs text-secondary">
                    <div class="flex items-center gap-1.5">
                        <i class="ri-ruler-line w-3.5 text-center"></i>
                        尺寸: ${image.width || '未知'}×${image.height || '未知'}
                    </div>
                    <div class="flex items-center gap-1.5">
                        <i class="ri-image-line w-3.5 text-center"></i>
                        大小: ${formatFileSize(image.file_size || 0)}
                    </div>
                    <div class="flex items-center gap-1.5">
                        <i class="ri-hard-drive-3-line"></i>
                        存储: ${(image.storage === 'default' ? '本地' : image.storage) || '未知'}
                    </div>
                </div>
            </div>
        `,
        type: 'default',
        buttons: [
            {
                text: '确定',
                type: 'default',
                callback: (modal) => {
                    modal.close()
                    // 清理全局函数和DOM
                    delete window.togglePreviewCopyMenu
                    delete window.copyPreviewImageLink
                    delete window.downloadPreviewImage
                    delete window.deletePreviewImage
                }
            }
        ],
        maskClose: true,
        zIndex: 10000,
        maxHeight: '90vh'
    });

    // 注册弹窗内操作函数（避免全局污染，关闭时清理）
    window.togglePreviewCopyMenu = () => {
        const dropdown = document.getElementById('previewCopyDropdown')
        const icon = document.getElementById('copyMenuIcon')
        if (dropdown && icon) {
            const isHidden = dropdown.classList.contains('hidden')
            if (isHidden) {
                dropdown.classList.remove('hidden', 'opacity-0', 'translate-y-[-5px]')
                dropdown.classList.add('block', 'opacity-100', 'translate-y-0')
                icon.classList.add('rotate-180')
            } else {
                dropdown.classList.add('hidden', 'opacity-0', 'translate-y-[-5px]')
                dropdown.classList.remove('block', 'opacity-100', 'translate-y-0')
                icon.classList.remove('rotate-180')
            }
        }
    }

    window.copyPreviewImageLink = (type) => copyImageLink(type)
    window.downloadPreviewImage = () => downloadImage()
    window.deletePreviewImage = async (id) => {
        customModal.close()
        await deleteImage(id)
    }

    customModal.open();
}

// 复制图片链接（预览弹窗内使用）
const copyImageLink = async (type) => {
    if (!currentPreviewImage.value) return
    const image = currentPreviewImage.value
    const fullUrl = getFullUrl(image.url) // 假设后端返回的已是完整URL，如需拼接可自行处理
    let copyText = ''
    
    switch (type) {
        case 'url':
            copyText = fullUrl
            break
        case 'html':
            copyText = `<img src="${fullUrl}" alt="${image.filename}" width="${image.width || ''}" height="${image.height || ''}">`
            break
        case 'markdown':
            copyText = `![${image.filename}](${fullUrl})`
            break
        default:
            copyText = fullUrl
    }
    
    try {
        await navigator.clipboard.writeText(copyText)
        Message.success(`已复制${type.toUpperCase()}格式链接`)
    } catch (error) {
        // 降级处理
        const textArea = document.createElement('textarea')
        textArea.value = copyText
        document.body.appendChild(textArea)
        textArea.select()
        document.execCommand('copy')
        document.body.removeChild(textArea)
        Message.success(`已复制${type.toUpperCase()}格式链接`)
    } finally {
        // 关闭下拉框
        nextTick(() => {
            const dropdown = document.getElementById('previewCopyDropdown')
            const icon = document.getElementById('copyMenuIcon')
            if (dropdown && icon) {
                dropdown.classList.add('hidden', 'opacity-0', 'translate-y-[-5px]')
                dropdown.classList.remove('block', 'opacity-100', 'translate-y-0')
                icon.classList.remove('rotate-180')
            }
        })
    }
}

// 下载图片（预览弹窗内使用）
const downloadImage = () => {
    if (!currentPreviewImage.value) return
    const image = currentPreviewImage.value
    const link = document.createElement('a')
    link.href = image.url
    link.download = image.filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    Message.success('下载已开始')
}

// 快捷删除图片功能（保留确认弹窗）
const deleteImage = async (imageId) => {
  // 保留需要用户确认的弹窗（PopupModal）
  const modal = new PopupModal({
    title: '确认删除',
    content: `
      <div class="flex gap-3">
        <i class="fa fa-exclamation-triangle text-warning text-xl mt-1"></i>
        <div>
          <p>确定要删除这张图片吗？</p>
          <p class="mt-1 text-secondary text-sm">删除后无法恢复，请谨慎操作</p>
        </div>
      </div>
    `,
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close()
      },
      {
        text: '确认删除',
        type: 'danger',
        callback: async (modal) => {
          modal.close()
          await deleteAsync(imageId)
        }
      }
    ],
    maskClose: true
  })
  modal.open()
}

// 删除图片（预览弹窗内使用）
const deleteAsync = async (id) => {    
    try {
        const response = await fetch(`/api/images/${id}`, {
            method: 'DELETE',
            headers: {
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            }
        })
        
        if (response.ok) {
            Message.success('图片删除成功')
            loadImages() // 重新加载列表
            return true
        } else {
            const result = await response.json()
            throw new Error(result.message || '删除失败')
        }
    } catch (error) {
        console.error('删除图片错误:', error)
        Message.error('删除图片失败: ' + error.message)
        return false
    }
}

// 图片加载错误处理
const handleImageError = (event) => {
    // 占位图（灰色背景+问号）
    event.target.src = 'data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjAwIiBoZWlnaHQ9IjIwMCIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj48cmVjdCB3aWR0aD0iMTAwJSIgaGVpZ2h0PSIxMDAlIiBmaWxsPSIjZGRkIi8+PHRleHQgeD0iNTAlIiB5PSI1MCUiIGZvbnQtZmFtaWx5PSJBcmlhbCIgZm9udC1zaXplPSIxNCIgZmlsbD0iIzk5OSIgdGV4dC1hbmNob3I9Im1pZGRsZSIgZHk9Ii4zZW0iPuWbvueJh+WKoOi9veWksei0pTwvdGV4dD48L3N2Zz4='
}

// 工具函数（仅保留必要项）
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

// 生命周期
onMounted(() => {
    loadImages()
})

// 清理资源
onUnmounted(() => {
    // 清理可能的全局函数
    delete window.togglePreviewCopyMenu
    delete window.copyPreviewImageLink
    delete window.downloadPreviewImage
    delete window.deletePreviewImage
})
</script>