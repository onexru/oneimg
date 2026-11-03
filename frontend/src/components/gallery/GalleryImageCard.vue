<template>
  <article class="gallery-image-card gallery-image-card-compact" :class="[`gallery-card-${viewMode}`, { 'gallery-card-selected': isImageSelected(image.id) }]" :data-image-id="image.id">
    <div class="gallery-file-main">
      <label class="gallery-card-checkbox" :for="`image-${image.id}`" @click.stop>
        <input type="checkbox" :id="`image-${image.id}`" :aria-label="`选择图片 ${image.filename}`" class="app-checkbox" :checked="isImageSelected(image.id)" @change="(e) => handleImageSelection(image.id, e.target.checked)" @click.stop />
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
      <!-- 详情不再内联展开：点这里打开详情弹窗，列表行高保持恒定 -->
      <button
        type="button" class="gallery-details-toggle"
        :aria-label="`查看图片详情 ${image.filename}`"
        @click.stop="openDetails(image)"
      ><i class="ri-more-2-fill" aria-hidden="true"></i></button>
    </div>
  </article>
</template>
<script setup>
import { galleryUploader } from '@/utils/galleryBrowser.js';
defineProps(['image', 'viewMode', 'management', 'isImageSelected', 'getRoleTagClass', 'handleImageSelection', 'openPreview', 'handleImageLoad', 'handleImageError', 'formatFileSize', 'formatDate', 'openDetails']);
</script>
<style scoped>
.gallery-image-card { min-width:0; cursor:default; box-shadow:none; }
.gallery-file-main { position:relative; display:flex; align-items:center; gap:.75rem; min-width:0; padding:.75rem; }
.gallery-card-checkbox { flex-shrink:0; }
.image-wrapper { position:relative; display:grid; place-items:center; width:2.75rem; height:2.75rem; flex-shrink:0; overflow:hidden; background:rgb(148 163 184 / .1); border-radius:.35rem; }
.image-info { min-width:0; flex:1; padding:0 !important; }
.image-filename { display:block; max-width:100%; text-overflow:ellipsis; overflow:hidden; white-space:nowrap; font-size:.875rem; font-weight:500; text-align:left; }
.image-filename:hover:not(:disabled) { color:#2563eb; }
.gallery-file-secondary,.gallery-file-date,.gallery-uploader { color:#64748b; font-size:.75rem; margin-top:.2rem; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.image-role { display:inline-block; font-size:.625rem; padding:0 .25rem; border-radius:.2rem; }
.gallery-desktop-meta { display:none; flex-direction:column; gap:.2rem; font-size:.75rem; color:#64748b; font-variant-numeric:tabular-nums; }
.gallery-details-toggle { width:2.25rem; height:2.25rem; flex-shrink:0; color:#64748b; border-radius:.4rem; }
.gallery-details-toggle:hover { background:rgb(148 163 184 / .15); }
.gallery-card-list { border:0; border-bottom:1px solid rgb(226 232 240 / .7); border-radius:0; }
.gallery-card-list .gallery-file-main:hover { background:rgb(59 130 246 / .03); }
.gallery-card-selected { background:rgb(59 130 246 / .06); }
.gallery-card-grid { border-radius:.75rem; align-self:start; }
.gallery-card-grid .gallery-file-main { display:grid; grid-template-columns:minmax(0,1fr) 2rem; gap:.6rem; padding:.6rem; }
.gallery-card-grid .gallery-card-checkbox { position:absolute; top:1rem; left:1rem; z-index:1; }
.gallery-card-grid .image-wrapper { width:100%; height:auto; aspect-ratio:1; grid-column:1 / -1; }
button:focus-visible { outline:none; box-shadow:0 0 0 2px rgb(37 99 235 / .45); }
:global(.dark) .gallery-card-list { border-color:rgb(255 255 255 / .1); }
:global(.dark) .gallery-file-secondary,:global(.dark) .gallery-file-date,:global(.dark) .gallery-uploader,:global(.dark) .gallery-desktop-meta,:global(.dark) .gallery-details-toggle { color:#94a3b8; }
@media (min-width:768px) { .gallery-card-list .gallery-desktop-meta { display:flex; width:11rem; } .gallery-card-list .gallery-file-date { display:none; } }
@media (max-width:480px) { .gallery-file-main { padding:.65rem .3rem; gap:.5rem; } .gallery-file-date { font-size:.65rem; } .gallery-card-grid .image-info { grid-column:1 / -1; } .gallery-card-grid .gallery-details-toggle { position:absolute; top:.85rem; right:.85rem; background:rgb(255 255 255 / .9); color:#475569; } }
</style>
