<template>
  <!-- 瀑布流卡片：只有图，信息与操作浮在图上。
       和列表/网格行的区别：没有独立的文件名行，卡片高度完全由图片宽高比决定，
       这样瀑布流各列才有高低错落的效果。 -->
  <article class="masonry-card" :class="{ 'masonry-card-selected': isImageSelected(image.id) }" :data-image-id="image.id">
    <button
      type="button"
      class="masonry-media"
      :style="{ aspectRatio: ratio }"
      :disabled="image.deleting"
      :aria-label="`预览图片 ${image.filename}`"
      @click="openPreview(image)"
    >
      <!-- 占位转圈用组件内状态控制：不能靠父级加 .hidden，因为
           .masonry-loading 自带 display:grid，scoped 特异性高过单类 .hidden，
           加了也不会隐藏（这正是「图片已加载还转圈」的原因）。 -->
      <span v-if="!image.deleting && !loaded" class="masonry-loading"><i class="ri-loader-4-line animate-spin" aria-hidden="true"></i></span>
      <i v-else-if="image.deleting" class="ri-image-off-line masonry-deleted" aria-hidden="true"></i>
      <img
        v-if="!image.deleting"
        :src="image.thumbnail || image.url"
        :alt="image.filename"
        class="masonry-image"
        :class="{ 'is-loaded': loaded }"
        loading="lazy" decoding="async"
        @load="onLoad" @error="onError"
      />

      <!-- 选中不依赖 hover：移动端没有 hover，必须常驻 -->
      <label class="masonry-checkbox" :for="`masonry-image-${image.id}`" @click.stop>
        <input
          :id="`masonry-image-${image.id}`" type="checkbox" class="app-checkbox"
          :aria-label="`选择图片 ${image.filename}`"
          :checked="isImageSelected(image.id)"
          @change="(e) => handleImageSelection(image.id, e.target.checked)"
          @click.stop
        />
      </label>

      <span v-if="image.deleting" class="masonry-flag">删除未完成</span>

      <!-- 悬浮信息与操作：桌面 hover / 键盘 focus 出现；移动端由下方固定信息条兜底 -->
      <div class="masonry-overlay">
        <p class="masonry-name" :title="image.filename">{{ image.filename }}</p>
        <div class="masonry-meta">
          <span>{{ formatFileSize(image.file_size) }}</span>
          <span aria-hidden="true">·</span>
          <span>{{ image.width }}×{{ image.height }}</span>
        </div>
        <div class="masonry-actions">
          <button type="button" class="masonry-action" :aria-label="`查看图片详情 ${image.filename}`" title="详情" @click.stop="openDetails(image)">
            <i class="ri-more-2-fill" aria-hidden="true"></i>
          </button>
          <button type="button" class="masonry-action" :aria-label="`预览图片 ${image.filename}`" title="预览" @click.stop="openPreview(image)">
            <i class="ri-eye-line" aria-hidden="true"></i>
          </button>
        </div>
      </div>
    </button>

    <!-- 移动端没有 hover，把这行常驻显示，否则拿不到文件名与操作入口 -->
    <div class="masonry-footer">
      <span class="masonry-footer-name" :title="image.filename">{{ image.filename }}</span>
      <button type="button" class="masonry-action" :aria-label="`查看图片详情 ${image.filename}`" @click.stop="openDetails(image)">
        <i class="ri-more-2-fill" aria-hidden="true"></i>
      </button>
    </div>
  </article>
</template>

<script setup>
import { computed, ref, watch } from 'vue';

const props = defineProps(['image', 'version', 'isImageSelected', 'handleImageSelection', 'openPreview', 'handleImageLoad', 'handleImageError', 'formatFileSize', 'openDetails']);

// 缺宽高数据时回退 3/4，避免卡片塌成一条线；有数据就按真实比例撑高
const ratio = computed(() => {
  const width = Number(props.image?.width);
  const height = Number(props.image?.height);
  return width > 0 && height > 0 ? `${width} / ${height}` : '3 / 4';
});

const loaded = ref(false);
// 同一张卡被复用（列表刷新 / 换图）时要重置加载态，否则新图会直接显示成已完成
watch(() => props.image?.id, () => { loaded.value = false; });

const onLoad = event => {
  loaded.value = true;
  props.handleImageLoad(event);
};

const onError = event => {
  loaded.value = true;   // 失败也要收掉转圈，改由父级的占位图接管
  props.handleImageError(event);
};
</script>

<style scoped>
.masonry-card {
  position: relative;
  min-width: 0;
  overflow: hidden;
  border: 1px solid rgb(226 232 240 / 0.8);
  border-radius: var(--radius-md);
  background: #fff;
}
.masonry-card-selected { border-color: #2563eb; box-shadow: 0 0 0 2px rgb(37 99 235 / 0.25); }

.masonry-media {
  position: relative;
  display: block;
  width: 100%;
  overflow: hidden;
  background: rgb(148 163 184 / 0.12);
  cursor: zoom-in;
}
.masonry-media:disabled { cursor: not-allowed; }

.masonry-image {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  opacity: 0;
  transition: opacity 0.3s ease;
}
/* 加载完成由组件内 loaded 状态驱动，不再依赖父级改 DOM 类 */
.masonry-image.is-loaded { opacity: 1; }

/* 用 display:flex 而不是 grid：即便外部（或未来的父级）给它加 .hidden，
   单类 .hidden 的特异性也不足以覆盖 scoped 规则 —— 这里显式处理见下 */
.masonry-loading { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; color: #94a3b8; }
.masonry-loading.hidden { display: none; }
.masonry-deleted { display: grid; place-items: center; height: 100%; min-height: 5rem; color: #94a3b8; font-size: 1.6rem; }

.masonry-checkbox {
  position: absolute;
  top: 0.5rem;
  left: 0.5rem;
  z-index: 2;
  display: grid;
  place-items: center;
  width: 1.75rem;
  height: 1.75rem;
  border-radius: 0.4rem;
  background: rgb(255 255 255 / 0.88);
  backdrop-filter: blur(2px);
}

.masonry-flag {
  position: absolute;
  top: 0.5rem;
  right: 0.5rem;
  z-index: 2;
  border-radius: 999px;
  background: rgb(245 158 11 / 0.92);
  padding: 0.1rem 0.5rem;
  font-size: 0.625rem;
  color: #fff;
}

.masonry-overlay {
  position: absolute;
  inset: auto 0 0 0;
  z-index: 1;
  display: none;
  padding: 1.6rem 0.7rem 0.6rem;
  background: linear-gradient(to top, rgb(0 0 0 / 0.72), rgb(0 0 0 / 0.05) 70%, transparent);
  opacity: 0;
  transition: opacity 0.2s ease;
}
.masonry-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.75rem;
  font-weight: 500;
  color: #fff;
  text-align: left;
}
.masonry-meta { display: flex; gap: 0.3rem; margin-top: 0.15rem; font-size: 0.6875rem; color: rgb(255 255 255 / 0.8); }
.masonry-actions { position: absolute; right: 0.5rem; bottom: 0.5rem; display: flex; gap: 0.25rem; }
.masonry-action {
  display: grid;
  place-items: center;
  width: 1.9rem;
  height: 1.9rem;
  border-radius: 0.4rem;
  background: rgb(255 255 255 / 0.9);
  color: #334155;
  transition: background 0.15s ease, color 0.15s ease;
}
.masonry-action:hover { background: #fff; color: #0f172a; }

.masonry-footer {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.5rem;
  border-top: 1px solid rgb(226 232 240 / 0.7);
}
.masonry-footer-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.75rem;
  color: #475569;
}

button:focus-visible { outline:none; box-shadow:0 0 0 2px rgb(37 99 235 / .45); }

/* 有 hover 能力的设备才用悬浮层，并隐藏占位的底部信息条（避免重复显示文件名） */
@media (hover: hover) and (pointer: fine) {
  .masonry-overlay { display: block; }
  .masonry-media:hover .masonry-overlay,
  .masonry-media:focus-visible .masonry-overlay { opacity: 1; }
  .masonry-footer { display: none; }
}

:global(.dark) .masonry-card { border-color: rgb(255 255 255 / 0.1); background: #0f172a; }
:global(.dark) .masonry-footer { border-color: rgb(255 255 255 / 0.1); }
:global(.dark) .masonry-footer-name { color: #94a3b8; }
:global(.dark) .masonry-checkbox { background: rgb(15 23 42 / 0.8); }
:global(.dark) .masonry-action { background: rgb(15 23 42 / 0.85); color: #e2e8f0; }
</style>
