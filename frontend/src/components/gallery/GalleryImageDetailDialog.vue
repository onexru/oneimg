<template>
  <!-- 图片详情弹窗：取代卡片里展开的下拉抽屉。
       抽屉会把列表行撑高、打断浏览节奏，而且详情与「改读取源」分成两个弹窗很不连贯 ——
       这里把存储同步状态、访问链接读取源、删除未完成重试全部收进同一个弹窗。 -->
  <DialogShell ref="shellRef" :title="image.filename" :subtitle="subtitle" icon="ri-image-line" width="md" close-label="关闭图片详情" @close="onShellClosed">
    <div class="space-y-4 px-6 py-5">
      <!-- 删除未完成：先给结论，再给重试入口 -->
      <div v-if="image.deleting" class="rounded-xl border border-amber-200 bg-amber-50 p-3 dark:border-amber-500/30 dark:bg-amber-500/10">
        <p class="text-sm font-medium text-amber-800 dark:text-amber-200">删除尚未完成</p>
        <p class="mt-1 text-xs leading-5 text-amber-700 dark:text-amber-300">部分存储副本可能已删除，重试会把剩余副本继续删完。</p>
        <p v-for="replica in image.storage_statuses || []" :key="replica.bucket_id" class="mt-1 break-words text-xs text-amber-700 dark:text-amber-300">
          {{ replica.bucket_name || replica.bucket_type }}：{{ replica.error || '等待继续删除' }}
        </p>
        <button type="button" class="danger-button mt-2.5 !min-h-0 h-8 px-3 text-xs" :disabled="retrying" @click="retryDeleteImage">
          <i :class="retrying ? 'ri-loader-4-line animate-spin' : 'ri-refresh-line'" aria-hidden="true"></i>
          {{ retrying ? '重试中…' : '重试删除' }}
        </button>
      </div>

      <!-- 基本信息：只读，补上卡片里被省略的字段 -->
      <dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm">
        <dt class="text-slate-500 dark:text-slate-400">大小</dt>
        <dd class="min-w-0 text-slate-900 dark:text-slate-100">{{ formatFileSize(image.file_size) }}</dd>
        <dt class="text-slate-500 dark:text-slate-400">尺寸</dt>
        <dd class="min-w-0 text-slate-900 dark:text-slate-100">{{ image.width }}×{{ image.height }}</dd>
        <dt class="text-slate-500 dark:text-slate-400">上传时间</dt>
        <dd class="min-w-0 text-slate-900 dark:text-slate-100">{{ formatDate(image.created_at) }}</dd>
        <dt v-if="management" class="text-slate-500 dark:text-slate-400">上传者</dt>
        <dd v-if="management" class="min-w-0 truncate text-slate-900 dark:text-slate-100">{{ uploader }}</dd>
      </dl>

      <!-- 访问链接读取源：唯一可写的设置项 -->
      <div class="rounded-xl border border-slate-200 p-3 dark:border-white/10">
        <div class="flex items-center justify-between gap-2">
          <span class="field-label mb-0"><i class="ri-route-line mr-1" aria-hidden="true"></i>访问链接读取源</span>
          <span class="inline-flex min-w-0 items-center gap-1 text-xs font-medium text-slate-900 dark:text-slate-100">
            <span class="truncate">{{ currentSourceLabel }}</span>
            <i v-if="isUpdating" class="ri-loader-4-line animate-spin text-slate-400" aria-label="正在设置" aria-hidden="true"></i>
          </span>
        </div>
        <select
          :id="`detail-access-source-${image.id}`"
          class="input-modern mt-2"
          :value="String(currentBucketId)"
          :disabled="image.deleting || isUpdating || selectableSources.length === 0"
          aria-label="访问链接读取源"
          @change="applyAccessSource($event.target.value)"
        >
          <option v-for="source in accessSources" :key="source.bucket_id" :value="String(source.bucket_id)" :disabled="source.bucket_disabled || source.access_unavailable">
            {{ getAccessSourceOptionLabel(source) }}
          </option>
        </select>
        <p class="field-hint">
          {{ selectableSources.length > 1 ? '切换后图片链接立即改为从所选存储读取。' : '该图片目前只有一个可用的存储源。' }}
        </p>
      </div>

      <!-- 存储同步状态：本机单独置顶，其余按副本列出 -->
      <div class="space-y-1.5">
        <p class="field-label">存储状态</p>
        <div v-if="!multiStorageSync" class="flex min-w-0 items-center justify-between gap-2 rounded-lg border border-slate-200/80 bg-slate-50 px-2.5 py-2 dark:border-white/10 dark:bg-slate-950">
          <span class="min-w-0 truncate text-xs font-medium text-slate-700 dark:text-slate-200">{{ primaryStorageName }}</span>
          <span class="inline-flex shrink-0 items-center gap-1 text-xs text-emerald-600 dark:text-emerald-300"><i class="ri-checkbox-circle-line" aria-hidden="true"></i>已保存</span>
        </div>
        <template v-else>
          <div class="flex min-w-0 items-center justify-between gap-2 rounded-lg border border-slate-200/80 bg-slate-50 px-2.5 py-2 dark:border-white/10 dark:bg-slate-950">
            <span class="min-w-0 truncate text-xs font-medium text-slate-700 dark:text-slate-200">本机</span>
            <span class="inline-flex shrink-0 items-center gap-1 text-xs text-emerald-600 dark:text-emerald-300"><i class="ri-checkbox-circle-line" aria-hidden="true"></i>已保存</span>
          </div>
          <div v-for="storage in storageStatuses" :key="storage.bucket_id" class="min-w-0 rounded-lg border border-slate-200/80 bg-slate-50 px-2.5 py-2 dark:border-white/10 dark:bg-slate-950">
            <div class="flex min-w-0 items-center justify-between gap-2">
              <span class="min-w-0 truncate text-xs font-medium text-slate-700 dark:text-slate-200" :title="getStorageDisplayName(storage)">{{ getStorageDisplayName(storage) }}</span>
              <span class="inline-flex shrink-0 items-center gap-1 rounded-full border px-1.5 py-0.5 text-xs" :class="getStorageStatusMeta(storage.status).badgeClass">
                <i :class="getStorageStatusMeta(storage.status).icon" aria-hidden="true"></i>{{ getStorageStatusMeta(storage.status).label }}
              </span>
            </div>
            <p v-if="storage.status === 'failed' && storage.error" class="mt-1 break-words text-xs text-red-600 dark:text-red-300">{{ storage.error }}</p>
          </div>
        </template>
      </div>
    </div>

    <template #footer>
      <!-- 预览是次要动作，放左侧；主操作是关闭 -->
      <button type="button" class="soft-button mr-auto" :disabled="image.deleting" @click="previewImage">
        <i class="ri-eye-line" aria-hidden="true"></i>预览大图
      </button>
      <button type="button" class="primary-button" @click="close">关闭</button>
    </template>
  </DialogShell>
</template>

<script setup>
import { computed, ref } from 'vue';
import DialogShell from '@/components/DialogShell.vue';
import { galleryUploader } from '@/utils/galleryBrowser.js';

const props = defineProps({
  image: { type: Object, required: true },
  management: { type: Boolean, default: false },
  multiStorageSync: { type: Boolean, default: false },
  presetBuckets: { type: Array, default: () => [] },
  formatFileSize: { type: Function, required: true },
  formatDate: { type: Function, required: true },
  getStorageDisplayName: { type: Function, required: true },
  getStorageStatuses: { type: Function, required: true },
  getStorageStatusMeta: { type: Function, required: true },
  getSelectedAccessBucketId: { type: Function, required: true },
  getAccessSourceOptions: { type: Function, required: true },
  getAccessSourceOptionLabel: { type: Function, required: true },
  isAccessSourceUpdating: { type: Function, required: true },
  applyAccessSource: { type: Function, required: true },
  retryDelete: { type: Function, required: true },
  openPreview: { type: Function, required: true },
});
const emit = defineEmits(['close']);

const shellRef = ref(null);
const retrying = ref(false);
const close = () => shellRef.value?.requestClose();
// 动画播完后才让父级销毁，否则内容先消失、留个空壳在淡出
const onShellClosed = () => emit('close');

const uploader = computed(() => galleryUploader(props.image));
const accessSources = computed(() => props.getAccessSourceOptions(props.image));
const selectableSources = computed(() => accessSources.value.filter(source => !source.bucket_disabled && !source.access_unavailable));
const currentBucketId = computed(() => props.getSelectedAccessBucketId(props.image));
const currentSourceLabel = computed(() => {
  const source = accessSources.value.find(item => Number(item.bucket_id) === Number(currentBucketId.value));
  return source ? props.getAccessSourceOptionLabel(source) : '未知存储';
});
const storageStatuses = computed(() => props.getStorageStatuses(props.image));
const primaryStorageName = computed(() => props.presetBuckets.find(bucket => bucket.id == props.image.bucket_id)?.name || '本地存储');
const isUpdating = computed(() => props.isAccessSourceUpdating(props.image.id));

const subtitle = computed(() => props.management ? '查看详情、切换访问源与存储同步状态。' : '查看详情，并设置访问链接从哪个存储读取。');

// select 里改一次就提交一次：这里只有一个设置项，再加一层「应用」按钮属于多余确认
const applyAccessSource = async value => {
  const bucketId = Number(value);
  if (!bucketId || bucketId === currentBucketId.value) return;
  await props.applyAccessSource(props.image, bucketId);
};

const previewImage = () => {
  close();
  props.openPreview(props.image);
};

const retryDeleteImage = async () => {
  if (retrying.value) return;
  retrying.value = true;
  try {
    const ok = await props.retryDelete(props.image.id);
    if (ok) close();
  } finally {
    retrying.value = false;
  }
};
</script>
