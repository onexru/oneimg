<template>
        <div
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
              loading="lazy" decoding="async"
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

              <DirectUploadTaskStatus v-if="directTaskForImage(image.id)" :task="directTaskForImage(image.id)" compact
                @retry="retryDirectTask" @cancel="cancelDirectTask" @confirm="confirmDirectTask" @reselect="reselectDirectFile" />

              <div v-if="multiStorageSync" class="grid gap-1.5 sm:grid-cols-2">
                <div class="flex min-w-0 items-center justify-between gap-2 rounded-xl border border-slate-200/80 bg-slate-50 px-2.5 py-1.5 dark:border-white/10 dark:bg-slate-900">
                  <span class="min-w-0 truncate text-xs font-medium text-slate-700 dark:text-slate-200">本机</span>
                  <span class="inline-flex shrink-0 items-center gap-1 text-[11px] text-emerald-600 dark:text-emerald-300">
                    <i class="ri-checkbox-circle-line"></i>已保存
                  </span>
                </div>
                <div
                  v-for="storage in getStorageStatuses(image)"
                  :key="`${image.id}-${storage.bucket_id}`"
                  class="min-w-0 rounded-xl border border-slate-200/80 bg-slate-50 px-2.5 py-1.5 dark:border-white/10 dark:bg-slate-900"
                >
                  <div class="flex min-w-0 items-center justify-between gap-2">
                    <span class="min-w-0 truncate text-xs font-medium text-slate-700 dark:text-slate-200" :title="getStorageDisplayName(storage)">{{ getStorageDisplayName(storage) }}</span>
                    <span class="inline-flex shrink-0 items-center gap-1 rounded-full border px-1.5 py-0.5 text-[10px]" :class="getStorageStatusMeta(storage.status).badgeClass">
                      <i :class="getStorageStatusMeta(storage.status).icon"></i>{{ getStorageStatusMeta(storage.status).label }}
                    </span>
                  </div>
                  <p v-if="storage.status === 'failed' && storage.error" class="mt-1 truncate text-[10px] text-red-600 dark:text-red-300" :title="storage.error">{{ storage.error }}</p>
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
</template>
<script setup>
import DirectUploadTaskStatus from '@/components/DirectUploadTaskStatus.vue';
defineProps(["image", "multiStorageSync", "formatFileSize", "getFullUrl", "handleImageLoad", "handleImageError", "previewImage", "downloadImage", "deleteImage", "directTaskForImage", "retryDirectTask", "cancelDirectTask", "confirmDirectTask", "reselectDirectFile", "getStorageStatuses", "getStorageStatusMeta", "getStorageDisplayName", "copyImageLink", "getHtmlCode", "getMarkdownCode"]);
</script>
