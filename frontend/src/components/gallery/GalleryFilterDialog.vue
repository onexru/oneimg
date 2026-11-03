<template>
  <!-- 筛选弹窗：把原本内联展开的筛选区收进 PopupModal，
       打开时不挤压工具栏与图库列表，参数也不会边点边刷新（只在「应用」时提交一次）。 -->
  <DialogShell ref="shellRef" title="筛选图片" icon="ri-filter-3-line" width="md" close-label="关闭筛选弹窗" @close="onShellClosed">
    <!-- 内容区边距必须自己声明：Vue 宿主模式下 PopupModal 的内容容器只有 min-h-0，
         padding 交给内容控制（命令式弹窗默认是 px-6 py-5，这里保持一致）。 -->
    <form id="gallery-filter-form" class="space-y-4 px-6 py-5" @submit.prevent="apply">
      <!-- 上传者角色：仅总图库 -->
      <div v-if="roles.length" class="space-y-1.5">
        <span class="field-label">上传者角色</span>
        <div class="flex flex-wrap gap-1.5" role="group" aria-label="上传者角色">
          <button
            v-for="option in roles" :key="option.value" type="button" class="filter-chip"
            :class="draft.role === option.value ? 'filter-chip-active' : ''"
            :aria-pressed="draft.role === option.value"
            :data-filter-role="option.value"
            @click="draft.role = option.value"
          >{{ option.label }}</button>
        </div>
      </div>

      <!-- 文件夹：切换的是浏览范围，所以单独放最前面 -->
      <div v-if="foldersAuthenticated">
        <label for="filter-folder" class="field-label">文件夹</label>
        <select id="filter-folder" v-model="draft.folder" class="input-modern" :disabled="!foldersLoaded">
          <option value="0">根目录 · {{ unfiledCount }} 张未分类</option>
          <option value="">所有图片 · {{ folderTotalImages }}</option>
          <option v-for="folder in folders" :key="folder.id" :value="String(folder.id)">{{ folderLabel(folder) }} · {{ folder.image_count }}</option>
        </select>
        <p v-if="!foldersLoaded" class="field-hint">文件夹列表加载中…</p>
      </div>

      <div class="grid gap-4 sm:grid-cols-2">
        <div>
          <label for="filter-bucket" class="field-label">存储</label>
          <select id="filter-bucket" v-model="draft.bucket" class="input-modern">
            <option value="">全部存储</option>
            <option v-for="bucket in buckets" :key="bucket.id" :value="String(bucket.id)">{{ bucket.name }}</option>
          </select>
        </div>
        <div>
          <label for="filter-deletion" class="field-label">删除状态</label>
          <select id="filter-deletion" v-model="draft.deletion" class="input-modern">
            <option value="all">全部记录</option>
            <option value="active">正常图片</option>
            <option value="deleting">删除未完成</option>
          </select>
        </div>
      </div>

      <!-- 标签：多选，点一次切换一次，全部选中态用实心胶囊 -->
      <div class="space-y-1.5">
        <span class="field-label">标签<span v-if="draft.tags.length" class="ml-1 font-normal text-slate-500 dark:text-slate-400">已选 {{ draft.tags.length }}</span></span>
        <div class="flex max-h-40 flex-wrap gap-1.5 overflow-y-auto" role="group" aria-label="标签">
          <button
            type="button" class="filter-chip" :class="draft.tags.includes(0) ? 'filter-chip-active' : ''"
            :aria-pressed="draft.tags.includes(0)" @click="toggleTag(0)"
          >默认</button>
          <button
            v-for="tag in tags" :key="tag.id" type="button" class="filter-chip"
            :class="draft.tags.includes(tag.id) ? 'filter-chip-active' : ''"
            :aria-pressed="draft.tags.includes(tag.id)" @click="toggleTag(tag.id)"
          >{{ tag.name }}</button>
        </div>
      </div>
    </form>

    <template #footer>
      <!-- mr-auto 把「清除」推到弹窗左侧，与右侧的应用/取消分开，避免误触 -->
      <button type="button" class="soft-button mr-auto" :disabled="!draftActiveCount" @click="clear">清除筛选</button>
      <button type="button" class="soft-button" @click="close">取消</button>
      <button type="submit" form="gallery-filter-form" class="primary-button">应用<span v-if="draftActiveCount">{{ draftActiveCount }}</span></button>
    </template>
  </DialogShell>
</template>

<script setup>
/**
 * 图库筛选弹窗。
 *
 * 为什么用「草稿 + 应用」而不是边改边生效：
 * 每次改筛选都会触发路由 replace 和整页重新加载，在弹窗里连续点几个标签就会
 * 连打若干次请求，列表在弹窗后面反复闪。草稿态只在点「应用」时提交一次。
 * 关闭（取消 / Esc / 点遮罩）直接丢弃草稿，不会写入任何参数。
 */
import { computed, reactive, ref } from 'vue';
import DialogShell from '@/components/DialogShell.vue';

const props = defineProps({
  // 角色选项由父级传入，避免两处各写一份标签文案
  roles: { type: Array, default: () => [] },
  role: { type: String, default: 'all' },
  buckets: { type: Array, default: () => [] },
  bucket: { type: String, default: '' },
  deletion: { type: String, default: 'all' },
  tags: { type: Array, default: () => [] },
  selectedTags: { type: Array, default: () => [] },
  foldersAuthenticated: { type: Boolean, default: false },
  folders: { type: Array, default: () => [] },
  foldersLoaded: { type: Boolean, default: false },
  folder: { type: String, default: '0' },
  unfiledCount: { type: Number, default: 0 },
  folderTotalImages: { type: Number, default: 0 },
  folderLabel: { type: Function, default: folder => folder.name },
});

const emit = defineEmits(['apply', 'clear', 'close']);

const shellRef = ref(null);

// 每次打开都是从父级当前状态复制一份，取消后不会残留上次的改动
const draft = reactive({
  role: props.role,
  bucket: props.bucket,
  deletion: props.deletion,
  tags: [...props.selectedTags],
  folder: props.folder,
});

// 与工具栏徽标同口径：文件夹和搜索不算「筛选」，它们各自有独立入口
const draftActiveCount = computed(() =>
  Number(!!draft.bucket) + Number(draft.deletion !== 'all') + draft.tags.length +
  Number(draft.role !== 'all'));

function toggleTag(tagId) {
  const index = draft.tags.indexOf(tagId);
  if (index >= 0) draft.tags.splice(index, 1);
  else draft.tags.push(tagId);
}

const close = () => shellRef.value?.requestClose();

function apply() {
  emit('apply', {
    role: draft.role,
    bucket: draft.bucket,
    deletion: draft.deletion,
    tags: [...draft.tags],
    folder: draft.folder,
  });
  close();
}

function clear() {
  emit('clear');
  close();
}

// 动画播完后才通知父级销毁，提前 v-if=false 会让内容先消失、弹窗留个空壳
const onShellClosed = () => emit('close');
</script>
