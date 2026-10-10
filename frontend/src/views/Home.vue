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
              <span class="rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 dark:border-white/10 dark:bg-slate-950">
                {{ multiStorageSync ? `本机 + ${syncBuckets.length} 个同步目标` : (presetBuckets.find(bucket => bucket.id == selectedBucket)?.name || '未选择存储') }}
              </span>
              <span v-if="foldersAuthenticated" class="max-w-full truncate rounded-full border border-slate-200 bg-slate-50 px-2.5 py-1 dark:border-white/10 dark:bg-slate-950" :title="uploadFolderName">{{ uploadFolderName }}</span>
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
            <p v-if="isGuest" class="mx-auto mb-3 max-w-md text-xs text-secondary">游客图片依赖本浏览器的安全 Cookie。清除 Cookie 或更换浏览器后无法找回，请注册账户保存长期身份。</p>
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
            <p class="text-secondary text-sm mb-3">{{ uploadSummary }}</p>
            <template v-if="legacyUploading">
              <progress class="block h-2 w-full max-w-md mx-auto accent-blue-600" :value="uploadProgress == null ? undefined : uploadProgress" max="100" aria-label="文件传输进度"></progress>
              <button class="soft-button mt-3 px-3 py-1.5 text-xs" type="button" @click.stop="cancelLegacyUpload()">停止本次请求</button>
              <p class="mt-2 text-xs text-slate-500">停止请求不保证撤回服务器已接收的文件，请检查最近上传。</p>
            </template>
          </div>
        </div>

        <input 
          ref="fileInput"
          type="file"
          multiple
          :accept="uploadAccept"
          @change="handleFileSelect"
          class="hidden"
        />

        </div>

        <div class="content-panel home-panel-compact space-y-2.5">
          <div class="flex flex-col gap-2 border-b border-slate-200/70 pb-2.5 dark:border-white/10 md:flex-row md:items-end md:justify-between">
            <div>
              <p class="panel-label">上传设置</p>
              <h2 class="section-title mt-1 text-base font-semibold text-slate-900 dark:text-white sm:text-lg">上传设置</h2>
            </div>
          </div>

          <div class="grid gap-2.5" :class="foldersAuthenticated ? 'xl:grid-cols-[minmax(0,0.82fr)_minmax(0,1.18fr)]' : ''">
            <div class="control-group control-group-compact">
            <template v-if="multiStorageSync">
              <p class="panel-label">存储流程</p>
              <p class="control-group-title">先保存到本机，再后台同步</p>
              <p class="control-group-hint">文件上传成功后可立即使用，远程存储由后台异步处理。</p>
              <div class="mt-3 flex flex-wrap gap-2">
                <span class="inline-flex items-center gap-1.5 rounded-full border border-emerald-200 bg-emerald-50 px-2.5 py-1 text-xs text-emerald-700 dark:border-emerald-500/20 dark:bg-emerald-500/10 dark:text-emerald-300">
                  <i class="ri-computer-line"></i>本机
                </span>
                <span
                  v-for="bucket in syncBuckets"
                  :key="bucket.id"
                  class="inline-flex items-center gap-1.5 rounded-full border border-slate-200 bg-white px-2.5 py-1 text-xs text-slate-600 dark:border-white/10 dark:bg-slate-950 dark:text-slate-300"
                >
                  <i class="ri-cloud-line"></i>{{ bucket.name }}
                </span>
                <span v-if="syncBuckets.length === 0" class="text-xs text-slate-400 dark:text-slate-500">未配置远程同步源</span>
              </div>
            </template>
            <template v-else>
              <p class="panel-label">上传目标</p>
              <p class="control-group-title">选择存储桶</p>
              <p class="control-group-hint">上传前先确定目标存储。</p>
              <select id="upload-bucket" aria-label="上传存储"
                class="input-modern mt-3"
                v-model="selectedBucket"
                :disabled="isGuest || isUploading"
                @change="handleBucketChange"
              >
                <option
                  v-for="bucket in presetBuckets"
                  :key="bucket.id"
                  :value="bucket.id"
                >{{ bucket.name }}  ({{ bucket.type }})</option>
              </select>
            </template>
          </div>

            <div v-if="foldersAuthenticated" class="control-group control-group-compact min-w-0">
              <p class="panel-label">图片归档</p>
              <label for="upload-folder" class="control-group-title block">上传文件夹</label>
              <p class="control-group-hint">按文件夹整理本次上传，不改变图片链接或存储位置。</p>
              <div class="mt-3 flex min-w-0 flex-col gap-2 sm:flex-row">
                <select id="upload-folder" v-model="selectedFolder" class="input-modern min-w-0 flex-1" :disabled="isUploading || foldersLoading || !foldersLoaded">
                  <option value="0">未分类</option>
                  <option v-for="folder in folders" :key="folder.id" :value="String(folder.id)" :disabled="folder.deleting">{{ folderLabel(folder) }}</option>
                </select>
                <button type="button" class="soft-button shrink-0" :disabled="isUploading || foldersLoading || !foldersLoaded || foldersAtLimit" @click="createFolderOpen = true"><i class="ri-folder-add-line" aria-hidden="true"></i>新建文件夹</button>
              </div>
              <p v-if="foldersAtLimit" class="field-hint">已达到 200 个文件夹上限。</p>
              <p v-if="foldersError" role="alert" class="field-hint text-red-600">{{ foldersError }} <button type="button" class="underline" :disabled="isUploading" @click="reloadFolders">重试</button></p>
            </div>
          </div>
        </div>

        <div v-if="unlinkedDirectTasks.length || directTasksError" class="content-panel home-panel-compact space-y-2.5">
          <div class="flex flex-wrap items-center justify-between gap-2">
            <h2 class="text-sm font-semibold text-slate-900 dark:text-white">上传任务</h2>
            <button class="soft-button px-2 py-1 text-xs" type="button" :disabled="directRefreshing" @click="refreshDirectTasks">刷新状态</button>
          </div>
          <p class="text-xs text-slate-500 dark:text-slate-400">显示最近 {{ MAX_RECENT_TASKS }} 个任务；文件传输、后台主图处理和缩略图分别显示。已到达服务器的任务可刷新恢复。</p>
          <p v-if="directTasksError" class="text-xs text-amber-700 dark:text-amber-300" role="status">{{ directTasksError }}</p>
          <DirectUploadTaskStatus v-for="task in unlinkedDirectTasks" :key="task.client_id || task.id" :task="task"
            @retry="retryDirectTask" @cancel="cancelDirectTask" @confirm="confirmDirectTask" @reselect="reselectDirectFile" />
        </div>

        <div class="content-panel home-panel-compact">
          <div class="mb-3 flex flex-col gap-2 border-b border-slate-200/70 pb-3 dark:border-white/10 md:flex-row md:items-center md:justify-between">
            <div>
              <p class="panel-label">结果流</p>
              <h2 class="section-title mt-1 flex items-center gap-2 text-base font-semibold sm:text-lg">
                <i class="ri-gallery-line text-primary"></i>
                最近上传
              </h2>
            </div>
            <span class="rounded-full border border-slate-200 bg-slate-50 px-3 py-1 text-sm text-slate-600 dark:border-white/10 dark:bg-slate-950 dark:text-slate-300">{{ recentImages.length }} 张</span>
          </div>

      <div v-if="recentImages.length > 0" class="result-stream">
        <RecentImageCard v-for="image in recentImages" :key="image.id" v-bind="{ image, multiStorageSync, formatFileSize, getFullUrl, handleImageLoad, handleImageError, previewImage, downloadImage, deleteImage, directTaskForImage, retryDirectTask, cancelDirectTask, confirmDirectTask, reselectDirectFile, getStorageStatuses, getStorageStatusMeta, getStorageDisplayName, copyImageLink, getHtmlCode, getMarkdownCode }" />
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
    <FolderDialog v-if="createFolderOpen && foldersAuthenticated" mode="create" :folders="folders" :account-key="folderAccountKey" @close="createFolderOpen = false" @saved="onFolderCreated" />
  </div>
</template>

<script setup>
import { escapeHtml, safeResourceUrl } from '@/utils/html.js'

import { boundedPage, RECENT_IMAGE_LIMIT } from '@/utils/renderBounds.js';
import { uploadLimits, allowedRasterTypes } from '@/utils/uploadValidation.js';
import { readApiResponse } from '@/utils/apiFeedback.js';
import errorImg from '@/assets/images/error.webp';

import { ref, computed, onMounted, nextTick, onBeforeUnmount, watch } from 'vue'
import { useRoute } from 'vue-router';
import FolderDialog from '@/components/FolderDialog.vue';
import { useFolders } from '@/composables/useFolders.js';
import { folderFilter, folderAccount, folderLabel } from '@/utils/folders.js';
import DirectUploadTaskStatus from '@/components/DirectUploadTaskStatus.vue';
import { createHomeDirectTasks } from '@/utils/homeDirectTasks.js';
import RecentImageCard from '@/components/home/RecentImageCard.vue';
import { createHomeUploadActions } from '@/utils/homeUploadActions.js';
import { createHomeDialogs } from '@/utils/homeDialogs.js';
import Message from '@/utils/message.js';
import { createDialogScope } from '@/utils/dialogScope.js';
const { Dialog: PopupModal, dispose: disposeViewDialogs } = createDialogScope();
import Loading from '@/utils/loading.js';
import { MAX_RECENT_TASKS, mergeImages } from '@/utils/directUpload.js'
import { getStorageDisplayName, getStorageStatuses, getStorageStatusMeta, hasActiveStorageSync } from '@/utils/storageStatus.js'

// ====================== 常量定义 ======================
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';
const authHeaders = () => ({});
const isGuest = computed(() => Number(JSON.parse(localStorage.getItem('userInfo') || '{}').role) === 2);


// ====================== 响应式数据 ======================
// 上传相关
const isDragOver = ref(false);
const legacyUploading = ref(false);
const directTasks = ref([]);
const isUploading = computed(() => legacyUploading.value || directTasks.value.some(task => task.busy));
const uploadConfig = ref({});
const clientUploadLimits = computed(() => uploadLimits(uploadConfig.value));
const uploadAccept = computed(() => allowedRasterTypes(uploadConfig.value).join(','));
const uploadingCount = ref(0);
const uploadProgress = ref(0);
const recentImages = ref([]);
const fileInput = ref(null);

// Folder state belongs to this mounted account only; never persist upload destinations.
const route = useRoute();
const { account: folderAccountKey, authenticated: foldersAuthenticated, folders, loading: foldersLoading,
  loaded: foldersLoaded, error: foldersError, atLimit: foldersAtLimit, reload: reloadFolders } = useFolders(API_BASE_URL);
const selectedFolder = ref('0');
const createFolderOpen = ref(false);
let initialFolderPending = true;
const uploadFolderName = computed(() => folders.value.find(folder => String(folder.id) === selectedFolder.value)?.name || '未分类');
watch(folderAccountKey, () => { selectedFolder.value = '0'; createFolderOpen.value = false; initialFolderPending = false; });
watch([folders, foldersLoaded], () => {
  if (!foldersLoaded.value) return;
  const requested = initialFolderPending ? folderFilter(route.query.folder_id, '0') : selectedFolder.value;
  initialFolderPending = false;
  selectedFolder.value = folders.value.some(folder => !folder.deleting && String(folder.id) === requested) ? requested : '0';
});
// Resolve at upload start, also rejecting stale IDs if another tab changes account.
const getUploadFolderId = () => foldersAuthenticated.value && folderAccount() === folderAccountKey.value && folders.value.some(folder => !folder.deleting && String(folder.id) === selectedFolder.value) ? Number(selectedFolder.value) : 0;
async function onFolderCreated({ folder }) {
  createFolderOpen.value = false;
  await reloadFolders();
  if (!disposed && folders.value.some(item => !item.deleting && item.id === folder?.id)) selectedFolder.value = String(folder.id);
}

// 存储相关
const multiStorageSync = ref(false);
const storageConfigLoaded = ref(false);
const presetBuckets = ref([
  { id: "1", name: '默认存储', type: "default" },
]);
const selectedBucket = ref("1");
const syncBuckets = ref([]);

// 预览相关
const activeCopyMenu = ref(null);
let previewCopyMenu = false;

let syncPollTimer = null;
let disposed = false;
const { directRefreshing, directTasksError, resumeTask, deletedImageIds, currentUser, realUser,
  directEnabled, uploadSummary, directTaskForImage, unlinkedDirectTasks, usableTasks,
  refreshDirectTasks, startDirectFiles, retryDirectTask, cancelDirectTask, confirmDirectTask,
  reselectDirectFile, handleResumeFile, disposeDirectTasks } = createHomeDirectTasks({
  API_BASE_URL, directTasks, recentImages, legacyUploading, uploadProgress, uploadingCount,
  multiStorageSync, storageConfigLoaded, presetBuckets, selectedBucket, getUploadFolderId, fileInput,
  validateFiles: files => validateFiles(files), loadRecentImages: () => loadRecentImages(),
  isDisposed: () => disposed,
  onSingleUpload: showUploadResult,
});

// ====================== 工具函数 ======================
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
  return `<img loading="lazy" decoding="async" src="${escapeHtml(safeResourceUrl(url))}" alt="${escapeHtml(alt)}"/>`;
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
        'X-Requested-With': 'XMLHttpRequest'
      }
    });
    
    const result = await readApiResponse(response, '请求失败');
    if (response.ok && result.code === 200) {
      uploadConfig.value = result.data || {};
      presetBuckets.value = Array.isArray(result.data?.buckets) ? result.data.buckets : [];
      multiStorageSync.value = result.data?.multi_storage_sync === true;
      const configuredSyncBuckets = result.data?.sync_buckets ?? result.data?.buckets ?? [];
      // 本机作为固定的第一落点，不重复列入远程同步目标。
      syncBuckets.value = Array.isArray(configuredSyncBuckets)
        ? configuredSyncBuckets.filter(bucket => bucket?.type !== 'default')
        : [];

      if (!multiStorageSync.value) {
        const defaultBucket = result.data?.default_bucket || '1';
        const storedBucketId = localStorage.getItem('currentBucket');
        const storedBucket = storedBucketId == null
          ? null
          : presetBuckets.value.find(bucket => bucket.id === Number(storedBucketId));
        selectedBucket.value = storedBucket?.id || defaultBucket;
      }
      storageConfigLoaded.value = true;
      scheduleSyncRefresh();
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
    const response = await fetch(`${API_BASE_URL}/api/images?scope=mine&limit=12`, {
      headers: {
        'X-Requested-With': 'XMLHttpRequest'
      }
    });
    
    const result = await readApiResponse(response, '加载图片失败');
    if (disposed) return;
    recentImages.value = boundedPage(mergeImages(Array.isArray(result.data?.images) ? result.data.images : [], usableTasks()), RECENT_IMAGE_LIMIT);
    scheduleSyncRefresh();
    if (foldersAuthenticated.value) reloadFolders();
  } catch (error) {
    if (disposed) return;
    console.error('加载图片失败:', error);
    Message.error(`加载图片失败: ${error.message}`, {
      duration: 3000,
      position: 'top-right',
      showClose: true
    });
  }
};

const scheduleSyncRefresh = () => {
  if (disposed || !multiStorageSync.value) {
    if (syncPollTimer) clearTimeout(syncPollTimer);
    syncPollTimer = null;
    return;
  }
  const hasActiveSync = recentImages.value.some(hasActiveStorageSync);
  if (!hasActiveSync) {
    if (syncPollTimer) clearTimeout(syncPollTimer);
    syncPollTimer = null;
    return;
  }
  if (syncPollTimer) return;

  syncPollTimer = setTimeout(async () => {
    syncPollTimer = null;
    await loadRecentImages();
  }, 2500);
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
        'X-Requested-With': 'XMLHttpRequest',
        'Content-Type': 'application/json'
      }
    });
    
    const result = await readApiResponse(response, '请求失败');
    if (response.ok && result.code === 200) {
      deletedImageIds.add(String(imageId));
      while (deletedImageIds.size > 64) deletedImageIds.delete(deletedImageIds.values().next().value);
      recentImages.value = recentImages.value.filter(image => String(image.id) !== String(imageId));
      Message.success('图片删除成功', {
        duration: 1500,
        position: 'top-right'
      });
      
      closeDeletedPreview(imageId);
      previewCopyMenu = false;
      activeCopyMenu.value = null;
      await loadRecentImages();
    } else {
      throw new Error(result.message || '删除失败；部分存储源可能已删除，请重试');
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

// ====================== 事件处理函数 ======================
/**
 * 拖拽相关处理
 */
const { handleDragOver, handleDragEnter, handleDragLeave, handleDrop, triggerFileInput,
  handleFileSelect, handlePaste, validateFiles, uploadFiles, cancelLegacyUpload } = createHomeUploadActions({
  API_BASE_URL, isDragOver, isUploading, fileInput, resumeTask, handleResumeFile, clientUploadLimits,
  uploadConfig, storageConfigLoaded, directEnabled, startDirectFiles, legacyUploading, uploadingCount,
  uploadProgress, getUploadFolderId, multiStorageSync, selectedBucket, recentImages, loadRecentImages,
  isDisposed: () => disposed,
  onSingleUpload: showUploadResult,
});

/**
 * 图片相关操作
 */
const handleImageLoad = (e) => {
  e.target.classList.remove('opacity-0');
  const loadingEl = e.target.parentElement.querySelector('.loading');
  if (loadingEl) loadingEl.classList.add('hidden');
};

const handleImageError = (e) => {
  if (e.target.dataset.fallbackApplied) return;
  e.target.dataset.fallbackApplied = 'true';
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
      copyText = `<img loading="lazy" decoding="async" src="${fullUrl}" alt="${escapeHtml(image.filename)}" width="${image.width || ''}" height="${image.height || ''}">`;
      break;
    case 'markdown':
      copyText = `![${escapeHtml(image.filename)}](${fullUrl})`;
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
 * 单存储模式下记住用户选择的存储桶。
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
  const downloadUrl = new URL(fullUrl, window.location.origin);
  downloadUrl.searchParams.set('download', '1');
  link.href = downloadUrl.toString();
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
/**
 * 从URL上传图片
 */
const { disposeDialogs, previewImage, uploadbyurlmodal, cleanupPreview, closeDeletedPreview } = createHomeDialogs({
  storageConfigLoaded, getUploadFolderId, folders, foldersAuthenticated, presetBuckets, selectedBucket, multiStorageSync, getFullUrl,
  errorImg, formatFileSize, formatDate, copyImageLink, downloadImage, deleteImage, loadRecentImages,
  onSingleUpload: showUploadResult, isDisposed: () => disposed,
});

function showUploadResult(image) {
  if (disposed || !image?.id || !image.url) return;
  // Resolve the uploaded ID, never whichever image happens to be first in the recent list.
  const recent = recentImages.value.find(item => String(item.id) === String(image.id));
  previewImage({
    ...image, ...recent,
    thumbnail: recent?.thumbnail || image.thumbnail || image.thumbnail_url || '',
    bucket_id: recent?.bucket_id ?? image.bucket_id ?? selectedBucket.value,
    uploader_role: recent?.uploader_role ?? image.uploader_role ?? currentUser().role,
    tags: recent?.tags ?? image.tags ?? [],
  }, { title: '上传成功', showLink: true });
}

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
  loadRecentImages();
  refreshDirectTasks();
  
  // 注册全局事件
  document.addEventListener('paste', handlePaste);
  document.addEventListener('click', handleGlobalClick);
});

onBeforeUnmount(() => {
  disposed = true;
  disposeDirectTasks();
  cancelLegacyUpload();
  if (syncPollTimer) clearTimeout(syncPollTimer);
  
  // 移除事件监听
  document.removeEventListener('paste', handlePaste);
  document.removeEventListener('click', handleGlobalClick);
  
  // 清理预览资源
  cleanupPreview();
  disposeDialogs();
  disposeViewDialogs();
  
  // 关闭所有消息提示
  if (window.Message) {
    Message.closeAll();
  }
});
</script>
