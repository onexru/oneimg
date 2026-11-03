<template>
  <!-- 瀑布流：按每列累计高度把卡片放进当前最短的列（Pinterest 式贪心）。
       列数由 CSS 决定视觉宽度，JS 只负责分配，二者必须同步维护（见 script 里的断点）。 -->
  <div ref="rootEl" class="masonry" :style="{ gap: gapPx, '--masonry-cols': columnCount }">
    <div v-for="(column, index) in columns" :key="index" class="masonry-column" :style="{ gap: gapPx }">
      <slot v-for="item in column" :key="item.id" :item="item" />
    </div>
  </div>
</template>

<script setup>
/**
 * JS 瀑布流容器（移植自 onewallpaper 的 Masonry 组件，改用 ResizeObserver 适配容器宽度）。
 *
 * 为什么用相对高度而不是真实像素：
 * 卡片高度由 `aspect-ratio` 按图片自身宽高比撑开，同列等宽时「高/宽」与实际渲染高度
 * 成正比。所以分配阶段完全不必等图片加载完，也不会出现先渲染成方块再跳动的抖动。
 * 底部信息条的固定高度对每张卡是相同偏移，不改变「哪列最短」的判定。
 *
 * 增量 vs 全量：
 *   - 整组替换（翻页 / 换筛选 / 删除后重排）→ 清空后全量重新分配
 *   - 懒加载追加（「显示更多」追加新卡片）→ 只把新增项放进最短列，已有卡片不移动
 *     否则每次追加都会让整屏卡片重排，视觉上像闪了一下
 */
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue';

const props = defineProps({
  items: { type: Array, default: () => [] },
  // [移动, 平板, 桌面, 大屏]，断点见 resolveColumns
  cols: { type: Array, default: () => [2, 3, 4, 5] },
  // Tailwind 间距单位，3 = 0.75rem
  gap: { type: Number, default: 3 },
});

const gapPx = `${props.gap * 4}px`;
const rootEl = ref(null);
const columnCount = ref(props.cols[0]);
const columns = ref([]);      // [[item, ...], ...]
const heights = ref([]);      // 每列累计相对高度
const assigned = new Set();   // 已分配过的 id，用于区分「追加」和「替换」

// 断点与页面里 .gallery-grid-layout 保持一致（640 / 1280 / 1536），
// 这样同一个窗口宽度下切换「网格 / 瀑布流」时列数不会突变；
// 瀑布流卡片更小（无独立信息行），因此各档多给一列。
function resolveColumns(width) {
  const [sm = 2, md = 3, lg = 4, xl = 5] = props.cols;
  if (width < 640) return sm;
  if (width < 1280) return md;
  if (width < 1536) return lg;
  return xl;
}

// 相对高度：h/w。缺宽高数据时回退 3/4，避免整列被判成 0 高度而全挤在首列。
function relativeHeight(item) {
  const width = Number(item?.width);
  const height = Number(item?.height);
  if (width > 0 && height > 0) return height / width;
  return 0.75;
}

function shortestColumn() {
  let index = 0;
  let min = Infinity;
  heights.value.forEach((value, i) => {
    if (value < min) { min = value; index = i; }
  });
  return index;
}

function initColumns() {
  columns.value = Array.from({ length: columnCount.value }, () => []);
  heights.value = Array.from({ length: columnCount.value }, () => 0);
}

function push(item) {
  const index = shortestColumn();
  columns.value[index].push(item);
  heights.value[index] += relativeHeight(item);
}

function resetAll(list) {
  assigned.clear();
  initColumns();
  list.forEach(item => {
    assigned.add(item.id);
    push(item);
  });
}

watch(
  () => props.items,
  list => {
    if (!list.length) {
      assigned.clear();
      initColumns();
      return;
    }
    const fresh = list.filter(item => !assigned.has(item.id));
    // 新增项覆盖了整组 → 说明是替换而不是追加
    if (fresh.length >= list.length) resetAll(list);
    else fresh.forEach(item => { assigned.add(item.id); push(item); });
  },
  { immediate: true },
);

// 用容器自身宽度而不是 window.innerWidth：页面有侧边导航时两者不等，
// 按窗口宽度算列数会多算一列，卡片被压窄后窄到放不下内容。
let resizeObserver;
function applyColumnCount() {
  const width = rootEl.value?.clientWidth || window.innerWidth;
  const next = resolveColumns(width);
  if (next === columnCount.value && columns.value.length) return;
  columnCount.value = next;
  resetAll(props.items);
}

onMounted(() => {
  applyColumnCount();
  if (typeof ResizeObserver === 'undefined') return;
  resizeObserver = new ResizeObserver(applyColumnCount);
  if (rootEl.value) resizeObserver.observe(rootEl.value);
});

onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  resizeObserver = undefined;
});

const columnCountLabel = computed(() => columns.value.length);
defineExpose({ columnCount: columnCountLabel });
</script>

<style scoped>
.masonry {
  display: grid;
  /* 列数由 JS 按容器宽度算好后通过 --masonry-cols 下发；
     两侧必须一致，否则会出现「JS 认为是 3 列、CSS 只画 2 列」的错位。 */
  grid-template-columns: repeat(var(--masonry-cols, 2), minmax(0, 1fr));
  align-items: start;
}
.masonry-column {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
</style>
