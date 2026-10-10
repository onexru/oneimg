<template>
  <DialogShell :title="title" :subtitle="subtitle" :icon="dialogIcon" presentation="folder" :initial-focus="initialFocus" close-label="关闭文件夹弹窗" @close="close">
    <form id="folder-action-form" class="folder-dialog-body space-y-4 p-4" @submit.prevent="submit">
      <template v-if="mode === 'create' || mode === 'rename'">
        <div>
          <label for="folder-name" class="field-label">文件夹名称</label>
          <input id="folder-name" name="name" v-model="name" class="input-modern" placeholder="请输入文件夹名称…" autocomplete="off" :disabled="busy || actionBlocked" required :aria-invalid="!!error" aria-describedby="folder-name-hint">
          <p id="folder-name-hint" class="field-hint">1–64 个字符，不支持 / 或 \。</p>
        </div>
        <div>
          <label for="folder-description" class="field-label">描述 <span class="optional-label">（可选）</span></label>
          <textarea id="folder-description" name="description" v-model="description" class="input-modern" rows="3" placeholder="请输入文件夹描述…" :disabled="busy || actionBlocked" aria-describedby="folder-description-hint"></textarea>
          <p id="folder-description-hint" class="field-hint description-count">{{ [...description.trim()].length }} / 500</p>
        </div>
      </template>
      <template v-else-if="mode === 'delete'">
        <p class="break-words text-sm leading-6">确定删除文件夹「{{ folder.name }}」？</p>
        <p v-if="!deleteImages" class="rounded-xl bg-slate-50 p-3 text-sm leading-6 dark:bg-white/5">文件夹内的图片将返回「未分类」，不会删除任何图片。</p>
        <label class="flex items-start gap-2 text-sm leading-6">
          <input id="delete-folder-images" name="delete_images" v-model="deleteImages" type="checkbox" class="mt-1 h-4 w-4 shrink-0" :disabled="busy || cascadeLocked">
          <span>同时删除文件夹内图片</span>
        </label>
        <p v-if="cascadeLocked" class="text-sm text-amber-700 dark:text-amber-300">此文件夹已开始删除，只能继续删除，不能改为保留图片。重试前请再次输入完整名称。</p>
        <div v-if="deleteImages" class="space-y-3">
          <p role="note" class="rounded-xl border border-red-200 bg-red-50 p-3 text-sm leading-6 text-red-700 dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-300">此操作会永久删除文件夹内所有图片及其存储副本，外部图片链接将失效，且无法恢复。删除未完成时文件夹会保留，请检查错误后重试。</p>
          <label for="delete-folder-confirm" class="field-label">输入文件夹名称</label>
          <input id="delete-folder-confirm" name="confirm_name" v-model="confirmName" class="input-modern" autocomplete="off" :disabled="busy" aria-describedby="delete-folder-confirm-hint">
          <p id="delete-folder-confirm-hint" class="field-hint break-words">请输入完整名称「{{ folder.name }}」以确认，空格和大小写必须完全一致。</p>
        </div>
      </template>
      <template v-else-if="mode === 'move'">
        <p class="text-sm text-slate-500">将已选的 {{ imageIds.length }} 张本人图片移动到：</p>
        <label for="move-folder" class="field-label">目标文件夹</label>
        <select id="move-folder" v-model="destination" class="input-modern min-w-0 max-w-full" :disabled="busy">
          <option value="0">未分类</option>
          <option v-for="item in folders" :key="item.id" :value="String(item.id)" :disabled="item.deleting">{{ folderLabel(item) }}</option>
        </select>
      </template>
      <p v-if="progress" role="status" aria-live="polite" class="text-sm text-slate-600 dark:text-slate-300">
        本次已删除 {{ progress.total_deleted_count }} 张图片<span v-if="Number.isSafeInteger(progress.remaining_count)">，剩余 {{ progress.remaining_count }} 张图片</span><span v-if="Number.isSafeInteger(progress.pending_upload_count)">，待处理上传 {{ progress.pending_upload_count }} 个</span><span v-if="progress.failed_count > 0">，本批失败 {{ progress.failed_count }} 项</span>。{{ busy ? '正在继续删除…' : '自动删除已停止。' }}
      </p>
      <p v-if="actionBlocked" role="alert" class="text-sm text-amber-700">删除中的文件夹或图片不能重命名或移动，请刷新后选择可用项目。</p>
      <p v-if="error" id="folder-dialog-error" role="alert" class="break-words text-sm text-red-600 dark:text-red-400">{{ error }}</p>
    </form>
    <template #footer>
      <button type="submit" form="folder-action-form" class="folder-submit" :class="mode === 'delete' ? 'danger-button' : 'primary-button'" :disabled="busy || actionBlocked || !confirmationReady || (isEditing && (!name.trim() || [...description.trim()].length > 500))">{{ busy ? '处理中…' : submitLabel }}</button>
      <button type="button" class="soft-button folder-cancel" :disabled="busy" @click="close">取消</button>
    </template>
  </DialogShell>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';
import DialogShell from '@/components/DialogShell.vue';
import { createFolderApi, continueFolderDeletion, folderAccount, folderLabel, folderNameError, folderDescriptionError, FOLDER_LIMIT } from '@/utils/folders.js';
const props = defineProps({
  mode: { type: String, required: true }, folder: { type: Object, default: () => ({}) },
  folders: { type: Array, default: () => [] }, imageIds: { type: Array, default: () => [] },
  moveAllowed: { type: Boolean, default: true },
  initialDestination: { type: String, default: '0' }, accountKey: { type: String, required: true },
});
const emit = defineEmits(['close', 'saved', 'partial-failure']);
const api = createFolderApi(import.meta.env.VITE_API_BASE_URL || '');
const name = ref(props.folder.name || ''), destination = ref(props.initialDestination), busy = ref(false), error = ref('');
const description = ref(props.folder.description || '');
const isEditing = computed(() => ['create', 'rename'].includes(props.mode));
const subtitle = computed(() => ({ create: '在根目录创建新文件夹', rename: '修改名称或描述，不影响图片链接', move: '选择图片的存放位置' })[props.mode] || '请确认删除范围后继续');
const dialogIcon = computed(() => props.mode === 'delete' ? 'ri-delete-bin-line' : 'ri-folder-line');
const initialFocus = computed(() => isEditing.value ? '#folder-name' : props.mode === 'move' ? '#move-folder' : undefined);
const serverDeleting = computed(() => !!props.folder.deleting || !!props.folders.find(item => item.id === props.folder.id)?.deleting);
const cascadeLocked = ref(serverDeleting.value);
const deleteImages = ref(cascadeLocked.value), confirmName = ref(''), progress = ref(null);
watch(serverDeleting, deleting => { if (deleting) { cascadeLocked.value = true; deleteImages.value = true; if (!busy.value) confirmName.value = ''; } });
const actionBlocked = computed(() => (props.mode === 'rename' && serverDeleting.value) || (props.mode === 'move' && (!props.moveAllowed || serverDeleting.value || !!props.folders.find(item => String(item.id) === destination.value)?.deleting)));
const confirmationReady = computed(() => props.mode !== 'delete' || (!deleteImages.value && !cascadeLocked.value) || confirmName.value === props.folder.name);
let disposed = false, controller;
const owner = props.accountKey;
const isCurrent = () => !disposed && props.accountKey === owner && folderAccount() === owner;
function checkAccount() { if (!isCurrent()) { controller?.abort(); emit('close'); } }
watch(() => props.accountKey, checkAccount, { flush: 'sync' });
onMounted(() => { window.addEventListener('storage', checkAccount); window.addEventListener('focus', checkAccount); });
onBeforeUnmount(() => { disposed = true; controller?.abort(); window.removeEventListener('storage', checkAccount); window.removeEventListener('focus', checkAccount); });
const title = computed(() => ({ create: '新建文件夹', rename: '重命名文件夹', delete: cascadeLocked.value ? '继续删除文件夹' : '删除文件夹', move: '移动图片' })[props.mode]);
const submitLabel = computed(() => ({ create: '创建', rename: '保存名称', delete: cascadeLocked.value ? '重试删除文件夹及图片' : deleteImages.value ? '删除文件夹及图片' : '删除文件夹', move: '确认移动' })[props.mode]);
function close() { if (!busy.value) emit('close'); }
async function submit() {
  if (busy.value || actionBlocked.value || !confirmationReady.value) return;
  error.value = '';
  if (!owner || !isCurrent()) { error.value = '账户已变更，请刷新页面后重试'; return; }
  if (['create', 'rename'].includes(props.mode)) {
    error.value = folderNameError(name.value, props.folders, props.mode === 'rename' ? props.folder.id : null) || folderDescriptionError(description.value);
    if (!error.value && props.mode === 'create' && props.folders.length >= FOLDER_LIMIT) error.value = '最多可创建 200 个文件夹';
    if (error.value) return;
  }
  busy.value = true;
  progress.value = null;
  controller = new AbortController();
  const deletingImages = props.mode === 'delete' && (deleteImages.value || cascadeLocked.value);
  try {
    let data;
    if (props.mode === 'create') data = await api.create(name.value, description.value);
    else if (props.mode === 'rename') data = await api.rename(props.folder.id, name.value, description.value);
    else if (props.mode === 'delete') {
      if (deletingImages) data = await continueFolderDeletion(api, props.folder.id, { confirmName: confirmName.value, signal: controller.signal, isCurrent,
        onProgress: value => {
          progress.value = value;
          if (value.deleting) { cascadeLocked.value = true; deleteImages.value = true; }
        },
      });
      else data = await api.remove(props.folder.id, { signal: controller.signal });
    }
    else if (props.mode === 'move') {
      if (!props.imageIds.length) throw new Error('请选择本人上传的图片');
      data = await api.move(props.imageIds, destination.value);
      // Already-in-destination IDs are valid no-ops; the server counts only changes.
      if (!Number.isInteger(data?.moved_count) || data.moved_count < 0 || data.moved_count > new Set(props.imageIds).size) throw new Error('移动结果无效，请刷新图库确认结果');
    }
    if (isCurrent()) emit('saved', { mode: props.mode, folder: data, folderId: props.folder.id, deletedImages: deletingImages });
  } catch (caught) {
    if (isCurrent() && caught.name !== 'AbortError') {
      error.value = caught.message || '文件夹操作失败，请重试';
      if (deletingImages) {
        if (caught.code === 'folder_delete_busy') { cascadeLocked.value = true; deleteImages.value = true; }
        confirmName.value = '';
        error.value += '；删除未完成，可能已有部分图片或存储副本被删除。正在刷新图库和数量，请检查后重试。';
        emit('partial-failure');
      }
    }
  } finally { if (!disposed) busy.value = false; controller = undefined; }
}
</script>

<style scoped>
input, select, textarea { min-width: 0; max-width: 100%; }
.folder-dialog-body { padding: 20px 24px 8px; }
.folder-dialog-body .field-label { display: block; margin-bottom: 8px; font-size: 14px; font-weight: 600; }
.folder-dialog-body .input-modern { min-height: 44px; width: 100%; font-size: 15px; border-radius: 12px; }
.folder-dialog-body textarea.input-modern { resize: vertical; min-height: 96px; max-height: 180px; line-height: 1.6; padding: 12px; }
.field-hint, .optional-label { color: #64748b; font-size: 12px; font-weight: 400; }
.field-hint { margin-top: 6px; }
.description-count { text-align: right; }
.folder-submit, .folder-cancel { min-height: 44px; }
button:disabled { cursor: not-allowed; opacity: .55; }
@media (max-width: 480px) { .folder-dialog-body { padding: 16px 18px 4px; } }
</style>
