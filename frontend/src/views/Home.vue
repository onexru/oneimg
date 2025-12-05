<template>
  <!-- 主要内容区域 -->
  <div class="pt-16 px-4 md:px-6 lg:px-8 xl:container xl:mx-auto">
    <!-- 上传区域 -->
    <section class="upload-section mb-6">
      <div class="bg-white dark:bg-dark-200 rounded-xl shadow-md dark:shadow-dark-md p-5 transition-all duration-300 hover:shadow-lg dark:hover:shadow-dark-lg">
        <h2 class="section-title text-lg font-semibold mb-4 flex items-center gap-2">
          <i class="ri-upload-line text-primary"></i>
          图片上传
        </h2>

        <!-- 拖拽上传区域 -->
        <div 
          class="upload-area relative rounded-xl border-2 border-dashed transition-all duration-300 cursor-pointer overflow-hidden"
          :class="{ 
            'border-primary/30 bg-primary/5 dark:bg-primary/5': isDragOver,
            'border-light-300 dark:border-dark-100 bg-light-50 dark:bg-dark-200/50': !isDragOver && !isUploading,
            'border-primary/50 bg-primary/10 dark:bg-primary/10': isUploading
          }"
          @drop="handleDrop"
          @dragover="handleDragOver"
          @dragenter="handleDragEnter"
          @dragleave="handleDragLeave"
          @click="triggerFileInput"
        >
          <!-- 未上传状态 -->
          <div v-if="!isUploading" class="upload-content py-16 px-4 text-center">
            <div class="upload-icon text-5xl text-primary mb-3">
              <i class="ri-upload-cloud-line"></i>
            </div>
            <h3 class="text-base font-medium mb-2">选择或拖拽图片到此处上传</h3>
            <p class="text-secondary text-sm mb-4">支持 JPG、PNG、GIF、WebP、SVG 格式，单张不超过 10MB</p>
            <button class="bg-primary hover:bg-primary-dark text-white px-5 py-2 rounded-lg transition-colors duration-200 flex items-center justify-center gap-2 mx-auto">
              <i class="ri-file-image-line"></i>
              选择图片
            </button>
            <p class="paste-tip text-sm text-secondary flex items-center justify-center gap-2 mt-3">
              支持 Ctrl+V 粘贴剪贴板图片，或直接拖入图片
            </p>
          </div>

          <!-- 上传进度状态 -->
          <div v-else class="upload-progress py-16 px-4 text-center">
            <div class="spinner w-10 h-10 border-4 border-primary/30 border-t-primary rounded-full animate-spin mx-auto mb-3"></div>
            <p class="text-secondary text-sm mb-3">正在上传 {{ uploadingCount }} 个文件（{{ Math.round(uploadProgress) }}%）</p>
            <div class="progress-bar w-full max-w-md mx-auto h-2 bg-light-200 dark:bg-dark-100 rounded-full overflow-hidden">
              <div 
                class="progress-fill h-full bg-primary transition-all duration-300 ease-out"
                :style="{ width: uploadProgress + '%' }"
              ></div>
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
          class="hidden"
        />
      </div>
    </section>

    <!-- 最近上传的图片 -->
    <section class="recent-section">
      <div class="flex justify-between items-center mb-3">
        <h2 class="section-title text-lg font-semibold flex items-center gap-2">
          <i class="ri-history-line text-primary"></i>
          最近上传
        </h2>
        <span class="text-sm text-secondary">{{ recentImages.length }} 张图片</span>
      </div>

      <!-- 图片网格 -->
      <div v-if="recentImages.length > 0" class="recent-grid grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
        <div 
          v-for="image in recentImages" 
          :key="image.id"
          class="recent-item rounded-lg bg-light-100 dark:bg-dark-100 transition-all duration-300 hover:shadow-md dark:hover:shadow-dark-md group relative"
        >
          <!-- 图片区域 -->
          <div class="aspect-square overflow-hidden cursor-pointer rounded" @click.stop="previewImage(image)">
            <img 
              :src="getFullUrl(image.thumbnail || image.url)"
              :alt="image.filename" 
              class="recent-image w-full h-full object-cover transition-transform duration-500 group-hover:scale-110"
              loading="lazy"
            />
          </div>
          <!-- 悬停操作栏 -->
          <div class="absolute inset-0 bg-gradient-to-t from-black/70 via-black/30 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 flex flex-col justify-end p-2 pointer-events-none">
            <div class="flex justify-between items-center pointer-events-auto">
              <p class="recent-filename text-white text-xs truncate max-w-[60%]">{{ image.filename }}</p>
              <div class="flex gap-1">
                <!-- 复制菜单：简化逻辑，确保显示 -->
                <div class="relative z-50">
                  <button 
                    class="w-6 h-6 rounded-full bg-white/20 hover:bg-white/40 text-white flex items-center justify-center transition-colors duration-200"
                    title="复制链接"
                    @click.stop="toggleCardCopyMenu(image.id)"
                  >
                    <i class="ri-file-copy-line text-xs"></i>
                  </button>
                  <!-- 复制下拉框：强制显示层级 -->
                  <div 
                    class="absolute right-0 mt-1 w-40 bg-white rounded-md shadow-lg z-60 transition-all duration-200"
                    :class="{
                      'dark:bg-dark-200 dark:shadow-dark-xl': true,
                      'block opacity-100': activeCopyMenu === image.id,
                      'hidden opacity-0': activeCopyMenu !== image.id
                    }"
                  >
                    <div class="p-1">
                      <button 
                        @click.stop="copyImageLink(image, 'url')"
                        class="w-full text-left px-2 py-1.5 text-xs text-gray-800 dark:text-light-100 hover:bg-light-100 dark:hover:bg-dark-300 rounded transition-colors duration-200 flex items-center gap-2"
                      >
                        <i class="ri-link text-xs w-4 text-center"></i>
                        URL
                      </button>
                      <button 
                        @click.stop="copyImageLink(image, 'html')"
                        class="w-full text-left px-2 py-1.5 text-xs text-gray-800 dark:text-light-100 hover:bg-light-100 dark:hover:bg-dark-300 rounded transition-colors duration-200 flex items-center gap-2"
                      >
                        <i class="ri-code-fill text-xs w-4 text-center"></i>
                        HTML
                      </button>
                      <button 
                        @click.stop="copyImageLink(image, 'markdown')"
                        class="w-full text-left px-2 py-1.5 text-xs text-gray-800 dark:text-light-100 hover:bg-light-100 dark:hover:bg-dark-300 rounded transition-colors duration-200 flex items-center gap-2"
                      >
                        <i class="ri-markdown-fill text-xs w-4 text-center"></i>
                        MD
                      </button>
                    </div>
                  </div>
                </div>
                <!-- 下载按钮 -->
                <button 
                  @click.stop="downloadImage(image)"
                  class="w-6 h-6 rounded-full bg-white/20 hover:bg-white/40 text-white flex items-center justify-center transition-colors duration-200"
                  title="下载图片"
                >
                  <i class="ri-download-fill text-xs"></i>
                </button>
                <!-- 删除按钮 -->
                <button 
                  @click.stop="deleteImage(image.id)"
                  class="w-6 h-6 rounded-full bg-danger/30 hover:bg-danger/50 text-white flex items-center justify-center transition-colors duration-200"
                  title="删除图片"
                >
                  <i class="ri-delete-bin-fill text-xs"></i>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 无图片状态 -->
      <div v-else class="no-images bg-white dark:bg-dark-200 rounded-xl shadow-md dark:shadow-dark-md p-8 text-center">
        <div class="text-5xl text-light-300 dark:text-dark-100 mb-3">
          <i class="ri-image-line"></i>
        </div>
        <p class="text-secondary text-base mb-4">暂无上传的图片</p>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, nextTick } from 'vue'

// 核心：获取完整URL的函数
const getFullUrl = (path) => {
  if (!path) return ''
  if (typeof window !== 'undefined') {
    return window.location.origin + path
  }
  return path
}

// 响应式数据
const isDragOver = ref(false)
const isUploading = ref(false)
const uploadingCount = ref(0)
const uploadProgress = ref(0)
const recentImages = ref([])
const fileInput = ref(null)

// 下拉框控制变量
const activeCopyMenu = ref(null) // 卡片复制菜单
let previewCopyMenu = false // 预览复制菜单（非响应式）
let currentPreviewImage = null // 当前预览的图片
let previewModalInstance = null // 预览弹窗实例（用于关闭控制）

// 卡片复制菜单切换（简化逻辑）
const toggleCardCopyMenu = (imageId) => {
  if (activeCopyMenu.value === imageId) {
    activeCopyMenu.value = null
  } else {
    activeCopyMenu.value = imageId
  }
}

// 全局点击关闭下拉框（增强版）
const handleGlobalClick = (e) => {
  // 关闭卡片下拉框
  if (activeCopyMenu.value !== null) {
    const cardCopyMenus = document.querySelectorAll('.recent-item .relative.z-50')
    let isClickInside = false
    cardCopyMenus.forEach(menu => {
      if (menu.contains(e.target)) {
        isClickInside = true
      }
    })
    if (!isClickInside) {
      activeCopyMenu.value = null
    }
  }
}

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
    // 替换为 Message 错误提示
    Message.error('请拖拽图片文件', {
      duration: 3000,
      position: 'top-right'
    })
  }
}

// 文件选择处理
const triggerFileInput = () => {
  if (!isUploading.value && fileInput.value) {
    fileInput.value.click()
  }
}

const handleFileSelect = (e) => {
  const files = Array.from(e.target.files)
  if (files.length > 0) {
    uploadFiles(files)
  }
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
    // 替换为 Message 成功提示
    Message.success(`从剪贴板粘贴了 ${imageFiles.length} 个图片`, {
      duration: 2000,
      position: 'top-right'
    })
  }
}

// 文件上传
const uploadFiles = async (files) => {
  if (isUploading.value) return
  
  isUploading.value = true
  uploadingCount.value = files.length
  uploadProgress.value = 0
  
  const formData = new FormData()
  files.forEach(file => {
    formData.append('images[]', file)
  })
  
  try {
    const progressInterval = setInterval(() => {
      if (uploadProgress.value < 95) {
        uploadProgress.value += Math.random() * 5
      }
    }, 150)
    
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
    
    if (response.ok && result.code === 200) {
      await loadRecentImages()
      Message.success(`上传成功`, {
        duration: 2000,
        position: 'top-right'
      })
    } else {
      throw new Error(result.message || '上传失败')
    }
  } catch (error) {
    console.error('上传错误:', error)
    // 替换为 Message 错误提示
    Message.error(`上传失败: ${error.message}`, {
      duration: 3000,
      position: 'top-right',
      showClose: true
    })
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
      recentImages.value = Array.isArray(result.data?.images) ? result.data.images : []
    }
  } catch (error) {
    console.error('加载图片失败:', error)
    recentImages.value = []
    // 替换为 Message 错误提示
    Message.error(`加载图片失败: ${error.message}`, {
      duration: 3000,
      position: 'top-right',
      showClose: true
    })
  }
}

// 多格式复制功能
const copyImageLink = async (image, type) => {
  if (!image) return
  const fullUrl = getFullUrl(image.url)
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
    // 替换为 Message 成功提示
    Message.success(`已复制${getTypeText(type)}格式`, {
      duration: 1500,
      position: 'top-right'
    })
  } catch (error) {
    const textArea = document.createElement('textarea')
    textArea.value = copyText
    document.body.appendChild(textArea)
    textArea.select()
    document.execCommand('copy')
    document.body.removeChild(textArea)
    // 替换为 Message 成功提示
    Message.success(`已复制${getTypeText(type)}格式`, {
      duration: 1500,
      position: 'top-right'
    })
  } finally {
    // 复制后强制关闭所有下拉框
    nextTick(() => {
      previewCopyMenu = false
      activeCopyMenu.value = null
      // 关闭预览弹窗内的复制下拉框
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

// 辅助函数：获取复制类型文本
const getTypeText = (type) => {
  switch (type) {
    case 'url': return 'URL'
    case 'html': return 'HTML'
    case 'markdown': return 'Markdown'
    default: return ''
  }
}

const formatDate = (dateString) => {
    if (!dateString) return ''
    const date = new Date(dateString)
    return date.toLocaleString('zh-CN')
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

const deleteAsync = async (imageId) => { 
  try {
    const response = await fetch(`/api/images/${imageId}`, {
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`,
        'Content-Type': 'application/json'
      }
    })
    
    if (response.ok) {
      // 替换为 Message 成功提示
      Message.success('图片删除成功', {
        duration: 1500,
        position: 'top-right'
      })
      // 如果删除的是当前预览的图片，关闭预览弹窗
      if (currentPreviewImage?.id === imageId && previewModalInstance) {
        previewModalInstance.close()
        currentPreviewImage = null
        previewModalInstance = null
      }
      previewCopyMenu = false
      activeCopyMenu.value = null
      await loadRecentImages()
    } else {
      const result = await response.json()
      throw new Error(result.message || '删除失败')
    }
  } catch (error) {
    console.error('删除图片错误:', error)
    // 替换为 Message 错误提示
    Message.error(`删除失败: ${error.message}`, {
      duration: 3000,
      position: 'top-right',
      showClose: true
    })
  }
}

// 工具函数
const formatFileSize = (bytes) => {
  if (!bytes || bytes < 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// 核心：图片预览（保留预览弹窗）
const previewImage = (image) => {
  if (!image || !image.url) {
    // 替换为 Message 错误提示
    Message.error('图片信息不完整，无法预览', {
      duration: 2000,
      position: 'top-right'
    })
    return
  }
  
  currentPreviewImage = image
  previewCopyMenu = false // 重置复制菜单状态
  
  // 构建预览弹窗内容
  const previewContent = `
    <div class="image-preview-popup w-full max-w-5xl max-h-[85vh] flex flex-col overflow-hidden bg-white dark:bg-dark-200">
      <!-- 顶部操作栏 -->
      <div class="preview-header bg-light-50 pb-2 flex justify-between items-center">
        <h3 class="text-xs font-medium truncate max-w-[50%]">${image.filename}</h3>
        <div class="flex gap-1">
          <!-- 多格式复制菜单 -->
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
            onclick="event.stopPropagation(); window.deletePreviewImage()"
          >
            <i class="ri-delete-bin-fill text-xs"></i>
            删除
          </button>
        </div>
      </div>
      
      <!-- 预览图片区域 -->
      <div class="max-h-[360px] flex-1 overflow-auto flex items-center justify-center">
        <a class="spotlight" href="${getFullUrl(image.url)}" data-description="尺寸: ${image.width || '未知'}×${image.height || '未知'} | 大小: ${formatFileSize(image.file_size || 0)} | 上传日期：${formatDate(image.created_at)}">
            <img 
                src="${getFullUrl(image.url)}"
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
  `

  // 全局注册预览相关函数（供弹窗内 DOM 调用）
  window.togglePreviewCopyMenu = () => {
    previewCopyMenu = !previewCopyMenu
    const dropdown = document.getElementById('previewCopyDropdown')
    const icon = document.getElementById('copyMenuIcon')
    if (dropdown && icon) {
      if (previewCopyMenu) {
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

  window.copyPreviewImageLink = (type) => {
    copyImageLink(currentPreviewImage, type)
  }

  window.downloadPreviewImage = () => {
    downloadImage(currentPreviewImage)
  }

  window.deletePreviewImage = () => {
    deleteImage(currentPreviewImage.id)
    closePreviewModal();
  }

  window.closePreviewModal = () => {
    if (previewModalInstance) {
      previewModalInstance.close()
      currentPreviewImage = null
      previewModalInstance = null
      previewCopyMenu = false
    }
  }

  // 创建预览弹窗实例
  previewModalInstance = new PopupModal({
    title: '图片预览',
    content: previewContent,
    type: 'default',
    buttons: [
        {
            text: '确定',
            type: 'default',
            callback: (modal) => modal.close()
        }
        ],
    maskClose: true,
    zIndex: 10000,
    maxHeight: '90vh',
    // 生命周期回调：清理资源
    onClose: () => {
      window.togglePreviewCopyMenu = null
      window.copyPreviewImageLink = null
      window.downloadPreviewImage = null
      window.deletePreviewImage = null
      window.closePreviewModal = null
      currentPreviewImage = null
      previewModalInstance = null
      previewCopyMenu = false
    }
  })

  // 打开弹窗
  previewModalInstance.open()

  // 处理弹窗内点击事件（阻止冒泡到遮罩）
  nextTick(() => {
    const previewContent = document.querySelector('.image-preview-popup')
    if (previewContent) {
      previewContent.addEventListener('click', (e) => {
        e.stopPropagation()
      })
    }
  })
}

const downloadImage = (image) => {
  if (!image || !image.url) {
    // 替换为 Message 错误提示
    Message.error('图片信息不完整，无法下载', {
      duration: 2000,
      position: 'top-right'
    })
    return
  }
  const fullUrl = getFullUrl(image.url)
  const link = document.createElement('a')
  link.href = fullUrl
  link.download = image.filename || `image-${Date.now()}.png`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  // 替换为 Message 信息提示
  Message.info('开始下载图片', {
    duration: 1500,
    position: 'top-right'
  })
  previewCopyMenu = false
  activeCopyMenu.value = null
}

// 生命周期
onMounted(() => {
  document.addEventListener('paste', handlePaste)
  document.addEventListener('click', handleGlobalClick)
  setTimeout(() => {
    loadRecentImages()
  }, 100)
})

onUnmounted(() => {
  document.removeEventListener('paste', handlePaste)
  document.removeEventListener('click', handleGlobalClick)
  // 清理全局预览相关函数
  window.togglePreviewCopyMenu = null
  window.copyPreviewImageLink = null
  window.downloadPreviewImage = null
  window.deletePreviewImage = null
  window.closePreviewModal = null
  // 关闭预览弹窗
  if (previewModalInstance) {
    previewModalInstance.close()
  }
  // 关闭所有通知
  if (window.Message) {
    Message.closeAll()
  }
})
</script>
