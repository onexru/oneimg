<template>
  <div class="min-w-0 space-y-1.5 text-xs" :class="compact ? '' : 'rounded-xl border border-slate-200/80 bg-slate-50 p-3 dark:border-white/10 dark:bg-slate-900'">
    <div class="flex flex-wrap items-center justify-between gap-2">
      <div class="min-w-0 flex-1">
        <p class="truncate font-medium text-slate-900 dark:text-white" :title="task.filename">{{ task.filename }}</p>
        <p class="text-slate-600 dark:text-slate-300" role="status">{{ view.label }}<span v-if="view.thumbnail"> · {{ view.thumbnail }}</span></p>
      </div>
      <div class="flex flex-wrap items-center gap-2">
        <button v-if="view.canConfirm" type="button" class="soft-button px-2 py-1 text-xs" @click="$emit('confirm', task)">确认上传状态</button>
        <button v-if="view.canRetry" type="button" class="soft-button px-2 py-1 text-xs" @click="$emit('retry', task)">{{ task.status === 'ready' ? '重试缩略图' : task.status === 'failed' ? '重试主图处理' : '重试此文件' }}</button>
        <button v-if="view.needsFile" type="button" class="soft-button px-2 py-1 text-xs" @click="$emit('reselect', task)">重新选择原文件</button>
        <button v-if="view.canCancel" type="button" class="soft-button px-2 py-1 text-xs" @click="$emit('cancel', task)">{{ task.busy ? '取消上传' : '取消任务' }}</button>
      </div>
    </div>
    <template v-if="view.showProgress">
      <progress class="block h-2 w-full accent-blue-600" :value="task.percent == null ? undefined : task.percent" max="100" :aria-label="`${task.filename} 文件传输进度`"></progress>
      <p class="text-slate-500 dark:text-slate-400">{{ bytes(task.loaded) }}<span v-if="task.total"> / {{ bytes(task.total) }}</span> · 传输完成后仍需后台处理主图</p>
    </template>
    <p v-if="task.clientError || task.message" class="break-words text-amber-700 dark:text-amber-300">{{ task.clientError || task.message }}<span v-if="task.error_code">（{{ task.error_code }}）</span></p>
    <p v-if="view.needsFile" class="text-slate-500 dark:text-slate-400">任务已保存，但浏览器原文件不会跨刷新保留。请重新选择同一文件继续原任务；服务器会核验大小与 SHA-256，不会新建图片。</p>
    <p v-if="task.next_retry_at && task.status !== 'ready'" class="text-slate-500 dark:text-slate-400">后台计划重试：{{ new Date(task.next_retry_at).toLocaleString('zh-CN') }}</p>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { taskPresentation } from '@/utils/directUpload.js';
const props = defineProps({ task: { type: Object, required: true }, compact: Boolean });
defineEmits(['retry', 'cancel', 'confirm', 'reselect']);
const view = computed(() => taskPresentation(props.task));
const bytes = value => value >= 1048576 ? `${(value / 1048576).toFixed(1)} MB` : `${((value || 0) / 1024).toFixed(1)} KB`;
</script>
