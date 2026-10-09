<template>
  <Teleport to="body">
    <div class="fixed inset-0 bg-black/50 dark:bg-black/70 flex items-center justify-center transition-opacity duration-300" :style="{ zIndex: layer }" @click.self="$emit('close')">
      <div ref="dialog" :class="{ 'folder-dialog-shell': presentation === 'folder' }" class="modal bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-md mx-4 transform transition-all duration-300 scale-100 overflow-y-auto max-h-[90dvh]" style="max-width:min(28rem,calc(100vw - 2rem))">
        <div class="modal-header p-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
          <div class="min-w-0">
            <h3 :id="titleId" class="modal-title text-lg font-bold text-gray-800 dark:text-white"><i v-if="icon" :class="icon" aria-hidden="true"></i>{{ title }}</h3>
            <p v-if="subtitle" class="dialog-subtitle">{{ subtitle }}</p>
          </div>
          <button type="button" :aria-label="closeLabel" class="modal-close text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 text-xl font-bold" @click="$emit('close')">×</button>
        </div>
        <slot />
        <div v-if="$slots.footer" class="dialog-footer flex flex-wrap justify-end gap-2 border-t border-slate-200 p-4 dark:border-white/10"><slot name="footer" /></div>
      </div>
    </div>
  </Teleport>
</template>
<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { activateDialog, nextDialogLayer } from '@/utils/overlay.js'
const props = defineProps({ title: { type: String, default: '安全验证' }, closeLabel: { type: String, default: '关闭验证弹窗' }, subtitle: String, icon: String, presentation: String, initialFocus: String })
const emit = defineEmits(['close'])
const dialog = ref(null)
const layer = nextDialogLayer()
const titleId = `dialog-title-${Math.random().toString(36).slice(2)}`
let deactivate
onMounted(() => { deactivate = activateDialog(dialog.value, { labelId: titleId, layer, initialFocus: props.initialFocus, close: () => emit('close') }) })
onBeforeUnmount(() => deactivate?.())
</script>
<style scoped>
.folder-dialog-shell { border-radius: 20px; border: 1px solid rgb(148 163 184 / .22); }
.folder-dialog-shell .modal-header { padding: 22px 24px 8px; border-bottom: 0; align-items: flex-start; gap: 12px; }
.folder-dialog-shell .modal-title { display: flex; align-items: center; gap: 10px; font-size: 20px; line-height: 1.4; }
.folder-dialog-shell .modal-title i { font-size: 24px; color: #0284c7; }
.folder-dialog-shell .dialog-subtitle { color: #64748b; font-size: 13px; margin-top: 8px; line-height: 1.6; }
.folder-dialog-shell .modal-close { width: 32px; height: 32px; flex: 0 0 32px; border-radius: 8px; }
.folder-dialog-shell .dialog-footer { border-top: 0; padding: 16px 24px 24px; }
.folder-dialog-shell .dialog-footer :deep(button) { flex: 1; min-width: 0; }
:global(.dark) .folder-dialog-shell .dialog-subtitle { color: #94a3b8; }
@media (max-width: 480px) {
  .folder-dialog-shell .modal-header { padding: 20px 18px 8px; }
  .folder-dialog-shell .dialog-footer { flex-direction: column; padding: 14px 18px 20px; }
  .folder-dialog-shell .dialog-footer :deep(button) { width: 100%; }
}
</style>
