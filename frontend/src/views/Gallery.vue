<template>
  <div class="page-shell text-gray-800 dark:text-gray-200">
    <section class="page-header border-b border-slate-200/70 pb-3 dark:border-white/10">
      <div>
        <p class="panel-label">Gallery Manager</p>
        <h1 class="page-title">图库管理台</h1>
      </div>
    </section>

    <div class="space-y-2.5 lg:space-y-3">
      <div class="content-panel gallery-panel-compact gallery-topbar-compact space-y-2">
        <div class="gallery-topbar-minimal">
          <div class="gallery-topbar-filters">
            <div v-if="isAdmin" class="gallery-inline-control">
              <span class="gallery-inline-label">角色</span>
              <div class="role-buttons grid w-full grid-cols-2 overflow-hidden rounded-[16px] border border-slate-200 bg-white dark:border-white/10 dark:bg-slate-900 sm:inline-flex sm:w-auto">
            <button
              @click="changeRole('admin')"
              class="px-3 py-1.5 text-sm transition-all"
              :class="[
                roleImage === 'admin' 
                  ? 'bg-slate-900 text-white dark:bg-white dark:text-slate-900' 
                  : 'bg-transparent hover:bg-slate-100 dark:hover:bg-white/10'
              ]"
            >
              管理员
            </button>
            <button
              @click="changeRole('guest')"
              class="px-3 py-1.5 text-sm transition-all"
              :class="[
                roleImage === 'guest' 
                  ? 'bg-slate-900 text-white dark:bg-white dark:text-slate-900' 
                  : 'bg-transparent hover:bg-slate-100 dark:hover:bg-white/10'
              ]"
            >
              游客
            </button>
            <button
              @click="changeRole('user')"
              class="px-3 py-1.5 text-sm transition-all"
              :class="[
                roleImage === 'user' 
                  ? 'bg-slate-900 text-white dark:bg-white dark:text-slate-900' 
                  : 'bg-transparent hover:bg-slate-100 dark:hover:bg-white/10'
              ]"
            >
              用户
            </button>
          </div>
            </div>

            <div class="gallery-inline-control gallery-inline-control-select">
              <span class="gallery-inline-label">存储</span>
          <select 
            class="input-modern gallery-inline-select"
            v-model="selectedBucket"
            @change="loadImages"
          >
            <option value="null">全部</option>
            <option 
              v-for="bucket in presetBuckets" 
              :key="bucket.id"
              :value="bucket.id"
            >{{ bucket.name }}</option>
          </select>
            </div>

            <div class="gallery-inline-control gallery-inline-control-tags">
              <span class="gallery-inline-label">标签</span>
          <div class="flex flex-wrap gap-1.5">
            <div class="filter-chip"
            :class="isTagSelected(0) ? 'filter-chip-active' : ''"
            @click="handleTagSelection(0)">
                <span>默认</span>
            </div>
            <div
            v-if="presetTags.length > 0"
            v-for="tag in presetTags"
            class="filter-chip"
            :class="isTagSelected(tag.id) ? 'filter-chip-active' : ''"
            @click="handleTagSelection(tag.id)">
                <span>{{tag.name}}</span>
            </div>
          </div>
            </div>
          </div>

          <div class="gallery-topbar-actions">
            <div class="gallery-topbar-stats">
              <label v-if="images.length > 0" for="selectAll" class="gallery-topbar-stat gallery-topbar-stat-action">
                <input
                  type="checkbox"
                  id="selectAll"
                  class="h-4 w-4 rounded border-gray-300 text-primary focus:ring-primary"
                  v-model="selectAll"
                  @change="handleSelectAll"
                >
                <span>全选</span>
              </label>
              <span class="gallery-topbar-stat">{{ ROLE_MAP[roleImage] }}</span>
              <span class="gallery-topbar-stat">{{ images.length }} 张</span>
              <span class="gallery-topbar-stat">已选 {{ selectedImages.length }}</span>
            </div>
          <div v-if="selectedImages.length > 0" class="batch-actions flex w-full flex-col gap-2 sm:flex-row sm:items-center xl:w-auto">
            <button
              @click="handleBatchCopy"
              class="soft-button">
              <i class="ri-file-copy-line"></i>
              批量复制
            </button>
            <button
              @click="handleBatchSetAccessSource"
              class="soft-button">
              <i class="ri-route-line"></i>
              批量设置访问源
            </button>
            <button
            @click="handleBatchSetTag"
            class="soft-button">
                <i class="ri-bookmark-2-fill"></i>
                批量设置Tag
            </button>
            <button
              @click="handleBatchDelete"
              class="danger-button"
            >
              <i class="ri-delete-bin-fill"></i>
              删除 ({{ selectedImages.length }})
            </button>
          </div>
          </div>
          </div>
      </div>

      <section class="space-y-3">
      <div v-if="loading" class="content-panel loading-container flex flex-col items-center justify-center py-12 sm:py-14">
        <div class="spinner w-10 h-10 border-4 border-gray-200 dark:border-gray-700 border-t-primary dark:border-t-primary rounded-full animate-spin mb-4"></div>
        <p class="text-gray-600 dark:text-gray-400">加载中...</p>
      </div>
      
      <div v-else-if="images.length > 0" class="content-panel gallery-panel-compact images-container">
        <div v-if="viewMode === 'grid'" class="images-grid grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6">
          <div 
            v-for="image in images" 
            :key="image.id"
            class="gallery-image-card gallery-image-card-compact"
            :class="isImageSelected(image.id) ? 'border-slate-900 dark:border-white' : 'hover:border-slate-300 dark:hover:border-white/20'"
          >
            <div class="gallery-image-card-head">
              <div class="gallery-card-badges">
                <span 
                  class="image-role gallery-card-badge"
                  :class="getRoleTagClass(image.uploader_role)"
                >
                  {{ image.uploader_role == '1' ? '管理员' : (image.uploader_role == '3' ? '用户' : '游客') }}
                </span>
                <span
                  v-if="multiStorageSync"
                  class="gallery-card-badge inline-flex items-center gap-1 border"
                  :class="getStorageSyncSummary(image).badgeClass"
                >
                  <i :class="getStorageSyncSummary(image).icon"></i>
                  {{ getStorageSyncSummary(image).label }}
                </span>
                <span v-else class="gallery-card-badge gallery-card-badge-dark">
                  {{ presetBuckets.find(bucket => bucket.id == image.bucket_id)?.name }}
                </span>
              </div>
              <label class="gallery-card-checkbox" :for="`image-${image.id}`" @click.stop>
                <input
                  type="checkbox"
                  :id="`image-${image.id}`"
                  class="h-4 w-4 rounded border-gray-300 bg-white text-primary focus:ring-primary dark:bg-gray-800"
                  :checked="isImageSelected(image.id)"
                  @change="(e) => handleImageSelection(image.id, e.target.checked)"
                  @click.stop
                >
              </label>
            </div>

            <div class="image-wrapper relative aspect-square overflow-hidden bg-gray-100 dark:bg-gray-950" @click="openPreview(image)">
              <div class="loading absolute inset-0 flex items-center justify-center z-0 text-slate-300">
                <svg class="w-8 h-8 animate-spin" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="transform: scaleX(-1) scaleY(-1);">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
              </div>
              <img 
                :src="image.thumbnail || image.url" 
                :alt="image.filename"
                class="image-thumbnail h-full w-full object-cover opacity-0"
                loading="lazy"
                @load="handleImageLoad"
                @error="handleImageError"
              />
            </div>
            <div class="image-info gallery-image-info-compact p-3">
              <p class="image-filename overflow-hidden truncate whitespace-nowrap text-sm font-medium">{{ image.filename }}</p>
              <p class="gallery-image-card-meta gallery-image-card-meta-inline">
                {{ formatFileSize(image.file_size) }} • 
                {{ image.width }}×{{ image.height }}
              </p>
              <p class="gallery-image-card-meta">{{ formatDate(image.created_at) }}</p>
              <div class="mt-2" @click.stop>
                <label :for="`access-source-${image.id}`" class="mb-1 flex items-center gap-1 text-[11px] font-medium text-slate-600 dark:text-slate-300">
                  <i class="ri-route-line"></i>
                  访问链接读取源
                </label>
                <select
                  :id="`access-source-${image.id}`"
                  class="w-full rounded-lg border border-slate-200 bg-white px-2 py-1.5 text-[11px] text-slate-700 outline-none transition focus:border-primary focus:ring-1 focus:ring-primary/20 disabled:cursor-wait disabled:opacity-60 dark:border-white/10 dark:bg-slate-950 dark:text-slate-200"
                  :value="getSelectedAccessBucketId(image)"
                  :disabled="isAccessSourceUpdating(image.id) || getAccessSourceOptions(image).length === 0"
                  @change="handleAccessSourceChange(image, $event)"
                  @click.stop
                >
                  <option
                    v-for="source in getAccessSourceOptions(image)"
                    :key="`${image.id}-access-${source.bucket_id}`"
                    :value="source.bucket_id"
                    :disabled="source.bucket_disabled || source.access_unavailable"
                  >
                    {{ getAccessSourceOptionLabel(source) }}
                  </option>
                </select>
              </div>
              <div v-if="multiStorageSync" class="mt-2 space-y-1.5">
                <div class="flex min-w-0 items-center justify-between gap-2 rounded-lg border border-slate-200/80 bg-slate-50 px-2 py-1.5 dark:border-white/10 dark:bg-slate-950">
                  <span class="min-w-0 truncate text-[11px] font-medium text-slate-700 dark:text-slate-200">本机</span>
                  <span class="inline-flex shrink-0 items-center gap-1 text-[10px] text-emerald-600 dark:text-emerald-300"><i class="ri-checkbox-circle-line"></i>已保存</span>
                </div>
                <div
                  v-for="storage in getStorageStatuses(image)"
                  :key="`${image.id}-${storage.bucket_id}`"
                  class="min-w-0 rounded-lg border border-slate-200/80 bg-slate-50 px-2 py-1.5 dark:border-white/10 dark:bg-slate-950"
                >
                  <div class="flex min-w-0 items-center justify-between gap-2">
                    <span class="min-w-0 truncate text-[11px] font-medium text-slate-700 dark:text-slate-200" :title="getStorageDisplayName(storage)">{{ getStorageDisplayName(storage) }}</span>
                    <span class="inline-flex shrink-0 items-center gap-1 rounded-full border px-1.5 py-0.5 text-[10px]" :class="getStorageStatusMeta(storage.status).badgeClass">
                      <i :class="getStorageStatusMeta(storage.status).icon"></i>{{ getStorageStatusMeta(storage.status).label }}
                    </span>
                  </div>
                  <p v-if="storage.status === 'failed' && storage.error" class="mt-1 truncate text-[10px] text-red-600 dark:text-red-300" :title="storage.error">{{ storage.error }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
        
        <div v-if="totalPages > 1" class="pagination flex flex-wrap items-center justify-center gap-2 py-4 sm:py-5">
          <button 
            @click="changePage(currentPage - 1)"
            :disabled="currentPage <= 1"
            class="soft-button"
            :class="{ 'opacity-50 cursor-not-allowed': currentPage <= 1 }"
          >
            上一页
          </button>
          
          <div class="page-numbers flex flex-wrap justify-center gap-1">
            <button 
              v-for="page in visiblePages"
              :key="page"
              @click="changePage(page)"
              class="flex h-9 w-9 items-center justify-center rounded-[16px] border text-sm transition-all"
              :class="[
                page === currentPage 
                  ? 'bg-slate-900 text-white border-slate-900 dark:bg-white dark:text-slate-900 dark:border-white' 
                  : 'border-gray-300 dark:border-gray-700 bg-white dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700'
              ]"
            >
              {{ page }}
            </button>
          </div>
          
          <button 
            @click="changePage(currentPage + 1)"
            :disabled="currentPage >= totalPages"
            class="soft-button"
            :class="{ 'opacity-50 cursor-not-allowed': currentPage >= totalPages }"
          >
            下一页
          </button>
        </div>
      </div>
      
      <div v-else class="content-panel empty-state flex flex-col items-center justify-center rounded-[22px] border border-dashed border-slate-300/80 bg-white/70 py-14 text-center dark:border-white/10 dark:bg-white/5">
        <div class="empty-icon mb-3 text-5xl text-gray-400 dark:text-gray-600">
          <i class="ri-image-ai-line"></i>
        </div>
        <h3 class="mb-2 text-lg font-bold">暂无图片</h3>
        <p class="mb-4 text-gray-600 dark:text-gray-400">
          还没有上传任何图片，
          <router-link to="/" class="text-primary hover:underline">去上传一些吧</router-link>
        </p>
      </div>
      </section>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed, onUnmounted, unref, watch } from 'vue'
import { useRouter } from 'vue-router'
import errorImg from '@/assets/images/error.webp';
import {
  getStorageDisplayName,
  getStorageStatuses,
  getStorageStatusMeta,
  getStorageSyncSummary,
  hasActiveStorageSync,
  renderStorageStatusesHtml,
} from '@/utils/storageStatus.js'

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';
const PAGE_SIZE = 20;
const ROLE_MAP = {
  admin: '管理员',
  guest: '游客',
  user: '用户'
};
const STORAGE_MAP = {
  default: '本地'
};

const getFullUrl = (path) => {
  if (!path) return '';
  if (typeof window === 'undefined') return path;
  if (path.startsWith('http')) return path;
  return `${window.location.origin}${path}`;
};

const formatFileSize = (bytes) => {
  if (!bytes || isNaN(bytes)) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
};

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

const getRoleTagClass = (role) => {
  return role == '1' || role == '3'
    ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200' 
    : 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
};

const serializeForm = (modal) => {
  const form = modal.content?.querySelector('form');
  if (!form) {
    console.warn('未找到表单元素');
    return {};
  }
  return Array.from(form.elements).reduce((acc, element) => {
    const { name, disabled, type, checked, value } = element;
    if (!name || disabled) return acc;
    if ((type === 'checkbox' || type === 'radio') && !checked) return acc;
    if (type === 'file') {
      acc[name] = element.files.length > 0 ? element.files[0].name : '';
      return acc;
    }
    if (acc[name]) {
      acc[name] = Array.isArray(acc[name]) ? [...acc[name], value] : [acc[name], value];
    } else {
      acc[name] = value;
    }
    return acc;
  }, {});
};

const images = ref([]);
const loading = ref(false);
const viewMode = ref('grid');
const currentPage = ref(1);
const totalPages = ref(1);
const roleImage = ref("admin");
const isAdmin = ref(false);
const presetTags = ref([]);
const presetBuckets = ref([]);
const selectedBucket = ref(null);
const selectedImages = ref([]);
const selectedTags = ref([]);
const selectAll = ref(false);
const currentPreviewImage = ref(null);
const multiStorageSync = ref(false);
const accessSourceUpdatingIds = ref([]);
let syncPollTimer = null;

const router = useRouter();

const visiblePages = computed(() => {
  const pages = [];
  const start = Math.max(1, currentPage.value - 2);
  const end = Math.min(totalPages.value, currentPage.value + 2);
  for (let i = start; i <= end; i++) {
    pages.push(i);
  }
  return pages;
});

const getSelectedAccessBucketId = (image) => {
  const selected = Number(image?.access_bucket_id || 0);
  if (selected > 0) return selected;
  const localSource = Array.isArray(image?.storage_statuses)
    ? image.storage_statuses.find(source => source?.status === 'success' && source?.bucket_type === 'default' && !source?.bucket_disabled)
    : null;
  return Number(localSource?.bucket_id || image?.bucket_id || 0);
};

const getAccessSourceOptions = (image) => {
  const statuses = Array.isArray(image?.storage_statuses) ? image.storage_statuses : [];
  const options = statuses
    .filter(source => source?.bucket_id && source.status === 'success')
    .map(source => ({ ...source }));
  const selectedBucketId = getSelectedAccessBucketId(image);

  if (!options.some(source => Number(source.bucket_id) === selectedBucketId)) {
    const selectedStatus = statuses.find(source => Number(source?.bucket_id) === selectedBucketId);
    if (selectedStatus) {
      options.push({ ...selectedStatus, access_unavailable: true });
    }
  }

  if (options.length === 0 && image?.bucket_id) {
    const bucket = presetBuckets.value.find(item => Number(item.id) === Number(image.bucket_id));
    options.push({
      bucket_id: Number(image.bucket_id),
      bucket_name: bucket?.name || `存储源 #${image.bucket_id}`,
      bucket_type: bucket?.type || image.storage,
      bucket_disabled: bucket?.disabled === true,
      status: 'success',
    });
  }

  const unique = new Map();
  options.forEach(source => unique.set(Number(source.bucket_id), source));
  return Array.from(unique.values()).sort((left, right) => {
    if (left.bucket_type === 'default' && right.bucket_type !== 'default') return -1;
    if (left.bucket_type !== 'default' && right.bucket_type === 'default') return 1;
    return Number(left.bucket_id) - Number(right.bucket_id);
  });
};

const getAccessSourceOptionLabel = (source) => {
  const name = source?.bucket_type === 'default'
    ? `${source?.bucket_name || '本机'}（默认）`
    : getStorageDisplayName(source);
  if (source?.bucket_disabled) return `${name}（已停用，回退本机）`;
  if (source?.access_unavailable) return `${name}（不可用，回退本机）`;
  return name;
};

const isAccessSourceUpdating = imageId => accessSourceUpdatingIds.value.includes(Number(imageId));

const setAccessSourceUpdating = (imageId, updating) => {
  const id = Number(imageId);
  if (updating && !accessSourceUpdatingIds.value.includes(id)) {
    accessSourceUpdatingIds.value = [...accessSourceUpdatingIds.value, id];
  } else if (!updating) {
    accessSourceUpdatingIds.value = accessSourceUpdatingIds.value.filter(item => item !== id);
  }
};

watch(
  () => [selectedImages.value.length, images.value.length],
  ([selectedLen, imageLen]) => {
    selectAll.value = imageLen > 0 && selectedLen === imageLen;
  },
  { immediate: true }
);

const getTagsList = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/tags`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    const result = await response.json();
    if (response.ok && result.code === 200) {
      presetTags.value = result.data?.list || [];
    } else {
      throw new Error(result.message || '获取标签列表失败');
    }
  } catch (error) {
    console.error('获取标签失败:', error);
    Message.error(error.message || '获取标签列表失败');
  }
};

const getBucketsList = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/buckets/list`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    const result = await response.json();
    if (response.ok && result.code === 200) {
      presetBuckets.value = result.data || [];
    } else {
      throw new Error(result.message || '获取存储列表失败');
    }
  } catch (error) {
    console.error('获取存储列表失败:', error);
    Message.error(error.message || '获取存储列表失败');
  }
};

const loadImages = async () => {
  if (loading.value) return;
  loading.value = true;
  try {
    const params = new URLSearchParams({
      page: currentPage.value,
      limit: PAGE_SIZE,
      sort_by: 'created_at',
      sort_order: 'desc',
      role: isAdmin.value ? roleImage.value : '',
      tags: selectedTags.value,
      bucket: selectedBucket.value
    });
    const response = await fetch(`${API_BASE_URL}/api/images?${params}`, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    if (response.ok) {
      const result = await response.json();
      images.value = result.data.images || [];
      totalPages.value = result.data.total_pages || 1;
      selectedImages.value = [];
      scheduleSyncRefresh();
    } else {
      if (response.status === 401) {
        localStorage.removeItem('authToken');
        router.push('/login');
        Message.error('登录已过期，请重新登录');
        return;
      }
      throw new Error('加载图片失败');
    }
  } catch (error) {
    console.error('加载图片错误:', error);
    Message.error(`加载图片失败: ${error.message}`);
  } finally {
    loading.value = false;
  }
};

const getStorageMode = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/uploadConfig`, {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    const result = await response.json();
    multiStorageSync.value = response.ok && result.code === 200 && result.data?.multi_storage_sync === true;
  } catch (error) {
    console.error('获取多存储模式失败:', error);
    multiStorageSync.value = false;
  }
};

const scheduleSyncRefresh = () => {
  if (!multiStorageSync.value) {
    if (syncPollTimer) clearTimeout(syncPollTimer);
    syncPollTimer = null;
    return;
  }
  const hasActiveSync = images.value.some(hasActiveStorageSync);
  if (!hasActiveSync) {
    if (syncPollTimer) clearTimeout(syncPollTimer);
    syncPollTimer = null;
    return;
  }
  if (syncPollTimer) return;
  syncPollTimer = setTimeout(async () => {
    syncPollTimer = null;
    await loadImages();
  }, 2500);
};

const batchDeleteImages = async (deleteIds) => {
  const promises = deleteIds.map(id => deleteAsync(id));
  await Promise.allSettled(promises);
  loadImages();
};

const deleteAsync = async (id) => {
  const loadingInstance = Loading.show({
    text: '删除中...',
    color: '#ff4d4f',
    mask: true
  });
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/${id}`, {
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    if (response.ok) {
      Message.success('图片删除成功');
      selectedImages.value = selectedImages.value.filter(imageId => imageId !== id);
      loadImages();
      return true;
    } else {
      const result = await response.json();
      throw new Error(result.message || '删除失败');
    }
  } catch (error) {
    console.error('删除图片错误:', error);
    Message.error(`删除图片失败: ${error.message}`);
    return false;
  } finally {
    await loadingInstance.hide();
  }
};

const pustImageTag = async (imageId, values) => {
  const { tag } = values;
  if (tag === '0') {
    Message.warning('请选择Tag标签');
    return;
  }
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/tag`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: JSON.stringify({ id: imageId, tag })
    });
    const result = await response.json();
    if (response.ok && result.code === 200) {
      const image = images.value.find(item => item.id === imageId);
      if (image) {
        image.tags = image.tags.filter(item => item.id !== 0);
        const newTag = presetTags.value.find(item => item.id === Number(tag));
        if (newTag) image.tags.push(newTag);
        currentPreviewImage.value = image;
        if (image) openPreview(image);
      }
      Message.success(result.message || '添加成功');
    } else {
      Message.error(result.message || '添加失败');
      openPreview(currentPreviewImage.value);
    }
  } catch (err) {
    Message.error(`出错了：${err.message}`);
    console.warn(err);
  }
};

const deleteImageTagAsync = async (imageId, tagId) => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/tag`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: JSON.stringify({ id: imageId, tag: tagId })
    });
    const result = await response.json();
    if (response.ok && result.code === 200) {
      const image = images.value.find(item => item.id === imageId);
      if (image) {
        image.tags = image.tags.filter(item => item.id !== tagId);
        if (image.tags.length === 0) {
          image.tags.push({ id: 0, name: '默认' });
        }
        currentPreviewImage.value = image;
      }
      Message.success(result.message || '删除成功');
      return true;
    } else {
      Message.error(result.message || '删除失败');
      return false;
    }
  } catch (err) {
    Message.error(`出错了：${err.message}`);
    console.warn(err);
    return false;
  }
};

const changeRole = (role) => {
  if (roleImage.value !== role) {
    roleImage.value = role;
    currentPage.value = 1;
    selectedImages.value = [];
    selectAll.value = false;
    loadImages();
  }
};

const changePage = (page) => {
  if (page >= 1 && page <= totalPages.value) {
    currentPage.value = page;
    selectedImages.value = [];
    selectAll.value = false;
    loadImages();
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }
};

const isImageSelected = (imageId) => {
  return selectedImages.value.includes(imageId);
};

const isTagSelected = (tagId) => {
  return selectedTags.value.includes(tagId);
};

const handleTagSelection = (tagId) => {
  if (!selectedTags.value.includes(tagId)) {
    selectedTags.value.push(tagId);
  } else {
    selectedTags.value = selectedTags.value.filter(id => id !== tagId);
  }
  loadImages();
};

const handleImageSelection = (imageId, isChecked) => {
  if (isChecked) {
    if (!selectedImages.value.includes(imageId)) {
      selectedImages.value.push(imageId);
    }
  } else {
    selectedImages.value = selectedImages.value.filter(id => id !== imageId);
  }
};

const handleAccessSourceChange = async (image, event) => {
  const previousBucketId = getSelectedAccessBucketId(image);
  const bucketId = Number(event.target.value);
  if (!bucketId || bucketId === previousBucketId) return;

  const source = getAccessSourceOptions(image).find(item => Number(item.bucket_id) === bucketId);
  if (!source || source.bucket_disabled || source.access_unavailable) {
    event.target.value = String(previousBucketId);
    Message.warning('该存储源当前不可用');
    return;
  }

  setAccessSourceUpdating(image.id, true);
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/${image.id}/access-source`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: JSON.stringify({ bucket_id: bucketId })
    });
    const result = await response.json();
    if (!response.ok || result.code !== 200) {
      throw new Error(result.message || '设置访问源失败');
    }
    image.access_bucket_id = bucketId;
    if (currentPreviewImage.value?.id === image.id) {
      currentPreviewImage.value.access_bucket_id = bucketId;
    }
    Message.success(result.message || '图片访问源已更新');
  } catch (error) {
    event.target.value = String(previousBucketId);
    console.error('设置图片访问源失败:', error);
    Message.error(error.message || '设置图片访问源失败');
  } finally {
    setAccessSourceUpdating(image.id, false);
  }
};

const getBatchAccessSourceOptions = () => {
  const selected = images.value.filter(image => selectedImages.value.includes(image.id));
  if (selected.length === 0) return [];
  const candidates = getAccessSourceOptions(selected[0]).filter(
    source => !source.bucket_disabled && !source.access_unavailable
  );
  return candidates.filter(candidate => selected.every(image =>
    getAccessSourceOptions(image).some(source =>
      Number(source.bucket_id) === Number(candidate.bucket_id) &&
      !source.bucket_disabled &&
      !source.access_unavailable
    )
  ));
};

const updateBatchAccessSource = async (bucketId) => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/access-source`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: JSON.stringify({
        image_ids: selectedImages.value,
        bucket_id: Number(bucketId),
      })
    });
    const result = await response.json();
    if (!response.ok || result.code !== 200) {
      throw new Error(result.message || '批量设置访问源失败');
    }
    images.value.forEach(image => {
      if (selectedImages.value.includes(image.id)) image.access_bucket_id = Number(bucketId);
    });
    Message.success(result.message || '批量访问源已更新');
    return true;
  } catch (error) {
    console.error('批量设置访问源失败:', error);
    Message.error(error.message || '批量设置访问源失败');
    return false;
  }
};

const handleBatchSetAccessSource = () => {
  if (selectedImages.value.length === 0) {
    Message.warning('请选择要编辑的图片');
    return;
  }
  const sources = getBatchAccessSourceOptions();
  if (sources.length === 0) {
    Message.warning('所选图片没有共同的、已同步成功的可用存储源');
    return;
  }
  const localSource = sources.find(source => source.bucket_type === 'default');
  const defaultBucketId = localSource?.bucket_id || sources[0].bucketId;
  const modal = new showFormModal({
    title: '批量设置访问源',
    formFields: [
      {
        name: 'bucket_id',
        label: '访问链接读取源',
        type: 'select',
        required: true,
        defaultValue: String(defaultBucketId),
        options: sources.map(source => ({
          value: String(source.bucket_id),
          label: getAccessSourceOptionLabel(source),
        })),
        tip: `仅显示这 ${selectedImages.value.length} 张图片都已同步成功的存储源`,
      },
    ],
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: modalInstance => modalInstance.close(),
      },
      {
        text: '确认设置',
        type: 'primary',
        callback: async modalInstance => {
          const formData = serializeForm(modalInstance);
          if (await updateBatchAccessSource(formData.bucket_id)) modalInstance.close();
        },
      },
    ],
  });
  modal.open();
};

const handleSelectAll = (e) => {
  const isChecked = e.target.checked;
  selectedImages.value = isChecked
    ? images.value.map(image => image.id)
    : [];
};

const handleBatchDelete = () => {
  if (selectedImages.value.length === 0) {
    Message.warning('请选择要删除的图片');
    return;
  }
  const userInfo = JSON.parse(localStorage.getItem('userInfo') || '{}');
  let filterIds = selectedImages.value;
  if (!isAdmin.value && userInfo.id) {
    filterIds = selectedImages.value.filter(id => {
      const image = images.value.find(item => item.id === id);
      return image && image.user_id === userInfo.id;
    });
  }
  if (filterIds.length === 0) {
    Message.warning('你没有权限删除选中的图片');
    return;
  }
  const modal = new PopupModal({
    title: '批量删除确认',
    content: `
      <div class="flex gap-3">
        <i class="fa fa-exclamation-triangle text-warning text-xl mt-1"></i>
        <div>
          <p>确定要删除选中的 ${filterIds.length} 张图片吗？</p>
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
          await batchDeleteImages(filterIds);
        }
      }
    ],
    maskClose: true
  });
  modal.open();
};

const handleBatchSetTag = () => {
  if (selectedImages.value.length === 0) {
    Message.warning('请选择要编辑的图片');
    return;
  }
  const imageId = selectedImages.value;
  const tagList = [
    { value: "0", label: "请选择Tag", disabled: true }
  ];
  presetTags.value.forEach(tag => {
    tagList.push({ value: tag.id, label: tag.name });
  });
  const modal = new showFormModal({
    title: '批量编辑Tag',
    formFields: [
      {
        name: 'tag',
        label: 'Tag标签',
        type: 'select',
        required: true,
        defaultValue: "0",
        options: tagList,
        tip: "已选择的图片：<br>" + images.value.filter(item => imageId.includes(item.id)).map(item => item.filename).join("<br>")
      },
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
        text: '删除Tag',
        type: 'danger',
        callback: (modal) => {
          const formData = serializeForm(modal);
          batchDeleteTag(formData);
          modal.close();
        }
      },
      {
        text: '添加Tag',
        type: 'primary',
        callback: (modal) => {
          const formData = serializeForm(modal);
          batchAddTag(formData);
          modal.close();
        }
      }
    ]
  });
  modal.open();
}

const batchDeleteTag = async (formData) => {
  if (formData.tag === "0") {
    Message.warning("请选择Tag");
    return;
  }
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/tags`, {
      method: 'DELETE',
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: JSON.stringify({
        image_ids: selectedImages.value,
        tag_id: formData.tag
      })
    });
    const result = await response.json();
    if (response.ok && result.code === 200) {
      Message.success('删除Tag成功');
      await loadImages();
    } else {
      throw new Error(result.message || '删除Tag失败');
    }
  } catch (error) {
    console.error('删除Tag失败:', error);
    Message.error(error.message || '删除Tag失败');
  }
}

const batchAddTag = async (formData) => {
  if (formData.tag === "0") {
    Message.warning("请选择Tag");
    return;
  }
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/tags`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      },
      body: JSON.stringify({
        image_ids: selectedImages.value,
        tag_id: formData.tag
      })
    });
    const result = await response.json();
    if (response.ok && result.code === 200) {
      Message.success('添加Tag成功');
      await loadImages();
    } else {
      throw new Error(result.message || '添加Tag失败');
    }
  } catch (error) {
    console.error('添加Tag失败:', error);
    Message.error(error.message || '添加Tag失败');
  }
}

const copyToClipboard = (text) => {
  return new Promise((resolve) => {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(() => resolve(true)).catch(() => resolve(fallbackCopy(text)));
    } else {
      resolve(fallbackCopy(text));
    }
  });
};

const fallbackCopy = (text) => {
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.style.cssText = 'position:fixed;opacity:0;left:-9999px;top:-9999px';
  document.body.appendChild(ta);
  ta.focus();
  ta.select();
  let ok = false;
  try {
    ok = document.execCommand('copy');
  } catch (e) {
    ok = false;
  }
  document.body.removeChild(ta);
  return ok;
};

const handleBatchCopy = () => {
  if (selectedImages.value.length === 0) {
    Message.warning('请选择要复制的图片');
    return;
  }
  const selectedImageList = images.value.filter(img => selectedImages.value.includes(img.id));
  if (selectedImageList.length === 0) {
    Message.warning('未找到选中的图片');
    return;
  }

  const generateText = (format) => {
    return selectedImageList.map(img => {
      const url = getFullUrl(img.url);
      switch (format) {
        case 'url': return url;
        case 'markdown': return `![${img.filename}](${url})`;
        case 'html': return `<img src="${url}" alt="${img.filename}">`;
        case 'bbcode': return `[img]${url}[/img]`;
        default: return url;
      }
    }).join('\n');
  };

  const formatLabels = {
    url: 'URL 链接',
    markdown: 'Markdown',
    html: 'HTML',
    bbcode: 'BBCode'
  };

  window._batchCopyGenerate = generateText;

  const modal = new PopupModal({
    title: `批量复制（${selectedImageList.length} 张图片）`,
    content: `
      <div class="space-y-3">
        <p class="text-sm text-secondary">选择要复制的格式：</p>
        <div class="space-y-2" id="batchCopyFormatList">
          <label class="flex items-center gap-3 p-3 rounded-lg border border-primary bg-primary/5 dark:bg-primary/10 cursor-pointer transition-colors batch-copy-option" data-format="url">
            <input type="radio" name="batchCopyFormat" value="url" checked class="h-4 w-4 text-primary shrink-0">
            <div class="min-w-0">
              <div class="text-sm font-medium">URL 链接</div>
              <div class="text-xs text-secondary mt-0.5">每行一个图片直链地址</div>
            </div>
          </label>
          <label class="flex items-center gap-3 p-3 rounded-lg border border-slate-200 dark:border-white/10 cursor-pointer hover:bg-slate-50 dark:hover:bg-white/5 transition-colors batch-copy-option" data-format="markdown">
            <input type="radio" name="batchCopyFormat" value="markdown" class="h-4 w-4 text-primary shrink-0">
            <div class="min-w-0">
              <div class="text-sm font-medium">Markdown</div>
              <div class="text-xs text-secondary mt-0.5">![filename](url) 格式，适用于 Markdown 编辑器</div>
            </div>
          </label>
          <label class="flex items-center gap-3 p-3 rounded-lg border border-slate-200 dark:border-white/10 cursor-pointer hover:bg-slate-50 dark:hover:bg-white/5 transition-colors batch-copy-option" data-format="html">
            <input type="radio" name="batchCopyFormat" value="html" class="h-4 w-4 text-primary shrink-0">
            <div class="min-w-0">
              <div class="text-sm font-medium">HTML</div>
              <div class="text-xs text-secondary mt-0.5">&lt;img src="url" alt="filename"&gt; 格式</div>
            </div>
          </label>
          <label class="flex items-center gap-3 p-3 rounded-lg border border-slate-200 dark:border-white/10 cursor-pointer hover:bg-slate-50 dark:hover:bg-white/5 transition-colors batch-copy-option" data-format="bbcode">
            <input type="radio" name="batchCopyFormat" value="bbcode" class="h-4 w-4 text-primary shrink-0">
            <div class="min-w-0">
              <div class="text-sm font-medium">BBCode</div>
              <div class="text-xs text-secondary mt-0.5">[img]url[/img] 格式，适用于论坛</div>
            </div>
          </label>
        </div>
        <div class="mt-3 p-3 rounded-lg bg-slate-50 dark:bg-white/5 border border-slate-200/50 dark:border-white/5">
          <label class="flex items-center gap-2 cursor-pointer mb-2">
            <input type="checkbox" id="batchCopyPreview" class="h-4 w-4 rounded border-gray-300 text-primary focus:ring-primary">
            <span class="text-sm font-medium">预览内容</span>
          </label>
          <pre id="batchCopyPreviewContent" class="hidden text-xs text-secondary overflow-auto max-h-40 whitespace-pre-wrap break-all bg-white dark:bg-slate-900 rounded-lg p-3 border border-slate-200 dark:border-white/10 font-mono"></pre>
        </div>
      </div>
    `,
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (m) => {
          m.close();
          delete window._batchCopyGenerate;
        }
      },
      {
        text: '复制到剪贴板',
        type: 'primary',
        callback: (m) => {
          const format = m.content?.querySelector('input[name="batchCopyFormat"]:checked')?.value || 'url';
          const genFn = window._batchCopyGenerate;
          if (typeof genFn !== 'function') {
            Message.error('复制功能异常，请重试');
            return;
          }
          const text = genFn(format);
          copyToClipboard(text).then(ok => {
            if (ok) {
              Message.success(`已复制 ${selectedImageList.length} 张图片的${formatLabels[format]}格式`);
              m.close();
              delete window._batchCopyGenerate;
            } else {
              Message.error('复制失败，请手动复制');
            }
          });
        }
      }
    ],
    maskClose: true
  });
  modal.open();

  requestAnimationFrame(() => {
    const container = modal.content;
    if (!container) return;

    container.querySelectorAll('input[name="batchCopyFormat"]').forEach(radio => {
      radio.addEventListener('change', () => {
        const selected = container.querySelector('input[name="batchCopyFormat"]:checked')?.value || 'url';
        container.querySelectorAll('.batch-copy-option').forEach(el => {
          if (el.dataset.format === selected) {
            el.classList.add('border-primary', 'bg-primary/5', 'dark:bg-primary/10');
            el.classList.remove('border-slate-200', 'dark:border-white/10');
          } else {
            el.classList.remove('border-primary', 'bg-primary/5', 'dark:bg-primary/10');
            el.classList.add('border-slate-200', 'dark:border-white/10');
          }
        });
        const checkbox = container.querySelector('#batchCopyPreview');
        const preview = container.querySelector('#batchCopyPreviewContent');
        if (checkbox?.checked && preview && typeof window._batchCopyGenerate === 'function') {
          preview.textContent = window._batchCopyGenerate(selected);
        }
      });
    });

    const checkbox = container.querySelector('#batchCopyPreview');
    const preview = container.querySelector('#batchCopyPreviewContent');
    checkbox?.addEventListener('change', () => {
      if (checkbox.checked) {
        const format = container.querySelector('input[name="batchCopyFormat"]:checked')?.value || 'url';
        if (typeof window._batchCopyGenerate === 'function') {
          preview.textContent = window._batchCopyGenerate(format);
        }
        preview.classList.remove('hidden');
      } else {
        preview.classList.add('hidden');
      }
    });
  });
};

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

const openPreview = (image) => {
  currentPreviewImage.value = image;
  const previewContent = generatePreviewContent(image);
  const customModal = new PopupModal({
    title: image.filename,
    content: previewContent,
    type: 'default',
    buttons: [
      {
        text: '确定',
        type: 'default',
        callback: (modal) => {
          modal.close();
          cleanPreviewGlobalFunctions();
        }
      }
    ],
    maskClose: true,
    zIndex: 10000,
    maxHeight: '90vh'
  });
  registerPreviewGlobalFunctions(customModal, image.id);
  customModal.open();
};

const generatePreviewContent = (image) => {
  const roleClass = image.user_id == '1'
    ? 'background-color: #e0f2fe; color: #0369a1; dark:background-color: #075985; dark:color: #bae6fd;'
    : 'background-color: #dcfce7; color: #166534; dark:background-color: #14532d; dark:color: #bbf7d0;';
  const syncSummary = getStorageSyncSummary(image);
  const tagsHtml = image.tags?.map(tag => `
    <div class="px-2 py-0.5 rounded bg-primary/10 dark:bg-primary/20 text-primary text-xs" data-tag-id="${tag.id}" data-image-id="${image.id}">
      <span>${tag.name}</span>
      <button
        onclick="window.deleteImageTag(event, ${image.id}, ${tag.id})"
        class="ml-1 text-primary/70 hover:text-primary/30">
        <i class="ri-close-line text-xs"></i>
      </button>
    </div>
  `).join('') || '';
  const headerStorageHtml = multiStorageSync.value ? `
    <span class="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs ${syncSummary.badgeClass}">
      <i class="${syncSummary.icon}"></i>${syncSummary.label}
    </span>
  ` : `
    <span class="rounded bg-success px-2 py-0.5 text-xs text-white">
      ${presetBuckets.value.find(bucket => bucket.id == image.bucket_id)?.name || '未知存储'}
    </span>
  `;
  const syncStatusHtml = multiStorageSync.value ? `
    <div class="mt-3 border-t border-slate-200/70 pt-3 dark:border-white/10">
      <div class="mb-2 flex items-center gap-2 text-xs font-semibold text-slate-700 dark:text-slate-200">
        <i class="ri-cloud-line"></i>存储同步状态
      </div>
      <div class="grid gap-2 sm:grid-cols-2">
        <div class="rounded-xl border border-emerald-200 bg-emerald-50 px-3 py-2 dark:border-emerald-500/20 dark:bg-emerald-500/10">
          <div class="flex items-center justify-between gap-2 text-xs">
            <span class="font-medium text-emerald-800 dark:text-emerald-200">本机</span>
            <span class="inline-flex items-center gap-1 text-emerald-700 dark:text-emerald-300"><i class="ri-checkbox-circle-line"></i>已保存</span>
          </div>
        </div>
        ${renderStorageStatusesHtml(image)}
      </div>
    </div>
  ` : '';
  const legacyStorageHtml = !multiStorageSync.value ? `
    <div class="flex items-center gap-1.5">
      <i class="ri-hard-drive-3-line"></i>
      存储: ${STORAGE_MAP[image.storage] || image.storage || '未知'}
    </div>
  ` : '';

  return `
    <div class="image-preview-popup w-full max-w-5xl max-h-[85vh] flex flex-col overflow-hidden bg-white dark:bg-dark-200">
      <div class="preview-header bg-light-50 pb-2 flex justify-between items-center">
        <div class="flex items-center gap-2">
          <span class="text-xs px-2 py-0.5 rounded" style="${roleClass}">
            ${image.uploader_role == '1' ? '管理员' : (image.uploader_role == '3' ? '用户' : '游客') }
          </span>
          ${headerStorageHtml}
        </div>
        <div class="flex gap-1">
          <button
            class="px-3 py-1.5 text-xs bg-light-100 dark:bg-dark-300 hover:bg-light-200 whitespace-nowrap dark:hover:bg-dark-400 text-secondary rounded-md transition-colors duration-200 flex items-center gap-1"
            onclick="event.stopPropagation(); window.downloadPreviewImage()"
          >
            <i class="ri-download-fill text-xs"></i>
            下载
          </button>
          <button
            class="px-3 py-1.5 text-xs bg-danger/10 hover:bg-danger/20 whitespace-nowrap text-danger rounded-md transition-colors duration-200 flex items-center gap-1"
            onclick="event.stopPropagation(); window.deletePreviewImage(${image.id})"
          >
            <i class="ri-delete-bin-fill text-xs"></i>
            删除
          </button>
        </div>
      </div>
      <div class="max-h-[360px] flex-1 overflow-auto flex items-center justify-center">
        <a
          class="spotlight min-w-full max-w-full min-h-[260px] block"
          href="${getFullUrl(image.url)}"
          data-description="尺寸: ${image.width || '未知'}×${image.height || '未知'} | 大小: ${formatFileSize(image.file_size || 0)} | 上传日期：${formatDate(image.created_at)} | 角色：${image.uploader_role == '1' ? '管理员' : (image.uploader_role == '3' ? '用户' : '游客')}"
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
      <div class="pt-2 flex flex-wrap gap-2 items-center">
        <p class="mr-1 text-xs text-secondary font-semibold">Tags：</p>
        ${tagsHtml}
        <button
          onclick="window.addImageTag(${image.id})"
          class="flex items-center px-2 py-1 bg-success/10 dark:bg-success/20 text-success rounded-full text-xs hover:text-success/30 transition-colors">
          <i class="ri-add-line"></i>
        </button>
      </div>
      ${syncStatusHtml}
      <div class="pt-2 flex flex-wrap gap-2 text-xs text-secondary">
        <div class="flex items-center gap-1.5">
          <i class="ri-ruler-line w-3.5 text-center"></i>
          尺寸: ${image.width || '未知'}×${image.height || '未知'}
        </div>
        <div class="flex items-center gap-1.5">
          <i class="ri-image-line w-3.5 text-center"></i>
          大小: ${formatFileSize(image.file_size || 0)}
        </div>
        ${legacyStorageHtml}
        <div class="flex items-center gap-1.5">
          <i class="ri-user-line"></i>
          角色: ${image.uploader_role == '1' ? '管理员' : (image.uploader_role == '3' ? '用户' : '游客')}
        </div>
      </div>
    </div>
  `;
};

const registerPreviewGlobalFunctions = (modal, imageId) => {
  window.copyPreviewImageLink = (type) => {
    if (!currentPreviewImage.value) return;
    const image = currentPreviewImage.value;
    const fullUrl = getFullUrl(image.url);
    let copyText = '';
    switch (type) {
      case 'url': copyText = fullUrl; break;
      case 'html': copyText = `<img src="${fullUrl}" alt="${image.filename}">`; break;
      case 'markdown': copyText = `![${image.filename}](${fullUrl})`; break;
      default: copyText = fullUrl;
    }
    copyToClipboard(copyText).then(ok => {
      if (ok) {
        Message.success('已复制到剪贴板');
      } else {
        Message.error('复制失败');
      }
    });
  };

  window.downloadPreviewImage = () => {
    if (!currentPreviewImage.value) return;
    const a = document.createElement('a');
    a.href = getFullUrl(currentPreviewImage.value.url);
    a.download = currentPreviewImage.value.filename || 'image';
    a.target = '_blank';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  window.deletePreviewImage = (id) => {
    const modal = new PopupModal({
      title: '删除确认',
      content: `
        <div class="flex gap-3">
          <i class="fa fa-exclamation-triangle text-warning text-xl mt-1"></i>
          <div>
            <p>确定要删除这张图片吗？</p>
            <p class="mt-1 text-secondary text-sm">删除后无法恢复</p>
          </div>
        </div>
      `,
      buttons: [
        {
          text: '取消',
          type: 'default',
          callback: (m) => m.close()
        },
        {
          text: '确认删除',
          type: 'danger',
          callback: async (m) => {
            m.close();
            await deleteAsync(id);
          }
        }
      ],
      maskClose: true
    });
    modal.open();
  };

  window.deleteImageTag = (event, imageId, tagId) => {
    event.stopPropagation();
    deleteImageTagAsync(imageId, tagId).then(success => {
      if (success) {
        const tagEl = event.target.closest(`[data-tag-id="${tagId}"]`);
        if (tagEl) tagEl.remove();
        const image = images.value.find(item => item.id === imageId);
        if (image) currentPreviewImage.value = image;
      }
    });
  };

  window.addImageTag = (imageId) => {
    const tagList = [{ value: "0", label: "请选择Tag", disabled: true }];
    presetTags.value.forEach(tag => {
      tagList.push({ value: tag.id, label: tag.name });
    });
    const modal = new showFormModal({
      title: '添加Tag',
      formFields: [
        {
          name: 'tag',
          label: 'Tag标签',
          type: 'select',
          required: true,
          defaultValue: "0",
          options: tagList
        },
      ],
      buttons: [
        {
          text: '取消',
          type: 'default',
          callback: (m) => m.close()
        },
        {
          text: '添加',
          type: 'primary',
          callback: (m) => {
            const formData = serializeForm(m);
            pustImageTag(imageId, formData);
            m.close();
          }
        }
      ]
    });
    modal.open();
  };
};

const cleanPreviewGlobalFunctions = () => {
  delete window.copyPreviewImageLink;
  delete window.downloadPreviewImage;
  delete window.deletePreviewImage;
  delete window.deleteImageTag;
  delete window.addImageTag;
};

onMounted(async () => {
  const userInfo = JSON.parse(localStorage.getItem('userInfo') || '{}');
  if (userInfo?.role === 1) {
    isAdmin.value = true;
  } else {
    roleImage.value = userInfo?.role == 2 ? "guest" : (userInfo?.role == 3 ? "user": "guest");
  }
  await Promise.all([getTagsList(), getBucketsList(), getStorageMode()]);
  await loadImages();
});

onUnmounted(() => {
  if (syncPollTimer) {
    clearTimeout(syncPollTimer);
    syncPollTimer = null;
  }
  cleanPreviewGlobalFunctions();
  delete window._batchCopyGenerate;
});
</script>