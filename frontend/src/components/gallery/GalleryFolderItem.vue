<template>
  <article class="gallery-folder-item" :class="`folder-${viewMode}`" :data-folder-id="folder.id">
    <button type="button" class="folder-open" :aria-label="`打开文件夹 ${folder.name}`" @click="$emit('open', folder.id)">
      <span class="folder-icon"><i class="ri-folder-3-fill" aria-hidden="true"></i></span>
      <span class="folder-copy">
        <span class="folder-name" :title="folder.name">{{ folder.name }}</span>
        <span class="folder-description" :title="folder.description">{{ folder.description || '文件夹' }}</span>
        <span v-if="folder.deleting" class="text-amber-700 dark:text-amber-300 text-xs">删除未完成</span>
      </span>
      <span class="folder-count">{{ folder.image_count }} 张</span>
    </button>
    <div class="folder-actions">
      <button type="button" class="folder-action" :disabled="folder.deleting" :aria-label="`编辑文件夹 ${folder.name}`" title="编辑文件夹" @click="$emit('edit', folder)"><i class="ri-edit-line" aria-hidden="true"></i></button>
      <button type="button" class="folder-action text-red-600 dark:text-red-400" :aria-label="`${folder.deleting ? '重试删除文件夹' : '删除文件夹'} ${folder.name}`" :title="folder.deleting ? '重试删除文件夹' : '删除文件夹'" @click="$emit('delete', folder)"><i class="ri-delete-bin-line" aria-hidden="true"></i></button>
    </div>
  </article>
</template>
<script setup>
defineProps({ folder: { type: Object, required: true }, viewMode: { type: String, default: 'list' } });
defineEmits(['open', 'edit', 'delete']);
</script>
<style scoped>
.gallery-folder-item { display:flex; align-items:center; min-width:0; border-bottom:1px solid rgb(226 232 240 / .7); }
.folder-open { display:flex; flex:1; gap:.75rem; align-items:center; min-width:0; padding:.7rem .75rem; text-align:left; border-radius:.4rem; }
.folder-open:hover { background:rgb(59 130 246 / .05); }
.folder-icon { display:grid; place-items:center; width:2.75rem; height:2.75rem; flex-shrink:0; color:#d49a34; background:rgb(245 158 11 / .1); border-radius:.5rem; font-size:1.6rem; }
.folder-copy { flex:1; min-width:0; display:flex; flex-direction:column; gap:.15rem; }
.folder-name { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-weight:500; font-size:.875rem; }
.folder-description { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; color:#64748b; font-size:.75rem; }
.folder-count { flex-shrink:0; font-size:.75rem; color:#64748b; font-variant-numeric:tabular-nums; }
.folder-actions { display:flex; padding-right:.5rem; }
.folder-action { width:2.25rem; height:2.25rem; border-radius:.4rem; }
.folder-action:hover { background:rgb(148 163 184 / .15); }
.folder-action:disabled { opacity:.4; cursor:not-allowed; }
button:focus-visible { outline:2px solid #3b82f6; outline-offset:-2px; }
.folder-grid { border:1px solid rgb(226 232 240 / .7); border-radius:.75rem; flex-wrap:wrap; }
.folder-grid .folder-open { flex-basis:100%; }
.folder-grid .folder-actions { margin-left:auto; padding-bottom:.35rem; }
:global(.dark) .gallery-folder-item { border-color:rgb(255 255 255 / .1); }
:global(.dark) .folder-description, :global(.dark) .folder-count { color:#94a3b8; }
@media (max-width:480px) { .folder-open { gap:.5rem; padding:.65rem .3rem; } .folder-actions { padding-right:0; } .folder-count { font-size:.7rem; } }
</style>
