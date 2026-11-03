<template>
  <div class="page-shell gallery-browser text-gray-800 dark:text-gray-200" :data-gallery-scope="management ? 'all' : 'mine'">
    <!-- 游客提示属于需要保留的说明，用紧凑 note 代替原来的标题+副标题 -->
    <p v-if="!management && isGuest" class="page-note">
      游客图片归属由本浏览器的安全 Cookie 保持。清除 Cookie 或更换浏览器后无法恢复，请注册账户保存长期身份。
    </p>
    <section class="surface gallery-panel-compact space-y-3" aria-label="图库工具栏">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div v-if="!management" class="flex flex-wrap items-center gap-2">
          <button v-if="foldersAuthenticated && selectedFolder === '0'" id="gallery-new-folder" type="button" class="soft-button" :disabled="foldersLoading || !foldersLoaded || foldersAtLimit" @click="openFolderDialog('create')"><i class="ri-folder-add-line" aria-hidden="true"></i>新建文件夹</button>
          <button v-if="activeFolder?.deleting" type="button" class="gallery-primary-button" disabled>上传到此文件夹</button>
          <router-link v-else id="gallery-upload" class="gallery-primary-button" :to="{ path: '/', query: foldersAuthenticated ? { folder_id: selectedFolder || '0' } : {} }"><i class="ri-upload-2-line" aria-hidden="true"></i>{{ activeFolder ? '上传到此文件夹' : '上传' }}</router-link>
        </div>
        <span v-else class="text-sm text-slate-500 dark:text-slate-400">全部上传者 · {{ ROLE_MAP[roleImage] }}</span>
        <div class="flex items-center gap-2">
          <button id="gallery-filter-toggle" type="button" class="soft-button" aria-haspopup="dialog" @click="openFilterDialog"><i class="ri-filter-3-line" aria-hidden="true"></i>筛选<span v-if="activeFilterCount" class="text-blue-600 dark:text-blue-400">{{ activeFilterCount }}</span></button>
          <button id="gallery-refresh" type="button" class="soft-button" :disabled="loading" aria-label="刷新图库" @click="refreshGallery"><i class="ri-refresh-line" aria-hidden="true"></i><span class="hidden sm:inline">刷新</span></button>
        </div>
      </div>
      <div class="gallery-searchbar">
        <!-- 搜索框与「标签管理」保持同一套控件：input-modern + 左侧内嵌图标。
             原来是自绘的 gallery-search 边框容器，视觉上和全站输入框不一致。 -->
        <form class="gallery-search" role="search" @submit.prevent="applySearch">
          <div class="relative min-w-0 flex-1">
            <label for="gallery-search" class="sr-only">按文件名搜索</label>
            <i class="ri-search-line pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-sm text-slate-400" aria-hidden="true"></i>
            <input id="gallery-search" v-model="searchInput" type="search" class="input-modern pl-9" placeholder="按文件名搜索" autocomplete="off" @input="queueSearch" />
          </div>
          <button id="gallery-search-submit" type="submit" class="primary-button shrink-0" aria-label="搜索图片">搜索</button>
        </form>
        <div class="gallery-sort-controls">
          <label for="gallery-sort" class="sr-only">排序字段</label>
          <select id="gallery-sort" class="input-modern" :value="sortBy" @change="setQueryFilter('sort_by', $event.target.value)"><option value="created_at">上传时间</option><option value="filename">文件名称</option><option value="file_size">文件大小</option></select>
          <button id="gallery-sort-order" type="button" class="soft-button" :aria-label="sortOrder === 'desc' ? '当前降序，切换升序' : '当前升序，切换降序'" @click="setQueryFilter('sort_order', sortOrder === 'desc' ? 'asc' : 'desc')"><i :class="sortOrder === 'desc' ? 'ri-sort-desc' : 'ri-sort-asc'" aria-hidden="true"></i>{{ sortOrder === 'desc' ? '降序' : '升序' }}</button>
          <div class="gallery-view-toggle" role="group" aria-label="显示方式"><button id="gallery-view-list" type="button" :aria-pressed="viewMode === 'list'" aria-label="列表视图" title="列表视图" @click="setViewMode('list')"><i class="ri-list-check" aria-hidden="true"></i></button><button id="gallery-view-grid" type="button" :aria-pressed="viewMode === 'grid'" aria-label="网格视图" title="网格视图" @click="setViewMode('grid')"><i class="ri-grid-line" aria-hidden="true"></i></button><button id="gallery-view-masonry" type="button" :aria-pressed="viewMode === 'masonry'" aria-label="瀑布流视图" title="瀑布流视图" @click="setViewMode('masonry')"><i class="ri-layout-masonry-line" aria-hidden="true"></i></button></div>
        </div>
      </div>
    </section>
    <section class="surface gallery-panel-compact images-container" aria-label="图库文件" :aria-busy="loading">
      <div class="gallery-location">
        <nav v-if="!management" id="gallery-breadcrumb" aria-label="文件夹路径" class="flex min-w-0 items-center gap-2 text-sm"><button type="button" class="gallery-breadcrumb-root" :aria-current="selectedFolder === '0' || !foldersAuthenticated ? 'page' : undefined" @click="changeFolder('0')"><i class="ri-home-4-line" aria-hidden="true"></i>根目录</button><template v-if="foldersAuthenticated && selectedFolder !== '0'"><i class="ri-arrow-right-s-line shrink-0" aria-hidden="true"></i><span class="min-w-0 truncate" aria-current="page">{{ selectedFolder === '' ? '所有图片' : activeFolder?.name || '文件夹' }}</span></template></nav>
        <span v-else class="text-sm font-medium">所有用户图片</span><button v-if="foldersAuthenticated" id="gallery-all-images" type="button" class="text-sm text-blue-600 dark:text-blue-400" :aria-pressed="selectedFolder === ''" @click="changeFolder('')">所有图片</button>
      </div>
      <div v-if="activeFolder" class="pb-3 space-y-2"><p v-if="activeFolder.description" class="text-sm text-slate-500 dark:text-slate-400 whitespace-pre-wrap break-words">{{ activeFolder.description }}</p><div class="flex flex-wrap gap-2"><button type="button" class="soft-button" :disabled="activeFolder.deleting" @click="openFolderDialog('rename')">编辑文件夹</button><button type="button" class="soft-button text-red-600 dark:text-red-400" @click="openFolderDialog('delete')">{{ activeFolder.deleting ? '重试删除文件夹' : '删除文件夹' }}</button></div><p v-if="activeFolder.deleting" role="status" class="text-sm text-amber-700 dark:text-amber-300">文件夹删除未完成，不能上传、重命名或移入/移出图片；可重新确认名称后继续删除。</p></div>
      <p v-if="foldersAuthenticated && foldersError" role="alert" class="pb-3 text-sm text-red-600">{{ foldersError }} <button type="button" class="underline" @click="reloadFolders">重试</button></p>
      <div class="gallery-selection-bar"><label v-if="images.length > 0" for="selectAll" class="gallery-select-all"><input id="selectAll" ref="selectAllEl" v-model="selectAll" type="checkbox" class="app-checkbox" :disabled="loading" @change="handleSelectAll" /><span>全选本页</span></label><span v-if="totalImages !== null">共 {{ totalImages }} 张 · 本页 {{ images.length }} 张<span v-if="showFolderBrowser"> · {{ visibleFolders.length }} 个文件夹</span></span><span v-if="selectedImages.length">已选 {{ selectedImages.length }} 张</span></div>
      <div v-if="selectedImages.length > 0" class="batch-actions flex flex-wrap gap-2 pb-3"><button v-if="foldersAuthenticated" type="button" class="soft-button" :disabled="!canMoveSelection || !foldersLoaded || foldersLoading" @click="openFolderDialog('move')">移动图片 ({{ selectedImages.length }})</button><span v-if="foldersAuthenticated && !canMoveSelection" role="status" class="text-xs text-amber-700 dark:text-amber-300">只能移动本人且未在删除中的图片，请取消选择其他用户或删除中的项目。</span><button type="button" class="soft-button" @click="handleBatchCopy">批量复制</button><button type="button" class="soft-button" @click="handleBatchSetAccessSource">批量设置访问源</button><button type="button" class="soft-button" @click="handleBatchSetTag">批量设置Tag</button><button type="button" class="danger-button" @click="handleBatchDelete">删除 ({{ selectedImages.length }})</button></div>
      <div v-if="viewMode === 'list' && (visibleFolders.length || images.length)" class="gallery-list-heading" aria-hidden="true"><span>名称</span><span>大小 / 上传时间</span><span>操作</span></div>
      <div v-if="showFolderBrowser" class="gallery-folders" :class="viewMode === 'list' ? '' : 'gallery-grid-layout'"><GalleryFolderItem v-for="folder in visibleFolders" :key="folder.id" :folder="folder" :view-mode="viewMode === 'list' ? 'list' : 'grid'" @open="changeFolder(String($event))" @edit="openFolderDialog('rename', $event)" @delete="openFolderDialog('delete', $event)" /><p v-if="foldersLoading && !foldersLoaded" role="status" class="py-3 text-sm text-slate-500">正在加载文件夹…</p></div>
      <div v-if="loading" class="loading-container flex items-center justify-center gap-2 py-12" role="status"><i class="ri-loader-4-line animate-spin" aria-hidden="true"></i>加载图片中…</div>
      <div v-else-if="loadError" role="alert" class="py-10 text-center"><p class="mb-3 text-red-600 dark:text-red-400">{{ loadError }}</p><button type="button" class="soft-button" @click="loadImages">重试加载图片</button></div>
      <template v-else-if="hasLoadedImages">
        <!-- 三种视图各自独立的容器：列表/网格共用 GalleryImageCard，瀑布流用独立的卡片 -->
        <template v-if="images.length">
          <GalleryMasonry v-if="viewMode === 'masonry'" :items="visibleImages" :cols="[2, 3, 4, 5]">
            <template #default="{ item }">
              <GalleryMasonryCard v-bind="{ image: item, isImageSelected, handleImageSelection, openPreview, handleImageLoad, handleImageError, formatFileSize, openDetails: openImageDetails }" />
            </template>
          </GalleryMasonry>
          <div v-else :class="viewMode === 'grid' ? 'images-grid gallery-grid-layout' : 'images-list'"><GalleryImageCard v-for="image in visibleImages" :key="image.id" v-bind="{ image, viewMode, management, isImageSelected, getRoleTagClass, handleImageSelection, openPreview, handleImageLoad, handleImageError, formatFileSize, formatDate, openDetails: openImageDetails }" /></div>
        </template>
        <div v-else class="empty-state py-10 text-center"><i class="ri-image-line text-3xl text-slate-400" aria-hidden="true"></i><h2 class="mt-2 font-medium">{{ activeFilterCount || searchTerm ? '没有匹配的图片' : showFolderBrowser && visibleFolders.length ? '根目录暂无未分类图片' : '暂无图片' }}</h2><p class="mt-1 text-sm text-slate-500 dark:text-slate-400">{{ activeFilterCount || searchTerm ? '可以清除搜索与筛选，重新查看图片。' : management ? '全站暂未上传图片。' : '可以上传图片，或打开文件夹查看。' }}</p><button v-if="activeFilterCount || searchTerm" type="button" class="soft-button mt-3" @click="clearFilters">清除搜索与筛选</button><button v-else type="button" class="soft-button mt-3" @click="refreshGallery">重新加载</button></div>
        <div v-if="hasHiddenImages" ref="sentinel" class="flex justify-center mt-4"><button class="soft-button" type="button" @click="revealImages">显示本页更多图片</button></div>
        <nav v-if="totalPages > 1" class="pagination flex flex-wrap items-center justify-center gap-2 py-4" aria-label="图片分页"><button type="button" class="soft-button" :disabled="currentPage <= 1" @click="changePage(currentPage - 1)">上一页</button><button v-for="page in visiblePages" :key="page" type="button" class="gallery-page-button" :aria-current="page === currentPage ? 'page' : undefined" @click="changePage(page)">{{ page }}</button><button type="button" class="soft-button" :disabled="currentPage >= totalPages" @click="changePage(currentPage + 1)">下一页</button></nav>
      </template>
    </section>
    <FolderDialog v-if="folderDialog && foldersAuthenticated" :mode="folderDialog.mode" :folder="folderDialog.folder" :folders="folders" :image-ids="folderDialog.imageIds" :move-allowed="canMoveSelection" :initial-destination="selectedFolder || '0'" :account-key="folderAccountKey" @close="folderDialog = null" @saved="onFolderSaved" @partial-failure="refreshAfterFolderFailure" />
    <!-- key 绑定在打开时递增，保证每次打开都是全新的草稿，不会残留上次未应用的改动 -->
    <GalleryFilterDialog
      v-if="filterDialogOpen"
      :key="filterDialogKey"
      :roles="management ? roleOptions : []"
      :role="roleImage"
      :buckets="presetBuckets"
      :bucket="selectedBucket"
      :deletion="deletionFilter"
      :tags="presetTags"
      :selected-tags="selectedTags"
      :folders-authenticated="foldersAuthenticated"
      :folders="folders"
      :folders-loaded="foldersLoaded"
      :folder="selectedFolder"
      :unfiled-count="unfiledCount"
      :folder-total-images="folderTotalImages"
      :folder-label="folderLabel"
      @apply="applyFilterDialog"
      @clear="clearFilters"
      @close="filterDialogOpen = false"
    />
    <!-- 图片详情：取代卡片里展开的下拉抽屉，详情 + 读取源 + 存储状态 + 删除重试都在这里 -->
    <GalleryImageDetailDialog
      v-if="detailImage"
      :key="`detail-${detailImage.id}`"
      :image="detailImage"
      :management="management"
      :multi-storage-sync="multiStorageSync"
      :preset-buckets="presetBuckets"
      :format-file-size="formatFileSize"
      :format-date="formatDate"
      :get-storage-display-name="getStorageDisplayName"
      :get-storage-statuses="getStorageStatuses"
      :get-storage-status-meta="getStorageStatusMeta"
      :get-selected-access-bucket-id="getSelectedAccessBucketId"
      :get-access-source-options="getAccessSourceOptions"
      :get-access-source-option-label="getAccessSourceOptionLabel"
      :is-access-source-updating="isAccessSourceUpdating"
      :apply-access-source="applyImageAccessSource"
      :retry-delete="retryDeleteForDetail"
      :open-preview="openPreview"
      @close="detailImage = null"
    />
  </div>
</template>

<script setup>

import Message from '@/utils/message.js';
import Loading from '@/utils/loading.js';
import { createDialogScope } from '@/utils/dialogScope.js';
const { Dialog: PopupModal, dispose: disposeViewDialogs } = createDialogScope();
import GalleryImageCard from '@/components/gallery/GalleryImageCard.vue';
import GalleryFolderItem from '@/components/gallery/GalleryFolderItem.vue';
import { galleryFolderFromQuery, galleryQuery, galleryImageParams, galleryViewPreference, GALLERY_VIEW_MODES, createGalleryRequests, parseGalleryPage } from '@/utils/galleryBrowser.js';
import { createGalleryAccessActions } from '@/utils/galleryAccessActions.js';
const showFormModal = function(options) { return new PopupModal({ type: 'form', ...options }); };

import { settleDeletions } from '@/utils/batchResults.js';
import { createGalleryBatchActions } from '@/utils/galleryBatchActions.js';
import { createGalleryPreviewActions } from '@/utils/galleryPreviewActions.js';
import { getSelectedAccessBucketId, getAccessSourceOptions as accessSourceOptions, getAccessSourceOptionLabel } from '@/utils/imageAccessSources.js';
import { boundedPage, galleryPageNumbers, GALLERY_PAGE_SIZE } from '@/utils/renderBounds.js';
import { useChunkedImages } from '@/composables/useChunkedImages.js';
import { readApiResponse } from '@/utils/apiFeedback.js';
import { ref, onMounted, computed, onUnmounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import FolderDialog from '@/components/FolderDialog.vue';
import GalleryFilterDialog from '@/components/gallery/GalleryFilterDialog.vue';
import GalleryImageDetailDialog from '@/components/gallery/GalleryImageDetailDialog.vue';
import GalleryMasonry from '@/components/gallery/GalleryMasonry.vue';
import GalleryMasonryCard from '@/components/gallery/GalleryMasonryCard.vue';
import { useFolders } from '@/composables/useFolders.js';
import { folderFilter, ownImageIds, folderAccount, folderLabel } from '@/utils/folders.js';
import errorImg from '@/assets/images/error.webp';

import { getStorageDisplayName, getStorageStatuses, getStorageStatusMeta, getStorageSyncSummary, hasActiveStorageSync } from '@/utils/storageStatus.js'

const props = defineProps({ management: { type: Boolean, default: false } });
const management = props.management; // GalleryRoute keys this owner on scope changes.
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '';
const PAGE_SIZE = GALLERY_PAGE_SIZE;
const isGuest = computed(() => Number(JSON.parse(localStorage.getItem('userInfo') || '{}').role) === 2);
const ROLE_MAP = {
  all: '全部',
  admin: '管理员',
  guest: '游客',
  user: '用户'
};
// 弹窗只接收 [{value,label}]，角色顺序沿用 ROLE_MAP 的声明顺序，避免两处各写一份
const roleOptions = Object.entries(ROLE_MAP).map(([value, label]) => ({ value, label }));
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

// 加载/错误处理只动「自身状态 + 同容器内的占位」，不绑定具体视图的类名：
// 列表/网格用 opacity-0 + .loading，瀑布流用 opacity/过渡，三种视图共用这一对函数。
const finishImagePlaceholder = target => {
  target.classList.remove('opacity-0');
  target.classList.add('is-loaded');
  const placeholder = target.parentElement?.querySelector('.loading, .masonry-loading');
  if (placeholder) placeholder.classList.add('hidden');
};

const handleImageLoad = (e) => finishImagePlaceholder(e.target);

const handleImageError = (e) => {
  if (e.target.dataset.fallbackApplied) return;
  e.target.dataset.fallbackApplied = 'true';
  e.target.src = errorImg;
  finishImagePlaceholder(e.target);
};


const images = ref([]);
const { sentinel, visibleImages, hasHiddenImages, revealImages } = useChunkedImages(images);
const loading = ref(true);
const hasLoadedImages = ref(false);
const totalImages = ref(null);
const viewMode = ref(galleryViewPreference(localStorage, management));
// 筛选项收进弹窗：不再内联展开，也不再边改边刷新
const filterDialogOpen = ref(false);
const filterDialogKey = ref(0);
const openFilterDialog = () => {
  // 每次递增 key：即使快速连开、或上次是「取消」，也总是从当前路由重建草稿
  filterDialogKey.value += 1;
  filterDialogOpen.value = true;
};
// 图片详情弹窗取代了卡片里展开的抽屉；存图片对象引用，切换读取源后能立刻回显
const detailImage = ref(null);
// 删除未完成的图片也要能打开：详情里正是「重试删除」的入口，不能拦
const openImageDetails = image => { if (image) detailImage.value = image; };
// 删除重试成功后关闭弹窗：此时该图片已从列表移除，留着详情没有意义
const retryDeleteForDetail = async imageId => {
  const ok = await deleteAsync(imageId);
  if (ok) detailImage.value = null;
  return ok;
};
const loadError = ref('');
const searchInput = ref(''), searchTerm = ref('');
const sortBy = ref('created_at'), sortOrder = ref('desc');
let searchTimer;
const imageRequests = createGalleryRequests();
const currentPage = ref(1);
const totalPages = ref(1);
const roleImage = ref("all");
const isAdmin = ref(false);
const presetTags = ref([]);
const presetBuckets = ref([]);
const selectedBucket = ref('');
const deletionFilter = ref("all");
const selectedImages = ref([]);
const selectedTags = ref([]);
const selectAll = ref(false);
// 部分选中：原生 indeterminate 是 DOM 属性，声明式绑定不了，必须拿到元素手动置位。
// 没有它时，「选了 2/8 张」和「一张没选」的全选框长得完全一样，用户看不出部分选中了。
const selectAllEl = ref(null);
const syncSelectAllIndeterminate = () => {
  const el = selectAllEl.value;
  if (!el) return;
  const total = images.value.length;
  const picked = selectedImages.value.length;
  el.indeterminate = !selectAll.value && picked > 0 && picked < total;
};
// immediate + 三个依赖：勾选单张、全选、翻页后都会重算
watch([selectAll, () => selectedImages.value.length, () => images.value.length], syncSelectAllIndeterminate, { flush: 'post', immediate: true });
const currentPreviewImage = ref(null);
const multiStorageSync = ref(false);
const accessSourceUpdatingIds = ref([]);
let syncPollTimer = null;
let disposed = false;

const router = useRouter();
const route = useRoute();
// Management never loads or exposes personal folders. Scope remount keeps this static.
const folderState = management ? {
  account: ref(''), authenticated: ref(false), folders: ref([]), unfiledCount: ref(0),
  loading: ref(false), loaded: ref(false), error: ref(''), atLimit: ref(false),
  totalImages: ref(0), reload: async () => {},
} : useFolders(API_BASE_URL);
const { account: folderAccountKey, authenticated: foldersAuthenticated, folders, unfiledCount,
  loading: foldersLoading, loaded: foldersLoaded, error: foldersError, atLimit: foldersAtLimit,
  totalImages: folderTotalImages, reload: reloadFolders } = folderState;
const selectedFolder = ref(galleryFolderFromQuery(route.query, foldersAuthenticated.value));
const folderDialog = ref(null);
const activeFolder = computed(() => foldersAuthenticated.value ? folders.value.find(folder => String(folder.id) === selectedFolder.value) : undefined);
const showFolderBrowser = computed(() => foldersAuthenticated.value && selectedFolder.value === '0');
const visibleFolders = computed(() => {
  if (!showFolderBrowser.value) return [];
  const term = searchTerm.value.trim().toLocaleLowerCase();
  const matches = folders.value.filter(folder => !term || folder.name.toLocaleLowerCase().includes(term));
  return matches.sort((a, b) => {
    const comparison = sortBy.value === 'created_at' ? String(a.created_at || '').localeCompare(String(b.created_at || '')) : a.name.localeCompare(b.name, 'zh-CN', { numeric: true });
    return (comparison || a.name.localeCompare(b.name, 'zh-CN', { numeric: true })) * (sortOrder.value === 'asc' ? 1 : -1);
  });
});
const activeFilterCount = computed(() => Number(!!selectedBucket.value) + Number(deletionFilter.value !== 'all') + selectedTags.value.length + Number(management && roleImage.value !== 'all'));
const movableImageIds = computed(() => ownImageIds(images.value, selectedImages.value, folderAccountKey.value, folders.value));
const canMoveSelection = computed(() => foldersAuthenticated.value && !activeFolder.value?.deleting && selectedImages.value.length > 0 && movableImageIds.value.length === selectedImages.value.length);
let pendingQuery = null, queryRevision = 0;
function syncQueryState(query = route.query) {
  const state = galleryQuery(query, management);
  selectedFolder.value = galleryFolderFromQuery(query, foldersAuthenticated.value);
  searchInput.value = searchTerm.value = state.filename;
  sortBy.value = state.sortBy; sortOrder.value = state.sortOrder;
  roleImage.value = state.role; selectedBucket.value = state.bucket;
  deletionFilter.value = state.deletion; selectedTags.value = state.tags;
}
syncQueryState();
function resetListing() {
  imageRequests.cancel();
  if (syncPollTimer) clearTimeout(syncPollTimer);
  syncPollTimer = null;
  currentPage.value = 1; totalPages.value = 1; selectedImages.value = []; images.value = [];
  totalImages.value = null; hasLoadedImages.value = false; loading.value = true;
  selectAll.value = false; folderDialog.value = null;
  // 筛选弹窗也要随图库重置关闭：否则切作用域后它还开着，里面的选项来自上一个账号
  filterDialogOpen.value = false;
  detailImage.value = null;
  currentPreviewImage.value = null;
  cleanupPreview(); disposeAccessDialogs(); disposeBatchDialogs(); disposeViewDialogs();
}
function navigateQuery(query) {
  const revision = ++queryRevision;
  pendingQuery = query;
  // Route guards are asynchronous: merge rapid controls against the latest intent,
  // not the previous URL, and fence old responses immediately.
  imageRequests.cancel(); syncQueryState(query); loading.value = true;
  Promise.resolve(router.replace({ query })).finally(() => {
    if (revision !== queryRevision || disposed) return;
    pendingQuery = null; syncQueryState(); loadImages();
  });
}
function setQueryFilter(key, value) {
  const query = { ...(pendingQuery || route.query) };
  if (value === '' && key !== 'folder_id') delete query[key];
  else query[key] = value;
  navigateQuery(query);
}
function changeFolder(value) {
  if (!foldersAuthenticated.value) return;
  // An explicit empty folder_id is All Images; an absent query is always root.
  setQueryFilter('folder_id', value === '' ? '' : folderFilter(value, '0'));
}
// 弹窗点「应用」：一次性把所有筛选写进路由，只触发一次加载
function applyFilterDialog({ role, bucket, deletion, tags, folder }) {
  const query = { ...(pendingQuery || route.query) };
  if (management) {
    if (role === 'all') delete query.role; else query.role = role;
  }
  if (bucket === '') delete query.bucket; else query.bucket = String(bucket);
  if (deletion === 'all') delete query.deletion; else query.deletion = deletion;
  if (tags.length) query.tags = tags.join(','); else delete query.tags;
  if (foldersAuthenticated.value) {
    if (folder === '') query.folder_id = '';
    else query.folder_id = folderFilter(folder, '0');
  }
  navigateQuery(query);
}
function applySearch() {
  clearTimeout(searchTimer);
  setQueryFilter('search', searchInput.value.trim());
}
function queueSearch() { clearTimeout(searchTimer); searchTimer = setTimeout(applySearch, 300); }
function clearFilters() {
  clearTimeout(searchTimer);
  const query = { ...(pendingQuery || route.query) };
  for (const key of ['role', 'bucket', 'deletion', 'tags', 'search']) delete query[key];
  navigateQuery(query);
}
function setViewMode(mode) {
  // 只接受已知值，避免手写 localStorage 或旧链接把界面切成空白
  viewMode.value = GALLERY_VIEW_MODES.includes(mode) ? mode : 'list';
  try { localStorage.setItem(`oneimg-gallery-view-${management ? 'all' : 'mine'}`, viewMode.value); } catch {}
}
async function refreshGallery() { await Promise.all([loadImages(), reloadFolders()]); }
watch(() => route.query, () => {
  clearTimeout(searchTimer); resetListing(); syncQueryState(pendingQuery || route.query);
  if (!pendingQuery) loadImages();
});
watch(folderAccountKey, () => {
  resetListing(); syncQueryState(); changeFolder('0'); loadImages();
});
function openFolderDialog(mode, folder = activeFolder.value) {
  if (!foldersAuthenticated.value || folderAccount() !== folderAccountKey.value) return;
  if (mode === 'create' && (selectedFolder.value !== '0' || !foldersLoaded.value || foldersAtLimit.value)) return;
  if (mode === 'move' && !canMoveSelection.value) return;
  if (mode === 'rename' && folder?.deleting) return;
  if (['rename', 'delete'].includes(mode) && !folder) return;
  folderDialog.value = { mode, folder: mode === 'create' ? {} : { ...(folder || {}) }, imageIds: [...movableImageIds.value] };
}
async function refreshAfterFolderFailure() {
  selectedImages.value = [];
  await Promise.all([reloadFolders(), loadImages()]);
}
async function onFolderSaved({ mode, deletedImages }) {
  folderDialog.value = null;
  selectedImages.value = []; currentPage.value = 1;
  if (mode === 'delete') {
    selectedFolder.value = '0';
    await router.replace({ query: { ...route.query, folder_id: '0' } });
  }
  await Promise.all([reloadFolders(), loadImages()]);
  if (!disposed) Message.success(({ create: '文件夹已创建', rename: '文件夹信息已更新', delete: deletedImages ? '文件夹及图片已删除' : '文件夹已删除，图片已返回未分类', move: '图片已移动' })[mode]);
}

const visiblePages = computed(() => galleryPageNumbers(currentPage.value, totalPages.value));
const getAccessSourceOptions = image => accessSourceOptions(image, presetBuckets.value);

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
        'X-Requested-With': 'XMLHttpRequest'
      }
    });
    const result = await readApiResponse(response, '请求失败');
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
        'X-Requested-With': 'XMLHttpRequest'
      }
    });
    const result = await readApiResponse(response, '请求失败');
    if (response.ok && result.code === 200) {
      presetBuckets.value = Array.isArray(result.data) ? result.data : [];
    } else {
      throw new Error(result.message || '获取存储列表失败');
    }
  } catch (error) {
    console.error('获取存储列表失败:', error);
    Message.error(error.message || '获取存储列表失败');
  }
};

const loadImages = async () => {
  if (disposed) return;
  const request = imageRequests.start();
  loading.value = true; loadError.value = '';
  try {
    const params = galleryImageParams({
      management, authenticated: foldersAuthenticated.value, folder: selectedFolder.value,
      page: currentPage.value, limit: PAGE_SIZE,
      sortBy: sortBy.value, sortOrder: sortOrder.value, filename: searchTerm.value,
      role: management ? roleImage.value : '', tags: selectedTags.value,
      bucket: selectedBucket.value, deletion: deletionFilter.value,
    });
    const endpoint = management ? '/api/admin/images' : '/api/images';
    const response = await fetch(`${API_BASE_URL}${endpoint}?${params}`, {
      signal: request.signal, credentials: 'same-origin', cache: 'no-store',
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
    });
    if (disposed || !request.isCurrent()) return;
    if (response.status === 401) {
      localStorage.removeItem('authToken');
      router.push('/login');
      Message.error('登录已过期，请重新登录');
    }
    const result = await readApiResponse(response, '加载图片失败');
    if (disposed || !request.isCurrent()) return;
    const page = parseGalleryPage(result.data);
    images.value = boundedPage(page.images, PAGE_SIZE);
    totalPages.value = page.totalPages;
    totalImages.value = page.total;
    hasLoadedImages.value = true;
    // Preserve failed delete selections after refreshing successful removals.
    selectedImages.value = selectedImages.value.filter(id => images.value.some(image => image.id === id));
    selectAll.value = images.value.length > 0 && images.value.every(image => selectedImages.value.includes(image.id));
    scheduleSyncRefresh();
  } catch (error) {
    if (disposed || !request.isCurrent() || error.name === 'AbortError') return;
    images.value = []; selectedImages.value = []; totalPages.value = 1;
    totalImages.value = null; hasLoadedImages.value = false;
    loadError.value = error.message || '加载图片失败，请重试';
    Message.error(`加载图片失败: ${loadError.value}`);
  } finally {
    if (request.isCurrent()) loading.value = false;
  }
};

const getStorageMode = async () => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/uploadConfig`, {
      headers: {
        'X-Requested-With': 'XMLHttpRequest'
      }
    });
    const result = await readApiResponse(response, '请求失败');
    if (disposed) return;
    multiStorageSync.value = response.ok && result.code === 200 && result.data?.multi_storage_sync === true;
    scheduleSyncRefresh();
  } catch (error) {
    console.error('获取多存储模式失败:', error);
    multiStorageSync.value = false;
  }
};

const scheduleSyncRefresh = () => {
  if (disposed || !multiStorageSync.value) {
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
  const { succeeded, failed } = await settleDeletions(deleteIds, id => deleteAsync(id, { batch: true }));
  selectedImages.value = [...new Set([...selectedImages.value.filter(id => !succeeded.includes(id)), ...failed])];
  if (failed.length) { deletionFilter.value = 'deleting'; currentPage.value = 1; }
  await Promise.all([loadImages(), reloadFolders()]);
  if (failed.length) Message.warning(`成功删除 ${succeeded.length} 张，失败 ${failed.length} 张。失败项已保留选中；部分存储源可能已删除，请重试删除。`, { duration: 7000, showClose: true });
  else Message.success(`成功删除 ${succeeded.length} 张图片`);
};

const deleteAsync = async (id, { batch = false } = {}) => {
  const loadingInstance = Loading.show({
    text: '删除中...',
    color: '#ff4d4f',
    mask: true
  });
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/${id}`, {
      method: 'DELETE',
      headers: {
        'X-Requested-With': 'XMLHttpRequest'
      }
    });
    const result = await readApiResponse(response, '请求失败');
    if (response.ok && result.code === 200) {
      if (!batch) Message.success('图片删除成功');
      selectedImages.value = selectedImages.value.filter(imageId => imageId !== id);
      if (!batch) await Promise.all([loadImages(), reloadFolders()]);
      return true;
    } else {
      throw new Error(result.message || '删除失败');
    }
  } catch (error) {
    console.error('删除图片错误:', error);
    if (!batch) {
      Message.error(`删除图片失败: ${error.message}；可从“删除未完成”继续删除`, { duration: 6000, showClose: true });
      if ([500,502].includes(error.status)) { deletionFilter.value = 'deleting'; currentPage.value = 1; await loadImages(); }
    }
    return false;
  } finally {
    await loadingInstance.hide();
  }
};

const changePage = (page) => {
  if (page >= 1 && page <= totalPages.value && !loading.value) {
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

const handleImageSelection = (imageId, isChecked) => {
  if (isChecked) {
    if (!selectedImages.value.includes(imageId)) {
      selectedImages.value.push(imageId);
    }
  } else {
    selectedImages.value = selectedImages.value.filter(id => id !== imageId);
  }
};

const { disposeDialogs: disposeAccessDialogs, applyImageAccessSource, handleBatchSetAccessSource } = createGalleryAccessActions({
  API_BASE_URL, images, selectedImages, currentPreviewImage, getAccessSourceOptions, setAccessSourceUpdating, serializeForm,
});

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
  if (!isAdmin.value && Number(userInfo.role) !== 2 && userInfo.id) {
    filterIds = selectedImages.value.filter(id => {
      const image = images.value.find(item => item.id === id);
      return image && String(image.user_id) === String(userInfo.id);
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

const { disposeDialogs: disposeBatchDialogs, handleBatchSetTag, handleBatchCopy } = createGalleryBatchActions({
  selectedImages, images, presetTags, loadImages, getFullUrl, API_BASE_URL, showFormModal, serializeForm,
});

const { openPreview, cleanupPreview } = createGalleryPreviewActions({ API_BASE_URL, images, presetTags, presetBuckets,
  currentPreviewImage, multiStorageSync, errorImg, formatFileSize, formatDate, getFullUrl, deleteAsync, serializeForm });

onMounted(async () => {
  const userInfo = JSON.parse(localStorage.getItem('userInfo') || '{}');
  if (Number(userInfo?.role) === 1) {
    isAdmin.value = true;
  }
  // Listing is independent of upload/storage/tag metadata. A slow auxiliary
  // request must never hold the gallery empty, especially on mobile networks.
  void loadImages();
  void Promise.allSettled([getTagsList(), getBucketsList(), getStorageMode()]);
});

onUnmounted(() => {
  disposed = true;
  imageRequests.dispose();
  clearTimeout(searchTimer);
  if (syncPollTimer) {
    clearTimeout(syncPollTimer);
    syncPollTimer = null;
  }
  cleanupPreview();
  disposeAccessDialogs();
  disposeBatchDialogs();
  disposeViewDialogs();
});
</script>

<style scoped>
.gallery-browser { min-width:0; }
.gallery-primary-button { display:inline-flex; align-items:center; justify-content:center; gap:.5rem; padding:.55rem .85rem; border-radius:.55rem; font-size:.875rem; font-weight:500; color:white; background:#2563eb; }
.gallery-primary-button:hover { background:#1d4ed8; }
.gallery-primary-button:disabled { opacity:.45; cursor:not-allowed; }
.gallery-searchbar { display:flex; flex-wrap:wrap; gap:.65rem; }
/* 搜索框不再是自绘的带边框容器：内层用 .input-modern，这里只负责横向排布。
   边框/圆角/聚焦环全部交给 .input-modern，与「标签管理」等页面完全一致。 */
.gallery-search { display:flex; align-items:center; gap:.5rem; min-width:0; flex:1 1 18rem; }
/* 关掉 WebKit 给 type=search 的原生放大镜与清除按钮：
   左侧已有自定义搜索图标，两者会叠在一起；清除按钮也与右侧「搜索」按钮重复。
   用 :global 是因为 scoped 会给伪元素选择器加属性选择器，::-webkit-* 匹配不到。 */
:global(#gallery-search::-webkit-search-decoration),
:global(#gallery-search::-webkit-search-cancel-button),
:global(#gallery-search::-webkit-search-results-button),
:global(#gallery-search::-webkit-search-results-decoration) { -webkit-appearance:none; appearance:none; }
.gallery-sort-controls { display:flex; align-items:center; gap:.5rem; min-width:0; }
.gallery-sort-controls select { width:auto; min-width:7rem; padding:.55rem .65rem; }
.gallery-view-toggle { display:flex; padding:.15rem; border:1px solid #e2e8f0; border-radius:.5rem; }
.gallery-view-toggle button { width:2.1rem; height:2.1rem; border-radius:.3rem; color:#64748b; }
.gallery-view-toggle button[aria-pressed="true"] { background:rgb(59 130 246 / .1); color:#2563eb; }
.gallery-location { display:flex; justify-content:space-between; align-items:center; gap:1rem; border-bottom:1px solid rgb(148 163 184 / .2); padding:.3rem 0 .85rem; margin-bottom:.85rem; }
.gallery-location > button { flex-shrink:0; }
.gallery-breadcrumb-root { display:flex; align-items:center; gap:.4rem; color:#2563eb; white-space:nowrap; }
.gallery-selection-bar { display:flex; flex-wrap:wrap; align-items:center; gap:.85rem; font-size:.75rem; color:#64748b; padding:0 0 .85rem; }
.gallery-list-heading { display:flex; gap:.75rem; padding:.65rem .75rem; border-radius:.4rem; background:rgb(148 163 184 / .08); font-size:.75rem; color:#64748b; }
.gallery-list-heading > :first-child { flex:1; }
.gallery-list-heading > :nth-child(2) { display:none; width:11rem; }
.gallery-list-heading > :last-child { width:2.25rem; }
.gallery-grid-layout { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:.65rem; }
.gallery-folders.gallery-grid-layout { padding-bottom:.65rem; }
.gallery-page-button { display:grid; place-items:center; width:2.25rem; height:2.25rem; border:1px solid rgb(148 163 184 / .25); border-radius:.4rem; font-size:.875rem; }
.gallery-page-button[aria-current="page"] { color:white; background:#2563eb; border-color:#2563eb; }
button:focus-visible,a:focus-visible { outline:none; box-shadow:0 0 0 2px rgb(37 99 235 / .45); }
:global(.dark) .gallery-view-toggle { border-color:rgb(255 255 255 / .1); }
:global(.dark) .gallery-selection-bar,:global(.dark) .gallery-list-heading { color:#94a3b8; }
:global(.dark) .gallery-breadcrumb-root,:global(.dark) .gallery-view-toggle button[aria-pressed="true"] { color:#60a5fa; }
@media (min-width:640px) { .gallery-grid-layout { grid-template-columns:repeat(3,minmax(0,1fr)); } }
@media (min-width:768px) { .gallery-list-heading > :nth-child(2) { display:block; } }
@media (min-width:1280px) { .gallery-grid-layout { grid-template-columns:repeat(4,minmax(0,1fr)); } }
@media (min-width:1536px) { .gallery-grid-layout { grid-template-columns:repeat(5,minmax(0,1fr)); } }
@media (max-width:480px) { .gallery-sort-controls { width:100%; } .gallery-sort-controls select { flex:1; } .gallery-location { gap:.5rem; } }
</style>
