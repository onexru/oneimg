<template>
  <div class="page-shell">
    <div class="space-y-3 lg:space-y-3.5">
      <section class="space-y-3">
        <div class="content-panel home-panel-compact">
          <div class="mb-3 flex flex-col gap-2 border-b border-slate-200/70 pb-3 dark:border-white/10 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p class="panel-label">主上传区</p>
              <h2 class="section-title mt-1 flex items-center gap-2 text-base font-semibold sm:text-lg">
                <i class="ri-upload-cloud-2-line text-primary"></i>
                图片上传
              </h2>
            </div>
            <div class="flex flex-wrap items-center gap-2 text-xs text-slate-500 dark:text-slate-400">
              <span class="rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 dark:border-white/10 dark:bg-slate-950">{{ presetBuckets.find(bucket => bucket.id == selectedBucket)?.name || '未选择存储' }}</span>
              <span class="rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 dark:border-white/10 dark:bg-slate-950">{{ selectedTags.length }} 个标签</span>
            </div>
          </div>

          <div 
          class="imageflow-dropzone upload-area relative cursor-pointer overflow-hidden transition-all duration-300"
          :class="{ 
            'border-primary/30 bg-primary/5 dark:bg-primary/5': isDragOver,
            'border-slate-200 bg-slate-50 dark:border-white/10 dark:bg-slate-900/40': !isDragOver && !isUploading,
            'border-primary/50 bg-primary/10 dark:bg-primary/10': isUploading
          }"
          @drop="handleDrop"
          @dragover.prevent="handleDragOver"
          @dragenter.prevent="handleDragEnter"
          @dragleave="handleDragLeave"
          @click="triggerFileInput"
        >
          <div v-if="!isUploading" class="upload-content py-6 text-center sm:py-7">
            <div class="upload-icon mb-2.5 text-4xl text-slate-900 dark:text-white sm:text-[42px]">
              <i class="ri-upload-cloud-line"></i>
            </div>
            <h3 class="mb-1.5 text-base font-semibold text-slate-900 dark:text-white">拖拽图片到此处，或点击立即上传</h3>
            <p class="mx-auto mb-3 max-w-md text-sm leading-5 text-slate-500 dark:text-slate-400">支持常见图片格式、剪贴板和 URL 上传。</p>
            <div class="flex flex-col items-stretch justify-center gap-2 sm:flex-row sm:flex-wrap sm:items-center">
            <button class="primary-button w-full px-4 py-2 sm:w-auto">
              <i class="ri-file-image-line"></i>
              选择图片
            </button>
            <button 
            @click.stop="uploadbyurlmodal"
            class="soft-button w-full border-slate-200 px-4 py-2 sm:w-auto">
              <i class="ri-links-line"></i>
              从URL上传
            </button>
            </div>
            <p class="paste-tip mt-2.5 text-center text-xs text-slate-500 dark:text-slate-400">
              支持 Ctrl+V 粘贴和直接拖入
            </p>
          </div>

          <!-- 上传进度状态 -->
          <div v-else class="upload-progress px-3 py-8 text-center sm:px-4 sm:py-10">
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

        <input 
          ref="fileInput"
          type="file"
          multiple
          accept="image/*"
          @change="handleFileSelect"
          class="hidden"
        </div>

        <div class="content-panel home-panel-compact space-y-2.5">
          <div class="flex flex-col gap-2 border-b border-slate-200/70 pb-2.5 dark:border-white/10 md:flex-row md:items-end md:justify-between">
            <div>
              <p class="panel-label">上传设置</p>
              <h2 class="section-title mt-1 text-base font-semibold text-slate-900 dark:text-white sm:text-lg">上传设置</h2>
            </div>
          </div>

          <div class="grid gap-2.5 xl:grid-cols-[minmax(0,0.82fr)_minmax(0,1.18fr)]">
            <div class="control-group control-group-compact">
            <p class="panel-label">上传目标</p>
            <p class="control-group-title">选择存储桶</p>
            <p class="control-group-hint">上传前先确定目标存储。</p>
            <select 
              class="input-modern mt-3"
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

            <div class="control-group control-group-compact">
            <p class="panel-label">标签区</p>
            <p class="control-group-title">给本次上传补充标签</p>
            <p class="control-group-hint">标签会跟随本次上传一起保存。</p>

            <div class="mt-2.5 space-y-2">
              <select 
                class="input-modern"
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

              <div class="relative flex w-full">
                <input 
                  type="text" 
                  placeholder="输入自定义标签"
                  class="input-modern flex-1 pr-14"
                  v-model="customTagInput"
                  @keyup.enter="addCustomTag"
                  maxlength="10"
                  :disabled="isUploading"
                >
                <button 
                  class="absolute right-1 top-1 inline-flex h-[calc(100%-8px)] items-center justify-center rounded-[16px] bg-slate-900 px-3.5 text-white transition hover:bg-slate-700 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-200"
                  @click="addCustomTag"
                  :disabled="isUploading || !customTagInput.trim()"
                >
                  <i class="ri-add-line"></i>
                </button>
              </div>

              <div class="tag-list flex flex-wrap gap-1">
                <div 
                  v-for="(tag, index) in selectedTags" 
                  :key="index"
                  class="flex items-center rounded-full bg-slate-900 px-2.5 py-1 text-sm text-white dark:bg-white dark:text-slate-900"
                >
                  <span>{{ tag }}</span>
                  <button 
                    class="ml-2 text-white/70 transition-colors hover:text-white dark:text-slate-500 dark:hover:text-slate-900"
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

              <div v-if="tagError" class="text-xs text-red-500 dark:text-red-400">
                {{ tagError }}
              </div>
            </div>
            </div>
          </div>
        </div>

        <div class="content-panel home-panel-compact">
          <div class="mb-3 flex flex-col gap-2 border-b border-slate-200/70 pb-3 dark:border-white/10 md:flex-row md:items-center md:justify-between">
            <div>
              <p class="panel-label">结果流</p>
              <h2 class="section-title mt-1 flex items-center gap-2 text-base font-semibold sm:text-lg">
                <i class="ri-gallery-line text-primary"></i>
                最近上传
              </h2>
              <p class="mt-1 text-sm text-slate-500 dark:text-slate-400">结果区保持紧凑，优先看图和复制链接。</p>
            </div>
            <span class="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-sm text-slate-600 dark:border-white/10 dark:bg-slate-950 dark:text-slate-300">{{ recentImages.length }} 张</span>
          </div>

      <div v-if="recentImages.length > 0" class="result-stream">
        <div
          v-for="image in recentImages" 
          :key="image.id"
          class="result-card result-card-compact result-card-mobile-safe"
        >
          <div class="result-card-layout">
            <div class="result-card-media result-card-media-large">
            <div class="loading absolute inset-0 z-0 flex items-center justify-center bg-gray-100 text-slate-300 dark:bg-gray-800">
              <svg class="w-8 h-8 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="transform: scaleX(-1) scaleY(-1);">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </div>
            <img 
              :src="getFullUrl(image.thumbnail || image.url)"
              :alt="image.filename || '图片预览'" 
              class="recent-image h-full w-full object-cover opacity-0"
              loading="lazy"
              @load="handleImageLoad"
              @error="(e) => handleImageError(e, image)"
              @click.stop="previewImage(image)"
            />
            </div>

            <div class="min-w-0 flex-1 space-y-2">
              <div class="flex flex-col gap-2 sm:gap-2.5 lg:flex-row lg:items-start lg:justify-between">
                <div class="min-w-0">
                  <p class="truncate text-sm font-medium text-slate-900 dark:text-white">{{ image.filename }}</p>
                  <div class="mt-1 flex flex-wrap items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
                    <span class="result-meta-pill">{{ formatFileSize(image.file_size) }}</span>
                    <span class="result-meta-pill">{{ image.width }}×{{ image.height }}</span>
                  </div>
                </div>
                <div class="flex items-center justify-end gap-1.5 sm:self-end lg:justify-end">
                  <button 
                    @click.stop="downloadImage(image)"
                    class="result-card-action"
                    title="下载图片"
                  >
                    <i class="ri-download-line text-sm"></i>
                  </button>
                  <button 
                    @click.stop="deleteImage(image.id)"
                    class="result-card-action border-red-200 bg-red-50 text-red-500 hover:bg-red-100 dark:border-red-500/20 dark:bg-red-500/10 dark:text-red-300"
                    title="删除图片"
                  >
                    <i class="ri-delete-bin-line text-sm"></i>
                  </button>
                </div>
              </div>

              <div class="result-links-grid result-links-grid-mobile">
            <div class="link-field cursor-pointer"
              @click.stop="copyImageLink(image, 'url')"
              title="点击复制URL"
            >
              <i class="ri-link text-sm text-slate-400"></i>
              <span class="w-8 shrink-0 font-medium text-slate-900 dark:text-white sm:w-10">URL</span>
              <span class="truncate">{{ getFullUrl(image.url) }}</span>
            </div>

            <div class="link-field cursor-pointer"
              @click.stop="copyImageLink(image, 'html')"
              title="点击复制HTML"
            >
              <i class="ri-code-line text-sm text-slate-400"></i>
              <span class="w-8 shrink-0 font-medium text-slate-900 dark:text-white sm:w-10">HTML</span>
              <span class="truncate">{{ getHtmlCode(image) }}</span>
            </div>

            <div class="link-field cursor-pointer"
              @click.stop="copyImageLink(image, 'markdown')"
              title="点击复制Markdown"
            >
              <i class="ri-markdown-line text-sm text-slate-400"></i>
              <span class="w-8 shrink-0 font-medium text-slate-900 dark:text-white sm:w-10">MD</span>
              <span class="truncate">{{ getMarkdownCode(image) }}</span>
            </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 无图片状态 -->
      <div v-else class="py-8 text-center">
        <div class="mb-2.5 text-5xl text-light-300 dark:text-dark-100">
          <i class="ri-image-line"></i>
        </div>
        <p class="mb-3 text-base text-secondary">暂无上传的图片</p>
      </div>
      </div>
      </section>
    </div>
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
 * 从URL上传图片
 */
const uploadbyurlmodal = () => {
  // 构建标签选项
  const tagList = [
    { value: "0", label: "不添加"}
  ];
  presetTags.value.forEach(tag => {
    tagList.push({ value: tag.id, label: tag.name });
  });
  const storageList = [];
  presetBuckets.value.forEach(storage => {
    storageList.push({ value: storage.id, label: storage.name });
  })
  const modal = new PopupModal({
    title: '从URL上传图片',
    type: 'form',
    formFields: [
      {
        name: 'url',
        label: '图片链接',
        type: 'text',
        required: true,
        placeholder: '请输入图片链接'
      },
      {
        name: 'tag_id',
        label: 'Tag标签',
        type: 'select',
        required: true,
        defaultValue: "0",
        options: tagList
      },
      {
        name: 'bucket_id',
        label: '存储',
        type: 'select',
        required: true,
        defaultValue: "1",
        options: storageList
      }
    ],
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => {
          modal.close();
        }
      },
      {
        text: '确定',
        type: 'primary',
        callback: (modal) => {
          const formData = serializeForm(modal);
          if(formData['url'].length === 0) {
            Message.error('请输入图片链接');
            return
          }
          postuploadbyurl(formData);
          modal.close();
        }
      }
    ]
  });
  modal.open();
}

const postuploadbyurl = async (formData) => {
  try {
    const res = await fetch(`/api/images/url`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: JSON.stringify(formData)
    });
    const result = await res.json();
    if (res.ok && result.code === 200) {
      await loadRecentImages();
      Message.success('上传成功');
    } else {
      throw new Error(result.message || '上传失败');
    }
  } catch (err) {
    console.error(err);
    Message.error(err.message || '上传失败');
  }
}

/**
 * 序列化表单数据
 * @param {Object} modal - 弹窗实例
 * @returns {Object} 表单数据对象
 */
const serializeForm = (modal) => {
  const form = modal.content?.querySelector('form');
  if (!form) {
    console.warn('未找到表单元素');
    return {};
  }

  return Array.from(form.elements).reduce((acc, element) => {
    const { name, disabled, type, checked, value } = element;
    
    // 跳过无name、禁用的元素
    if (!name || disabled) return acc;
    
    // 处理复选框/单选框
    if ((type === 'checkbox' || type === 'radio') && !checked) return acc;
    
    // 处理文件输入
    if (type === 'file') {
      acc[name] = element.files.length > 0 ? element.files[0].name : '';
      return acc;
    }
    
    // 处理多值字段
    if (acc[name]) {
      acc[name] = Array.isArray(acc[name]) ? [...acc[name], value] : [acc[name], value];
    } else {
      acc[name] = value;
    }
    
    return acc;
  }, {});
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
