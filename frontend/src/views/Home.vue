<template>
  <!-- 主要内容区域 -->
  <div class="pt-6 md:px-4 xl:container xl:mx-auto">
    <!-- 上传区域 -->
    <section class="upload-section mb-6">
      <div class="bg-white dark:bg-dark-200 rounded-xl shadow-md dark:shadow-dark-md p-5 transition-all duration-300 hover:shadow-lg dark:hover:shadow-dark-lg">
        <h2 class="section-title text-lg font-semibold mb-4 flex justify-between items-center gap-2">
          <span>
            <i class="ri-upload-line text-primary"></i>
            图片上传
          </span>
          <!-- 存储选择 -->
          <div class="w-[40%] max-w-[210px]">
              <select 
                class="w-full px-3 py-2 border border-light-300 dark:border-dark-100 rounded-lg bg-white dark:bg-dark-200 text-sm outline-none focus:ring-1 focus:ring-primary focus:border-primary transition-all"
                v-model="selectedBucket"
                :disabled="isGuest()"
                @change="handleBucketChange"
              >
                <option 
                  v-for="bucket in presetBuckets" 
                  :key="bucket.id"
                  :value="bucket.id"
                >{{ bucket.name }}  ({{ bucket.type }})</option>
              </select>
            </div>
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
          @dragover.prevent="handleDragOver"
          @dragenter.prevent="handleDragEnter"
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

        <!-- 自定义标签 -->
        <div class="mt-5">
          <!-- 标签标题 -->
          <div class="flex items-center mb-3">
            <i class="ri-bookmark-line text-primary"></i>
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300 ml-1">标签管理</span>
          </div>
          
          <!-- 标签选择与添加 -->
          <div class="flex flex-wrap items-center gap-3 mb-4">
            <!-- 预设标签选择 -->
            <div class="w-[40%]">
              <select 
                class="w-full px-3 py-2 border border-light-300 dark:border-dark-100 rounded-lg bg-white dark:bg-dark-200 text-sm outline-none focus:ring-1 focus:ring-primary focus:border-primary transition-all"
                v-model="selectedPresetTag"
                @change="addPresetTag"
                :disabled="isUploading"
              >
                <option value="" selected>请选择...</option>
                <option 
                  v-for="presetTag in presetTags" 
                  :key="presetTag.id"
                  :value="presetTag.name"
                >{{ presetTag.name }}</option>
              </select>
            </div>
            
            <!-- 自定义标签输入 -->
            <div class="flex flex-1 relative">
              <input 
                type="text" 
                placeholder="输入自定义标签"
                class="w-[30%] flex-1 px-3 py-2 border border-light-300 dark:border-dark-100 rounded-lg bg-white dark:bg-dark-200 text-sm outline-none focus:ring-1 focus:ring-primary focus:border-primary transition-all"
                v-model="customTagInput"
                @keyup.enter="addCustomTag"
                maxlength="10"
                :disabled="isUploading"
              >
              <button 
                class="bg-primary absolute right-0 p-0 hover:bg-primary-dark text-white px-3 py-[7px] rounded-r-lg transition-colors duration-200 flex items-center justify-center"
                @click="addCustomTag"
                :disabled="isUploading || !customTagInput.trim()"
              >
                <i class="ri-add-line"></i>
              </button>
            </div>
          </div>
          
          <!-- 已选择标签展示 -->
          <div class="tag-list flex flex-wrap gap-2">
            <div 
              v-for="(tag, index) in selectedTags" 
              :key="index"
              class="flex items-center px-3 py-1.5 bg-primary/10 dark:bg-primary/20 text-primary rounded-full text-sm"
            >
              <span>{{ tag }}</span>
              <button 
                class="ml-2 text-primary/70 hover:text-primary-dark transition-colors"
                @click="removeTag(index)"
                :disabled="isUploading"
              >
                <i class="ri-close-line text-xs"></i>
              </button>
            </div>
            <div v-if="selectedTags.length === 0" class="text-sm text-secondary italic">
              暂无已选标签
            </div>
          </div>
          
          <!-- 错误提示 -->
          <div v-if="tagError" class="mt-2 text-xs text-red-500 dark:text-red-400">
            {{ tagError }}
          </div>
        </div>
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

      <div v-if="recentImages.length > 0" class="recent-images-container space-y-4">
        <div
          v-for="image in recentImages" 
          :key="image.id"
          class="flex items-center mt-2 bg-white dark:bg-dark-100 rounded-lg p-3 shadow-sm hover:shadow-md transition-all duration-300"
        >
          <!-- 图片预览区域 -->
          <div class="aspect-square overflow-hidden cursor-pointer rounded w-[160px] min-w-[160px] relative group border-2 border-[#f6f6f6]
          hover:ring-2 ring-primary ring-offset-2 ease-in-out duration-300 dark:border-dark-300">
            <!-- 加载动画 -->
            <div class="loading absolute inset-0 flex items-center justify-center z-0 text-slate-300 bg-gray-100 dark:bg-gray-800">
              <svg class="w-8 h-8 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="transform: scaleX(-1) scaleY(-1);">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </div>
            <!-- 图片 -->
            <img 
              :src="getFullUrl(image.thumbnail || image.url)"
              :alt="image.filename || '图片预览'" 
              class="recent-image w-full h-full object-cover transition-all duration-500 group-hover:scale-105 opacity-0"
              loading="lazy"
              @load="handleImageLoad"
              @error="(e) => handleImageError(e, image)"
              @click.stop="previewImage(image)"
            />
            <!-- 悬停操作栏 -->
            <div class="absolute inset-0 bg-gradient-to-t from-black/70 via-black/30 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 flex flex-col justify-end p-2 pointer-events-none">
              <div class="flex justify-between items-center pointer-events-auto">
                <p class="recent-filename text-white text-xs truncate max-w-[60%]">{{ image.filename }}</p>
                <div class="flex gap-1">
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

          <!-- 链接区域 -->
          <div class="flex flex-col justify-between gap-2 w-full text-secondary max-w-full overflow-hidden p-2">
            <!-- URL 链接 -->
            <div class="recent-filename text-sm truncate bg-white border-2 dark:bg-dark-200 dark:border-dark-300 rounded-[5px] pl-8 pr-2 py-3 relative
              hover:ring-2 ring-primary ring-offset-2 dark:ring-offset-gray-900 transition ease-in-out duration-300 cursor-pointer"
              @click.stop="copyImageLink(image, 'url')"
              title="点击复制URL"
            >
              <i class="ri-link text-xs w-4 text-center text-secondary absolute left-3 top-1/2 -translate-y-1/2"></i>
              <span class="select-none pr-2 text-[#2463eb] font-medium">URL</span>
              <span class="truncate text-overflow">{{ getFullUrl(image.url) }}</span>
            </div>

            <!-- HTML 代码 -->
            <div class="recent-filename text-sm truncate bg-white border-2 dark:bg-dark-200 dark:border-dark-300 rounded-[5px] pl-8 pr-2 py-3 relative
              hover:ring-2 ring-primary ring-offset-2 dark:ring-offset-gray-900 transition ease-in-out duration-300 cursor-pointer"
              @click.stop="copyImageLink(image, 'html')"
              title="点击复制HTML"
            >
              <i class="ri-code-fill text-xs w-4 text-center text-secondary absolute left-3 top-1/2 -translate-y-1/2"></i>
              <span class="select-none pr-2 text-[#ff8c00] font-medium">HTML</span>
              <span class="truncate text-overflow">{{ getHtmlCode(image) }}</span>
            </div>

            <!-- Markdown 代码 -->
            <div class="recent-filename text-sm truncate bg-white border-2 dark:bg-dark-200 dark:border-dark-300 rounded-[5px] pl-8 pr-2 py-3 relative
              hover:ring-2 ring-primary ring-offset-2 dark:ring-offset-gray-900 transition ease-in-out duration-300 cursor-pointer"
              @click.stop="copyImageLink(image, 'markdown')"
              title="点击复制Markdown"
            >
              <i class="ri-markdown-fill text-xs w-4 text-center text-secondary absolute left-3 top-1/2 -translate-y-1/2"></i>
              <span class="select-none pr-2 text-[#6e5494] font-medium">MD</span>
              <span class="truncate">{{ getMarkdownCode(image) }}</span>
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
import errorImg from '@/assets/images/error.webp';
import { ref, onMounted, nextTick, onBeforeUnmount } from 'vue'

// ====================== 常量定义 ======================
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';
const ALLOWED_FILE_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp', 'image/svg+xml'];

// ====================== 响应式数据 ======================
// 上传相关
const isDragOver = ref(false);
const isUploading = ref(false);
const uploadingCount = ref(0);
const uploadProgress = ref(0);
const recentImages = ref([]);
const fileInput = ref(null);

// 标签相关
const presetTags = ref([]);
const selectedPresetTag = ref('');
const customTagInput = ref('');
const selectedTags = ref([]);
const tagError = ref('');

// 存储相关
const presetBuckets = ref([
  { id: "1", name: '默认存储', type: "default" },
]);
const selectedBucket = ref("1");

// 预览相关
const activeCopyMenu = ref(null);
let previewCopyMenu = false;
let currentPreviewImage = null;
let previewModalInstance = null;
let progressInterval = null; // 上传进度定时器

// ====================== 工具函数 ======================
/**
 * 检查是否为游客
 */
function isGuest() {
  const userInfo = JSON.parse(localStorage.getItem('userInfo') || '{}');
  if(userInfo?.isTourist == true) return true;
  else return false;
}

/**
 * 获取完整的图片URL
 */
const getFullUrl = (path) => {
  if (!path) return '';
  if (typeof window === 'undefined') return path;
  
  // 处理绝对路径和相对路径
  if (path.startsWith('http')) return path;
  return `${window.location.origin}${path}`;
};

/**
 * 格式化文件大小
 */
const formatFileSize = (bytes) => {
  if (!bytes || bytes < 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

/**
 * 格式化日期
 */
const formatDate = (dateString) => {
  if (!dateString) return '';
  try {
    const date = new Date(dateString);
    return date.toLocaleString('zh-CN');
  } catch (error) {
    console.error('日期格式化失败:', error);
    return dateString;
  }
};

/**
 * 获取复制类型文本
 */
const getTypeText = (type) => {
  const typeMap = {
    url: 'URL',
    html: 'HTML',
    markdown: 'Markdown'
  };
  return typeMap[type] || '';
};

/**
 * 生成HTML代码
 */
const getHtmlCode = (image) => {
  const url = getFullUrl(image.url);
  const alt = image.filename || '图片预览';
  return `<img src="${url}" alt="${alt}"/>`;
};

/**
 * 生成Markdown代码
 */
const getMarkdownCode = (image) => {
  const url = getFullUrl(image.url);
  const filename = image.filename || '图片';
  return `![${filename}](${url})`;
};

// ====================== API 请求函数 ======================
/**
 * 获取上传配置
 */
const getUploadConfig = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/uploadConfig`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    
    const result = await response.json();
    if (response.ok && result.code === 200) {
      presetTags.value = result.data?.tags || [];
      presetBuckets.value = result.data?.buckets || [];
      const bucketId = localStorage.getItem('currentBucket');
      if (bucketId != null){
        const num = parseInt(bucketId);
        selectedBucket.value = Number.isNaN(num) ? '1' : bucketId;
      } else {
        selectedBucket.value = result.data?.default_bucket || '1';
      }
    } else {
      throw new Error(result.message || '获取上传配置失败');
    }
  } catch (error) {
    console.error('获取上传配置失败:', error);
    Message.error(error.message || '获取上传配置失败');
  }
};

/**
 * 加载最近上传的图片
 */
const loadRecentImages = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/images?limit=12`, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    
    if (response.ok) {
      const result = await response.json();
      recentImages.value = Array.isArray(result.data?.images) ? result.data.images : [];
    }
  } catch (error) {
    console.error('加载图片失败:', error);
    recentImages.value = [];
    Message.error(`加载图片失败: ${error.message}`, {
      duration: 3000,
      position: 'top-right',
      showClose: true
    });
  }
};

/**
 * 删除单张图片
 */
const deleteAsync = async (imageId) => { 
  let loadingInstance;
  try {
    loadingInstance = Loading.show({
      text: '删除中...',
      color: '#ff4d4f',
      mask: true
    });
    
    const response = await fetch(`${API_BASE_URL}/api/images/${imageId}`, {
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`,
        'Content-Type': 'application/json'
      }
    });
    
    if (response.ok) {
      Message.success('图片删除成功', {
        duration: 1500,
        position: 'top-right'
      });
      
      // 如果删除的是当前预览的图片，关闭预览弹窗
      if (currentPreviewImage?.id === imageId && previewModalInstance) {
        previewModalInstance.close();
        currentPreviewImage = null;
        previewModalInstance = null;
      }
      
      previewCopyMenu = false;
      activeCopyMenu.value = null;
      await loadRecentImages();
    } else {
      const result = await response.json();
      throw new Error(result.message || '删除失败');
    }
  } catch (error) {
    console.error('删除图片错误:', error);
    Message.error(`删除失败: ${error.message}`, {
      duration: 3000,
      position: 'top-right',
      showClose: true
    });
  } finally {
    if (loadingInstance) await loadingInstance.hide();
  }
};

/**
 * 添加标签到服务器
 */
const addTagToServer = async (tag) => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/tags`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: JSON.stringify({ name: tag })
    });
    
    const result = await response.json();
    if (response.ok && result.code === 200) {
      return result.data;
    } else {
      throw new Error(result.message || '添加标签失败');
    }
  } catch (error) {
    console.error('添加标签失败:', error);
    throw error;
  }
};

// ====================== 事件处理函数 ======================
/**
 * 拖拽相关处理
 */
const handleDragOver = () => {
  isDragOver.value = true;
};

const handleDragEnter = () => {
  isDragOver.value = true;
};

const handleDragLeave = (e) => {
  if (!e.currentTarget.contains(e.relatedTarget)) {
    isDragOver.value = false;
  }
};

const handleDrop = (e) => {
  e.preventDefault();
  isDragOver.value = false;
  
  const files = Array.from(e.dataTransfer.files);
  const validFiles = validateFiles(files);
  
  if (validFiles.length > 0) {
    uploadFiles(validFiles);
  } else {
    Message.error('请拖拽有效的图片文件（仅支持JPG、PNG、GIF、WebP、SVG）', {
      duration: 3000,
      position: 'top-right'
    });
  }
};

/**
 * 文件选择处理
 */
const triggerFileInput = () => {
  if (!isUploading.value && fileInput.value) {
    fileInput.value.click();
  }
};

const handleFileSelect = (e) => {
  const files = Array.from(e.target.files);
  if (files.length > 0) {
    const validFiles = validateFiles(files);
    if (validFiles.length > 0) {
      uploadFiles(validFiles);
    }
  }
  e.target.value = ''; // 清空文件选择
};

/**
 * 剪贴板粘贴处理
 */
const handlePaste = async (e) => {
  const items = e.clipboardData?.items;
  if (!items) return;
  
  const imageFiles = [];
  
  for (let item of items) {
    if (item.type.startsWith('image/')) {
      const file = item.getAsFile();
      if (file) {
        const timestamp = new Date().getTime();
        const extension = item.type.split('/')[1] || 'png';
        const newFile = new File([file], `paste-${timestamp}.${extension}`, {
          type: item.type
        });
        imageFiles.push(newFile);
      }
    }
  }
  
  if (imageFiles.length > 0) {
    e.preventDefault();
    uploadFiles(imageFiles);
    Message.success(`从剪贴板粘贴了 ${imageFiles.length} 个图片`, {
      duration: 2000,
      position: 'top-right'
    });
  }
};

/**
 * 验证文件有效性
 */
const validateFiles = (files) => {
  const validFiles = [];
  
  files.forEach(file => {
    // 验证文件类型
    if (!file.type.startsWith('image/') || !ALLOWED_FILE_TYPES.includes(file.type)) {
      Message.warning(`文件 ${file.name} 不是支持的图片格式`, {
        duration: 2000,
        position: 'top-right'
      });
      return;
    }
    
    validFiles.push(file);
  });
  
  return validFiles;
};

/**
 * 文件上传
 */
const uploadFiles = async (files) => {
  if (isUploading.value) return;
  
  isUploading.value = true;
  uploadingCount.value = files.length;
  uploadProgress.value = 0;
  
  // 重置进度定时器
  if (progressInterval) clearInterval(progressInterval);
  progressInterval = setInterval(() => {
    if (uploadProgress.value < 95) {
      uploadProgress.value += Math.random() * 5;
    }
  }, 150);
  
  try {
    const formData = new FormData();
    files.forEach(file => {
      formData.append('images[]', file);
    });
    
    // 携带标签数据
    if (selectedTags.value.length > 0) {
      formData.append('tags', JSON.stringify(selectedTags.value));
    }
    // 携带存储桶信息
    formData.append('bucket_id', selectedBucket.value || '1')
    
    const response = await fetch(`${API_BASE_URL}/api/upload/images`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: formData
    });
    
    clearInterval(progressInterval);
    uploadProgress.value = 100;
    
    const result = await response.json();
    
    if (response.ok && result.code === 200) {
      await loadRecentImages();
      Message.success(`上传成功`, {
        duration: 2000,
        position: 'top-right'
      });
    } else {
      throw new Error(result.message || '上传失败');
    }
  } catch (error) {
    console.error('上传错误:', error);
    Message.error(`上传失败: ${error.message}`, {
      duration: 3000,
      position: 'top-right',
      showClose: true
    });
  } finally {
    isUploading.value = false;
    uploadingCount.value = 0;
    uploadProgress.value = 0;
    if (progressInterval) clearInterval(progressInterval);
  }
};

/**
 * 标签相关处理
 */
const addPresetTag = () => {
  const tag = selectedPresetTag.value;
  if (!tag) return;
  
  tagError.value = '';
  
  if (selectedTags.value.includes(tag)) {
    tagError.value = '该标签已添加';
    selectedPresetTag.value = '';
    return;
  }
  
  selectedTags.value.push(tag);
  selectedPresetTag.value = ''; // 清空选择
};

const addCustomTag = async () => {
  const tag = customTagInput.value.trim();
  if (!tag) {
    tagError.value = '标签不能为空';
    return;
  }
  
  // 校验标签长度
  if (tag.length > 10) {
    tagError.value = '标签长度不能超过10个字符';
    return;
  }
  
  // 校验标签不重复
  if (selectedTags.value.includes(tag)) {
    tagError.value = '该标签已添加';

    return;
  }
  
  try {
    // 添加到服务器
    const newTag = await addTagToServer(tag);
    
    // 更新本地列表
    selectedTags.value.push(tag);
    presetTags.value.push(newTag);
    customTagInput.value = ''; // 清空输入框
    tagError.value = '';
    
    Message.success('标签添加成功');
  } catch (error) {
    tagError.value = error.message || '添加标签失败';
    Message.error(error.message || '添加标签失败');
  }
};

const removeTag = (index) => {
  selectedTags.value.splice(index, 1);
  tagError.value = '';
};

/**
 * 图片相关操作
 */
const handleImageLoad = (e) => {
  e.target.classList.remove('opacity-0');
  const loadingEl = e.target.parentElement.querySelector('.loading');
  if (loadingEl) loadingEl.classList.add('hidden');
};

const handleImageError = (e) => {
  e.target.src = errorImg;
  const loadingEl = e.target.parentElement.querySelector('.loading');
  if (loadingEl) loadingEl.classList.add('hidden');
};

const copyImageLink = async (image, type) => {
  if (!image) return;
  
  const fullUrl = getFullUrl(image.url);
  let copyText = '';
  
  switch (type) {
    case 'url':
      copyText = fullUrl;
      break;
    case 'html':
      copyText = `<img src="${fullUrl}" alt="${image.filename}" width="${image.width || ''}" height="${image.height || ''}">`;
      break;
    case 'markdown':
      copyText = `![${image.filename}](${fullUrl})`;
      break;
    default:
      copyText = fullUrl;
  }
  
  try {
    await navigator.clipboard.writeText(copyText);
    Message.success(`已复制${getTypeText(type)}格式`, {
      duration: 1500,
      position: 'top-right'
    });
  } catch (error) {
    // 降级处理
    const textArea = document.createElement('textarea');
    textArea.value = copyText;
    document.body.appendChild(textArea);
    textArea.select();
    document.execCommand('copy');
    document.body.removeChild(textArea);
    Message.success(`已复制${getTypeText(type)}格式`, {
      duration: 1500,
      position: 'top-right'
    });
  } finally {
    // 关闭所有下拉菜单
    nextTick(() => {
      previewCopyMenu = false;
      activeCopyMenu.value = null;
      
      // 关闭预览弹窗内的复制下拉框
      const dropdown = document.getElementById('previewCopyDropdown');
      const icon = document.getElementById('copyMenuIcon');
      if (dropdown && icon) {
        dropdown.classList.add('hidden', 'opacity-0', 'translate-y-[-5px]');
        dropdown.classList.remove('block', 'opacity-100', 'translate-y-0');
        icon.classList.remove('rotate-180');
      }
    });
  }
};

/**
 * 存储选择处理事件，设置后优先使用选择的存储
 */
const handleBucketChange = () => {
  const bucketId = selectedBucket.value;
  if (!bucketId) return;
  localStorage.setItem('currentBucket', bucketId);
};

const deleteImage = (imageId) => {
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
          modal.close();
          await deleteAsync(imageId);
        }
      }
    ],
    maskClose: true
  });
  modal.open();
};

const downloadImage = (image) => {
  if (!image || !image.url) {
    Message.error('图片信息不完整，无法下载', {
      duration: 2000,
      position: 'top-right'
    });
    return;
  }
  
  const fullUrl = getFullUrl(image.url);
  const link = document.createElement('a');
  link.href = fullUrl;
  link.download = image.filename || `image-${Date.now()}.png`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  
  Message.info('开始下载图片', {
    duration: 1500,
    position: 'top-right'
  });
  
  previewCopyMenu = false;
  activeCopyMenu.value = null;
};

/**
 * 图片预览功能
 */
const previewImage = (image) => {
  if (!image || !image.url) {
    Message.error('图片信息不完整，无法预览', {
      duration: 2000,
      position: 'top-right'
    });
    return;
  }
  
  currentPreviewImage = image;

  const tagsHtml = image.tags?.map(tag => `
    <div class="px-2 py-0.5 rounded bg-primary/10 dark:bg-primary/20 text-primary text-xs">
      <span>${tag.name}</span>
    </div>
  `).join('') || '';
  
  // 构建预览弹窗内容
  const previewContent = `
    <div class="image-preview-popup w-full max-w-5xl max-h-[85vh] flex flex-col overflow-hidden bg-white dark:bg-dark-200">
      <!-- 顶部操作栏 -->
      <div class="preview-header bg-light-50 pb-2 flex justify-between items-center">
          <h3 class="text-xs font-medium truncate max-w-[50%]">${image.filename}</h3>
          <div class="flex gap-1">
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
          <a 
              class="spotlight min-w-full max-w-full min-h-[260px] block" 
              href="${getFullUrl(image.url)}" 
              data-description="尺寸: ${image.width || '未知'}×${image.height || '未知'} | 大小: ${formatFileSize(image.file_size || 0)} | 上传日期：${formatDate(image.created_at)} | 角色：${image.user_id == '1' ? '管理员' : '游客'}"
          >
              <div class="relative max-w-full w-fill max-h-[360px] min-h-[260px] rounded-lg overflow-hidden animate-pulse flex items-center justify-center">
                  <div class="absolute inset-0 flex items-center justify-center">
                      <svg class="w-10 h-10 text-slate-300 animate-spin loading-svg" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="transform: scaleX(-1) scaleY(-1);">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                      </svg>
                  </div>
                  <img 
                      src="${getFullUrl(image.url)}"
                      alt="${image.filename}" 
                      class="max-w-full w-fill max-h-[360px] min-h-[260px] object-contain rounded-lg relative z-10 opacity-0 transition-opacity duration-300"
                      onload="this.classList.remove('opacity-0'); this.parentElement.classList.remove('animate-pulse'); this.parentElement.querySelector('.loading-svg').classList.add('hidden');"
                      onerror="this.parentElement.classList.remove('animate-pulse'); this.classList.remove('opacity-0'); this.src='${errorImg}';"
                  />
              </div>
          </a>
      </div>

      <!-- 图片复制区域 -->
      <div class="flex gap-1 flex-wrap items-center w-full mt-3 mb-3">
          <p class="mr-1 text-xs text-secondary font-semibold">复制：</p>
          <button 
              onclick="window.copyPreviewImageLink('url')"
              class="px-2 py-1 text-xs bg-primary shadow-md text-white dark:bg-dark-300 hover:bg-blue-800 rounded transition-colors duration-200">
              <i class="ri-link text-xs w-4 text-center text-white"></i> URL
          </button>

          <button 
              onclick="window.copyPreviewImageLink('html')"
              class="px-2 py-1 text-xs bg-primary shadow-md text-white dark:bg-dark-300 hover:bg-blue-800 rounded transition-colors duration-200">
              <i class="ri-code-fill text-xs w-4 text-center text-white"></i> HTML
          </button>

          <button 
              onclick="window.copyPreviewImageLink('markdown')"
              class="px-2 py-1 text-xs bg-primary shadow-md text-white dark:bg-dark-300 hover:bg-blue-800 rounded transition-colors duration-200">
              <i class="ri-markdown-fill text-xs w-4 text-center text-white"></i> Markdown
          </button>
      </div>

      <!-- Tags -->
      <div class="pt-2 flex flex-wrap gap-2 items-center">
        <p class="mr-1 text-xs text-secondary font-semibold">Tags：</p>
        ${tagsHtml}
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
  `;

  // 注册预览相关全局函数
  window.copyPreviewImageLink = (type) => copyImageLink(currentPreviewImage, type);
  window.downloadPreviewImage = () => downloadImage(currentPreviewImage);
  window.deletePreviewImage = () => {
    deleteImage(currentPreviewImage.id);
    closePreviewModal();
  };
  window.closePreviewModal = () => {
    if (previewModalInstance) {
      previewModalInstance.close();
      cleanupPreview();
    }
  };

  // 创建预览弹窗
  previewModalInstance = new PopupModal({
    title: '图片预览',
    content: previewContent,
    type: 'default',
    buttons: [{
      text: '确定',
      type: 'default',
      callback: (modal) => modal.close()
    }],
    maskClose: true,
    zIndex: 10000,
    maxHeight: '90vh',
    onClose: cleanupPreview
  });

  previewModalInstance.open();

  // 阻止弹窗内容冒泡
  nextTick(() => {
    const previewContent = document.querySelector('.image-preview-popup');
    if (previewContent) {
      previewContent.addEventListener('click', (e) => e.stopPropagation());
    }
  });
};

/**
 * 清理预览相关资源
 */
const cleanupPreview = () => {
  // 清理全局函数
  window.copyPreviewImageLink = null;
  window.downloadPreviewImage = null;
  window.deletePreviewImage = null;
  window.closePreviewModal = null;
  
  // 重置状态
  currentPreviewImage = null;
  previewModalInstance = null;
  previewCopyMenu = false;
};

/**
 * 全局点击处理（关闭下拉菜单）
 */
const handleGlobalClick = (e) => {
  if (activeCopyMenu.value !== null) {
    const cardCopyMenus = document.querySelectorAll('.recent-item .relative.z-50');
    let isClickInside = false;
    
    cardCopyMenus.forEach(menu => {
      if (menu.contains(e.target)) {
        isClickInside = true;
      }
    });
    
    if (!isClickInside) {
      activeCopyMenu.value = null;
    }
  }
};

// ====================== 生命周期 ======================
onMounted(() => {
  // 初始化数据
  getUploadConfig();
  setTimeout(loadRecentImages, 100);
  
  // 注册全局事件
  document.addEventListener('paste', handlePaste);
  document.addEventListener('click', handleGlobalClick);
});

onBeforeUnmount(() => {
  // 清理定时器
  if (progressInterval) clearInterval(progressInterval);
  
  // 移除事件监听
  document.removeEventListener('paste', handlePaste);
  document.removeEventListener('click', handleGlobalClick);
  
  // 清理预览资源
  cleanupPreview();
  
  // 关闭所有消息提示
  if (window.Message) {
    Message.closeAll();
  }
});
</script>