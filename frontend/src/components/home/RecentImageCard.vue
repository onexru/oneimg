<template>
  <!-- 最近上传：一张图一张卡片，左右布局。
       左：缩略图；右：上面一行文件名，下面一行「大小 · 分辨率」。
       复制格式由父级标题处的胶囊统一控制，卡片里只留复制和删除。 -->
  <div class="recent-card">
    <button
      type="button"
      class="recent-thumb"
      :title="`预览 ${image.filename}`"
      @click.stop="previewImage(image)"
    >
      <img
        :src="getFullUrl(image.thumbnail || image.url)"
        :alt="image.filename || '图片预览'"
        class="recent-image h-full w-full object-cover opacity-0"
        loading="lazy" decoding="async"
        @load="handleImageLoad"
        @error="(e) => handleImageError(e, image)"
      />
    </button>

    <div class="recent-info">
      <!-- 文件名在上 -->
      <button
        type="button"
        class="recent-name"
        :title="image.filename"
        @click.stop="previewImage(image)"
      >
        {{ image.filename }}
      </button>

      <!-- 元信息在下 -->
      <p class="recent-meta-line">
        <span>{{ formatFileSize(image.file_size) }}</span>
        <span class="recent-dot" aria-hidden="true"></span>
        <span>{{ image.width }}×{{ image.height }}</span>
      </p>

      <!-- 多存储同步状态：只在配置了同步时出现 -->
      <p v-if="multiStorageSync" class="recent-meta-line">
        <span class="recent-storage-inline">
          <i class="ri-computer-line" aria-hidden="true"></i>本机
          <i class="ri-checkbox-circle-line text-emerald-600 dark:text-emerald-400" aria-hidden="true"></i>
        </span>
        <span
          v-for="storage in getStorageStatuses(image)"
          :key="`${image.id}-${storage.bucket_id}`"
          class="recent-storage-inline"
          :title="storage.status === 'failed' && storage.error ? storage.error : getStorageDisplayName(storage)"
        >
          <i class="ri-cloud-line" aria-hidden="true"></i>
          <span class="max-w-[120px] truncate">{{ getStorageDisplayName(storage) }}</span>
          <i :class="[getStorageStatusMeta(storage.status).icon, getStorageStatusMeta(storage.status).badgeClass]" aria-hidden="true"></i>
        </span>
      </p>

      <DirectUploadTaskStatus v-if="directTaskForImage(image.id)" :task="directTaskForImage(image.id)" compact
        @retry="retryDirectTask" @cancel="cancelDirectTask" @confirm="confirmDirectTask" @reselect="reselectDirectFile" />
    </div>

    <div class="recent-card-tools">
      <button
        type="button"
        class="recent-icon-btn"
        :title="`复制${formatLabel}`"
        :aria-label="`复制${formatLabel}`"
        @click.stop="copyImageLink(image, copyFormat)"
      >
        <i class="ri-file-copy-line"></i>
      </button>

      <button
        type="button"
        class="recent-icon-btn recent-icon-btn-danger"
        title="删除图片"
        aria-label="删除图片"
        @click.stop="deleteImage(image.id)"
      >
        <i class="ri-delete-bin-line"></i>
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import DirectUploadTaskStatus from '@/components/DirectUploadTaskStatus.vue';

const props = defineProps(["image", "copyFormat", "multiStorageSync", "formatFileSize", "getFullUrl", "handleImageLoad", "handleImageError", "previewImage", "downloadImage", "deleteImage", "directTaskForImage", "retryDirectTask", "cancelDirectTask", "confirmDirectTask", "reselectDirectFile", "getStorageStatuses", "getStorageStatusMeta", "getStorageDisplayName", "copyImageLink", "getHtmlCode", "getMarkdownCode"]);

// 复制格式由父级「最近上传」标题处的胶囊控件统一控制，这里只做展示与取用
const formatLabel = computed(() => ({ url: 'URL', html: 'HTML', markdown: 'Markdown', bbcode: 'BBCode' })[props.copyFormat] || 'URL')
</script>
