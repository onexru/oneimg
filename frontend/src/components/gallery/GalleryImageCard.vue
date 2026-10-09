<template>
  <article class="gallery-image-card gallery-image-card-compact" :class="[`gallery-card-${viewMode}`, { 'gallery-card-selected': isImageSelected(image.id) }]" :data-image-id="image.id">
    <div class="gallery-file-main">
      <label class="gallery-card-checkbox" :for="`image-${image.id}`" @click.stop>
        <input type="checkbox" :id="`image-${image.id}`" :aria-label="`选择图片 ${image.filename}`" class="h-4 w-4 rounded border-gray-300 bg-white text-primary focus:ring-primary dark:bg-gray-800" :checked="isImageSelected(image.id)" @change="(e) => handleImageSelection(image.id, e.target.checked)" @click.stop />
      </label>
      <button type="button" class="image-wrapper" :disabled="image.deleting" :aria-label="`预览图片 ${image.filename}`" @click="openPreview(image)">
        <span v-if="!image.deleting" class="loading absolute inset-0 flex items-center justify-center text-slate-400"><i class="ri-loader-4-line animate-spin" aria-hidden="true"></i></span>
        <i v-else class="ri-image-off-line text-slate-400" aria-hidden="true"></i>
        <img v-if="!image.deleting" :src="image.thumbnail || image.url" :alt="image.filename" class="image-thumbnail h-full w-full object-cover opacity-0" loading="lazy" @load="handleImageLoad" @error="handleImageError" />
      </button>
      <div class="image-info">
        <button type="button" class="image-filename" :title="image.filename" :disabled="image.deleting" @click="openPreview(image)">{{ image.filename }}</button>
        <p class="gallery-file-secondary">{{ formatFileSize(image.file_size) }} <span aria-hidden="true">·</span> {{ image.width }}×{{ image.height }}<span v-if="image.deleting" class="text-amber-700 dark:text-amber-300"> · 删除未完成</span></p>
        <p v-if="management" class="gallery-uploader" :title="galleryUploader(image)">上传者：{{ galleryUploader(image) }} <span class="image-role" :class="getRoleTagClass(image.uploader_role)">{{ image.uploader_role == 1 ? '管理员' : image.uploader_role == 3 ? '用户' : '游客' }}</span></p>
        <p class="gallery-file-date">{{ formatDate(image.created_at) }}</p>
      </div>
      <div class="gallery-desktop-meta" aria-hidden="true"><span>{{ formatFileSize(image.file_size) }}</span><span>{{ formatDate(image.created_at) }}</span></div>
      <button type="button" class="gallery-details-toggle" :aria-label="`${detailsOpen ? '收起' : '展开'}图片详情 ${image.filename}`" :aria-expanded="detailsOpen" :aria-controls="`image-details-${image.id}`" @click.stop="detailsOpen = !detailsOpen"><i :class="detailsOpen ? 'ri-arrow-up-s-line' : 'ri-more-2-fill'" aria-hidden="true"></i></button>
    </div>
    <div v-show="detailsOpen" :id="`image-details-${image.id}`" class="gallery-file-details">
      <div v-if="image.deleting" class="mb-3 text-xs text-amber-700 dark:text-amber-200">
        删除尚未完成，部分存储副本可能已删除。
        <p v-for="replica in image.storage_statuses || []" :key="replica.bucket_id" class="break-words mt-1">{{ replica.bucket_name }}：{{ replica.error || '等待继续删除' }}</p>
        <button type="button" class="danger-button mt-2" @click.stop="retryDelete(image.id)">重试删除</button>
      </div>
      <div class="gallery-card-badges mb-2">
        <span v-if="multiStorageSync" class="gallery-card-badge inline-flex items-center gap-1 border" :class="getStorageSyncSummary(image).badgeClass"><i :class="getStorageSyncSummary(image).icon" aria-hidden="true"></i><span class="min-w-0 truncate" :title="getStorageSyncSummary(image).label">{{ getStorageSyncSummary(image).label }}</span></span>
        <span v-else class="gallery-card-badge gallery-card-badge-dark">{{ presetBuckets.find(bucket => bucket.id == image.bucket_id)?.name || '本地存储' }}</span>
      </div>
      <div @click.stop>
        <label :for="`access-source-${image.id}`" class="mb-1 flex items-center gap-1 text-xs font-medium text-slate-600 dark:text-slate-300"><i class="ri-route-line" aria-hidden="true"></i>访问链接读取源</label>
        <select :id="`access-source-${image.id}`" class="w-full rounded-lg border border-slate-200 bg-white px-2 py-2 text-xs text-slate-700 outline-none transition focus:border-primary focus:ring-1 focus:ring-primary/20 disabled:cursor-wait disabled:opacity-60 dark:border-white/10 dark:bg-slate-950 dark:text-slate-200" :value="getSelectedAccessBucketId(image)" :disabled="image.deleting || isAccessSourceUpdating(image.id) || getAccessSourceOptions(image).length === 0" @change="handleAccessSourceChange(image, $event)" @click.stop>
          <option v-for="source in getAccessSourceOptions(image)" :key="`${image.id}-access-${source.bucket_id}`" :value="source.bucket_id" :disabled="source.bucket_disabled || source.access_unavailable">{{ getAccessSourceOptionLabel(source) }}</option>
        </select>
      </div>
      <div v-if="multiStorageSync" class="mt-2 space-y-1.5">
        <div class="flex min-w-0 items-center justify-between gap-2 rounded-lg border border-slate-200/80 bg-slate-50 px-2 py-1.5 dark:border-white/10 dark:bg-slate-950"><span class="min-w-0 truncate text-xs font-medium text-slate-700 dark:text-slate-200">本机</span><span class="inline-flex shrink-0 items-center gap-1 text-xs text-emerald-600 dark:text-emerald-300"><i class="ri-checkbox-circle-line" aria-hidden="true"></i>已保存</span></div>
        <div v-for="storage in getStorageStatuses(image)" :key="`${image.id}-${storage.bucket_id}`" class="min-w-0 rounded-lg border border-slate-200/80 bg-slate-50 px-2 py-1.5 dark:border-white/10 dark:bg-slate-950">
          <div class="flex min-w-0 items-center justify-between gap-2"><span class="min-w-0 truncate text-xs font-medium text-slate-700 dark:text-slate-200" :title="getStorageDisplayName(storage)">{{ getStorageDisplayName(storage) }}</span><span class="inline-flex shrink-0 items-center gap-1 rounded-full border px-1.5 py-0.5 text-xs" :class="getStorageStatusMeta(storage.status).badgeClass"><i :class="getStorageStatusMeta(storage.status).icon" aria-hidden="true"></i>{{ getStorageStatusMeta(storage.status).label }}</span></div>
          <p v-if="storage.status === 'failed' && storage.error" class="mt-1 break-words text-xs text-red-600 dark:text-red-300">{{ storage.error }}</p>
        </div>
      </div>
      <button v-if="!image.deleting" type="button" class="soft-button mt-3" @click="openPreview(image)">预览与图片操作<i class="ri-arrow-right-s-line" aria-hidden="true"></i></button>
    </div>
  </article>
</template>
<script setup>
import { ref } from 'vue';
import { galleryUploader } from '@/utils/galleryBrowser.js';
defineProps(['retryDelete', 'image', 'viewMode', 'management', 'presetBuckets', 'isImageSelected', 'getRoleTagClass', 'handleImageSelection', 'openPreview', 'handleImageLoad', 'handleImageError', 'getFullUrl', 'formatFileSize', 'formatDate', 'multiStorageSync', 'getStorageDisplayName', 'getStorageStatuses', 'getStorageStatusMeta', 'getStorageSyncSummary', 'getSelectedAccessBucketId', 'getAccessSourceOptions', 'getAccessSourceOptionLabel', 'isAccessSourceUpdating', 'handleAccessSourceChange']);
const detailsOpen = ref(false);
</script>
<style scoped>
.gallery-image-card { min-width:0; cursor:default; box-shadow:none; }
.gallery-file-main { position:relative; display:flex; align-items:center; gap:.75rem; min-width:0; padding:.75rem; }
.gallery-card-checkbox { flex-shrink:0; display:grid; place-items:center; width:1.25rem; }
.image-wrapper { position:relative; display:grid; place-items:center; width:2.75rem; height:2.75rem; flex-shrink:0; overflow:hidden; background:rgb(148 163 184 / .1); border-radius:.35rem; }
.image-info { min-width:0; flex:1; padding:0 !important; }
.image-filename { display:block; max-width:100%; text-overflow:ellipsis; overflow:hidden; white-space:nowrap; font-size:.875rem; font-weight:500; text-align:left; }
.image-filename:hover:not(:disabled) { color:#2563eb; }
.gallery-file-secondary,.gallery-file-date,.gallery-uploader { color:#64748b; font-size:.75rem; margin-top:.2rem; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.image-role { display:inline-block; font-size:.625rem; padding:0 .25rem; border-radius:.2rem; }
.gallery-desktop-meta { display:none; flex-direction:column; gap:.2rem; font-size:.75rem; color:#64748b; font-variant-numeric:tabular-nums; }
.gallery-details-toggle { width:2.25rem; height:2.25rem; flex-shrink:0; color:#64748b; border-radius:.4rem; }
.gallery-details-toggle:hover { background:rgb(148 163 184 / .15); }
.gallery-file-details { border-top:1px solid rgb(148 163 184 / .2); background:rgb(148 163 184 / .04); padding:.85rem; }
.gallery-card-list { border:0; border-bottom:1px solid rgb(226 232 240 / .7); border-radius:0; }
.gallery-card-list .gallery-file-main:hover { background:rgb(59 130 246 / .03); }
.gallery-card-selected { background:rgb(59 130 246 / .06); }
.gallery-card-grid { border-radius:.75rem; align-self:start; }
.gallery-card-grid .gallery-file-main { display:grid; grid-template-columns:minmax(0,1fr) 2rem; gap:.6rem; padding:.6rem; }
.gallery-card-grid .gallery-card-checkbox { position:absolute; top:1rem; left:1rem; z-index:1; padding:.2rem; width:1.6rem; height:1.6rem; border-radius:.3rem; background:rgb(255 255 255 / .9); }
.gallery-card-grid .image-wrapper { width:100%; height:auto; aspect-ratio:1; grid-column:1 / -1; }
.gallery-card-grid .gallery-file-details { padding:.6rem; }
button:focus-visible { outline:2px solid #3b82f6; outline-offset:2px; }
:global(.dark) .gallery-card-list { border-color:rgb(255 255 255 / .1); }
:global(.dark) .gallery-file-secondary,:global(.dark) .gallery-file-date,:global(.dark) .gallery-uploader,:global(.dark) .gallery-desktop-meta,:global(.dark) .gallery-details-toggle { color:#94a3b8; }
@media (min-width:768px) { .gallery-card-list .gallery-desktop-meta { display:flex; width:11rem; } .gallery-card-list .gallery-file-date { display:none; } .gallery-card-list .gallery-file-details { margin-left:2.75rem; max-width:34rem; } }
@media (max-width:480px) { .gallery-file-main { padding:.65rem .3rem; gap:.5rem; } .gallery-file-date { font-size:.65rem; } .gallery-card-checkbox { width:1rem; } .gallery-card-grid .image-info { grid-column:1 / -1; } .gallery-card-grid .gallery-details-toggle { position:absolute; top:.85rem; right:.85rem; background:rgb(255 255 255 / .9); color:#475569; } }
</style>
