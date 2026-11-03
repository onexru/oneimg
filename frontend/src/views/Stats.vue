<template>
  <div class="page-shell">
    <section class="page-header">
      <div class="page-actions">
        <button type="button" class="soft-button" :disabled="loading" @click="loadStats">
          <i :class="loading ? 'ri-loader-4-line animate-spin' : 'ri-refresh-line'" aria-hidden="true"></i>
          <span>刷新</span>
        </button>
      </div>
    </section>

    <!-- 加载态 -->
    <div v-if="loading" class="surface flex flex-col items-center justify-center py-20" role="status">
      <i class="ri-loader-4-line animate-spin text-3xl text-primary" aria-hidden="true"></i>
      <p class="mt-3 text-sm text-slate-500 dark:text-slate-400">正在统计…</p>
    </div>

    <template v-else>
      <!-- 关键指标：四个数字一行铺开，每个都带一句口径说明，
           避免「本月上传 0」这种数字被误读成「系统坏了」 -->
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div v-for="metric in metrics" :key="metric.key" class="surface flex flex-col gap-1.5">
          <div class="flex items-center gap-1.5">
            <i :class="[metric.icon, 'text-sm text-slate-400']" aria-hidden="true"></i>
            <span class="stat-label">{{ metric.label }}</span>
          </div>
          <p class="text-2xl font-semibold leading-tight text-slate-900 tabular-nums dark:text-slate-100">
            {{ metric.value }}<span v-if="metric.unit" class="ml-1 text-xs font-normal text-slate-400">{{ metric.unit }}</span>
          </p>
          <p class="text-[11px] text-slate-400 dark:text-slate-500">{{ metric.hint }}</p>
        </div>
      </div>

      <div class="grid gap-4 lg:grid-cols-[minmax(0,1.45fr)_minmax(0,1fr)]">
        <!-- 上传趋势：7 天柱状图。纯 CSS 实现，不引图表库 -->
        <div class="surface">
          <div class="mb-4 flex flex-wrap items-baseline justify-between gap-2">
            <h2 class="surface-title">
              <i class="ri-bar-chart-2-line text-primary" aria-hidden="true"></i>
              近 7 天上传
            </h2>
            <p class="text-xs text-slate-400 dark:text-slate-500">
              合计 <strong class="font-semibold text-slate-600 dark:text-slate-300">{{ trendTotal }}</strong> 张
            </p>
          </div>

          <div class="flex items-stretch gap-2" role="img" :aria-label="`近 7 天共上传 ${trendTotal} 张`">
            <div
              v-for="day in trend"
              :key="day.date"
              class="group flex min-w-0 flex-1 flex-col items-center gap-1.5"
              :title="`${day.label}：${day.count} 张`"
            >
              <span class="text-[11px] font-medium tabular-nums text-slate-500 dark:text-slate-400">{{ day.count }}</span>
              <!-- 柱子轨道固定 120px 高：百分比高度必须有确定高度的父级，
                   否则 height:2% 会按内容高度(约 45px)算，直接变成 0px。 -->
              <div class="flex h-[120px] w-full items-end">
                <div
                  class="w-full rounded-t-[4px] transition-colors"
                  :class="day.count > 0 ? 'bg-primary/70 group-hover:bg-primary' : 'bg-slate-100 dark:bg-white/5'"
                  :style="{ height: `${day.percent}%` }"
                ></div>
              </div>
              <span class="w-full truncate text-center text-[11px] text-slate-400 dark:text-slate-500">{{ day.short }}</span>
            </div>
          </div>

          <!-- 全为 0 时明确说明，而不是让一排空柱子显得像渲染失败 -->
          <p v-if="trendTotal === 0" class="mt-3 rounded-[var(--radius-sm)] bg-slate-50 px-3 py-2 text-xs text-slate-500 dark:bg-slate-950 dark:text-slate-400">
            最近 7 天没有上传记录。
          </p>
        </div>

        <!-- 格式分布 -->
        <div class="surface">
          <h2 class="surface-title mb-4">
            <i class="ri-file-list-3-line text-primary" aria-hidden="true"></i>
            格式分布
          </h2>
          <div v-if="formats.length" class="space-y-3">
            <div v-for="item in formats" :key="item.format">
              <div class="mb-1 flex items-baseline justify-between gap-2 text-xs">
                <span class="min-w-0 truncate font-medium text-slate-700 dark:text-slate-200">{{ item.label }}</span>
                <span class="shrink-0 tabular-nums text-slate-500 dark:text-slate-400">
                  {{ item.count }} 张 · {{ item.percent }}%
                </span>
              </div>
              <div class="h-1.5 w-full overflow-hidden rounded-full bg-slate-100 dark:bg-white/5">
                <div class="h-full rounded-full bg-primary/70" :style="{ width: `${item.percent}%` }"></div>
              </div>
            </div>
          </div>
          <p v-else class="text-sm text-slate-500 dark:text-slate-400">暂无数据。</p>
        </div>
      </div>

      <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.45fr)]">
        <!-- 大小分布 -->
        <div class="surface">
          <h2 class="surface-title mb-4">
            <i class="ri-scales-3-line text-primary" aria-hidden="true"></i>
            文件大小分布
          </h2>
          <div v-if="sizes.length" class="space-y-3">
            <div v-for="item in sizes" :key="item.range">
              <div class="mb-1 flex items-baseline justify-between gap-2 text-xs">
                <span class="min-w-0 truncate font-medium text-slate-700 dark:text-slate-200">{{ item.range }}</span>
                <span class="shrink-0 tabular-nums text-slate-500 dark:text-slate-400">{{ item.count }}</span>
              </div>
              <!-- 用面积感的横条：宽度按「占最大档的比例」，能看出分布重心 -->
              <div class="h-1.5 w-full overflow-hidden rounded-full bg-slate-100 dark:bg-white/5">
                <div class="h-full rounded-full bg-emerald-500/70" :style="{ width: `${item.percent}%` }"></div>
              </div>
            </div>
          </div>
          <p v-else class="text-sm text-slate-500 dark:text-slate-400">暂无数据。</p>
        </div>

        <!-- 最近上传 -->
        <div class="surface">
          <div class="mb-3 flex items-center justify-between gap-2">
            <h2 class="surface-title">
              <i class="ri-history-line text-primary" aria-hidden="true"></i>
              最近上传
            </h2>
            <router-link to="/gallery" class="text-xs text-slate-500 transition-colors hover:text-primary dark:text-slate-400">
              全部图库<i class="ri-arrow-right-s-line ml-0.5" aria-hidden="true"></i>
            </router-link>
          </div>
          <ul v-if="recent.length" class="space-y-1.5">
            <li
              v-for="image in recent"
              :key="image.id"
              class="flex items-center gap-2.5 rounded-[var(--radius-sm)] px-1.5 py-1.5 transition-colors hover:bg-slate-50 dark:hover:bg-white/5"
            >
              <!-- 缩略图采用「占位层常驻 + 图片淡入」：
                   占位图标始终铺在底层，<img> 叠在上面且默认透明，只有 onload 成功才淡入。
                   这样无论是慢加载还是 404，都不会出现浏览器默认的破损图标；
                   之前 visibility:hidden 的做法会留下一个带边框的空盒子，同样难看。 -->
              <div class="relative h-9 w-9 shrink-0">
                <span
                  class="absolute inset-0 flex items-center justify-center rounded-[6px] border border-dashed border-slate-200 bg-slate-50 text-slate-300 dark:border-white/10 dark:bg-slate-950 dark:text-slate-600"
                  :title="brokenThumbs[image.id] === 'broken' ? '缩略图不可用' : undefined"
                >
                  <i class="ri-image-off-line text-sm" aria-hidden="true"></i>
                </span>
                <img
                  v-if="brokenThumbs[image.id] !== 'broken'"
                  :src="brokenThumbs[image.id] === 'fallback' ? image.url : (image.thumbnail || image.url)"
                  :alt="image.filename"
                  loading="lazy" decoding="async"
                  class="absolute inset-0 h-9 w-9 rounded-[6px] border border-slate-200 object-cover transition-opacity duration-200 dark:border-white/10"
                  :class="loadedThumbs[image.id] ? 'opacity-100' : 'opacity-0'"
                  @error="onThumbError(image)"
                  @load="onThumbLoad(image)"
                />
              </div>
              <div class="min-w-0 flex-1">
                <p class="truncate text-[13px] font-medium text-slate-800 dark:text-slate-100" :title="image.filename">
                  {{ image.filename }}
                </p>
                <p class="text-[11px] text-slate-400 dark:text-slate-500">
                  {{ formatFileSize(image.file_size) }}<template v-if="image.width && image.height"> · {{ image.width }}×{{ image.height }}</template>
                </p>
              </div>
              <span class="shrink-0 text-[11px] tabular-nums text-slate-400 dark:text-slate-500">{{ shortDate(image.created_at) }}</span>
            </li>
          </ul>
          <p v-else class="text-sm text-slate-500 dark:text-slate-400">还没有上传过图片。</p>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue';
import Message from '@/utils/message.js';

const loading = ref(true);
const stats = ref({
  total_images: 0,
  total_size: 0,
  today_uploads: 0,
  month_uploads: 0,
  recent_images: [],
  upload_trend: [],
  format_stats: [],
  size_distribution: [],
});

const formatNumber = (value) => (value ? Number(value).toLocaleString('zh-CN') : '0');

const formatFileSize = (bytes) => {
  const n = Number(bytes);
  if (!n) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.min(Math.floor(Math.log(n) / Math.log(1024)), units.length - 1);
  return `${parseFloat((n / 1024 ** i).toFixed(2))} ${units[i]}`;
};

// 指标卡：把「今天/本月」这种容易误读的 0 也用一句话说明口径
const metrics = computed(() => [
  {
    key: 'total',
    label: '总图片',
    icon: 'ri-image-2-line',
    value: formatNumber(stats.value.total_images),
    unit: '张',
    hint: '当前账号可见的全部图片',
  },
  {
    key: 'size',
    label: '总占用',
    icon: 'ri-hard-drive-3-line',
    value: formatFileSize(stats.value.total_size),
    unit: '',
    hint: '图片文件原始体积之和',
  },
  {
    key: 'month',
    label: '本月上传',
    icon: 'ri-calendar-line',
    value: formatNumber(stats.value.month_uploads),
    unit: '张',
    hint: '按自然月统计',
  },
  {
    key: 'today',
    label: '今日上传',
    icon: 'ri-sun-line',
    value: formatNumber(stats.value.today_uploads),
    unit: '张',
    hint: '按服务器本地时区',
  },
]);

// 趋势：高度按当周最大值归一化，全 0 时统一给一个最小可见高度
const trend = computed(() => {
  const list = stats.value.upload_trend || [];
  const counts = list.map(d => Number(d.count) || 0);
  const max = Math.max(...counts, 0);
  return list.map((day, index) => {
    const count = counts[index];
    return {
      date: day.date,
      count,
      // 有数据的柱子至少 10% 高（1 张与最大值差距再大也看得见）；
      // 全 0 时给 3% 的浅色基线，表示「这天有位置但没数据」
      percent: max > 0 ? Math.max(count > 0 ? 10 : 3, Math.round((count / max) * 100)) : 3,
      label: day.date,
      short: day.date ? day.date.slice(5) : '',
    };
  });
});

const trendTotal = computed(() => trend.value.reduce((sum, day) => sum + day.count, 0));

const formats = computed(() => {
  const list = stats.value.format_stats || [];
  const total = list.reduce((sum, item) => sum + (Number(item.count) || 0), 0);
  return list
    .filter(item => Number(item.count) > 0)
    .sort((a, b) => Number(b.count) - Number(a.count))
    .map(item => ({
      format: item.format,
      label: (item.format || '').replace('image/', '').toUpperCase() || '未知',
      count: formatNumber(item.count),
      percent: total > 0 ? Math.round((Number(item.count) / total) * 100) : 0,
      size: item.size,
    }));
});

const sizes = computed(() => {
  const list = stats.value.size_distribution || [];
  // 横条宽度按「本档 / 最大档」算，而不是「本档 / 总数」：
  // 后者在头重脚轻的分布里，后面的档位会细到看不见，失去比较意义
  const max = Math.max(...list.map(item => Number(item.count) || 0), 0);
  return list.map(item => {
    const count = Number(item.count) || 0;
    return {
      range: item.range,
      count: formatNumber(count),
      percent: max > 0 ? Math.max(count > 0 ? 4 : 0, Math.round((count / max) * 100)) : 0,
    };
  });
});

const recent = computed(() => (stats.value.recent_images || []).slice(0, 8));

const shortDate = (value) => {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  const now = new Date();
  const sameYear = date.getFullYear() === now.getFullYear();
  return date.toLocaleDateString('zh-CN', sameYear
    ? { month: '2-digit', day: '2-digit' }
    : { year: '2-digit', month: '2-digit', day: '2-digit' });
};

// 缩略图加载失败记录：按图片 id 记，避免同一张图反复重试。
const brokenThumbs = ref({});
// 加载成功的图片 id，用来控制淡入（占位层在下面，成功后才盖上去）
const loadedThumbs = ref({});

// 缩略图可能因存储源不可用而失败（例如 R2 副本失效）。
// 先退回原图；原图也不可达时标记为「不可用」，由模板换成占位图标。
// 用 id 作键而不是直接改 DOM：Vue 重新渲染（刷新统计）时要能重置状态。
const onThumbError = (image) => {
  // 已经退回过一次原图仍然失败 → 认定为不可用
  if (brokenThumbs.value[image.id] === 'fallback') {
    brokenThumbs.value = { ...brokenThumbs.value, [image.id]: 'broken' };
    return;
  }
  // 首次失败：若原图地址与当前不同，尝试换用原图
  if (image.url && image.url !== image.thumbnail) {
    brokenThumbs.value = { ...brokenThumbs.value, [image.id]: 'fallback' };
    return;
  }
  brokenThumbs.value = { ...brokenThumbs.value, [image.id]: 'broken' };
};

// 图片最终加载成功时清除失败记录（首次失败后换原图成功的情况），并标记为已加载。
// 没有这一步的话，条目会永久停在 fallback 状态，
// 同一张图第二次 error 就会被误判成 broken。
const onThumbLoad = (image) => {
  loadedThumbs.value = { ...loadedThumbs.value, [image.id]: true };
  if (!brokenThumbs.value[image.id]) return;
  const next = { ...brokenThumbs.value };
  delete next[image.id];
  brokenThumbs.value = next;
};

const loadStats = async () => {
  loading.value = true;
  // 重新拉取时清空失败记录：存储恢复了的话，图片应该能重新显示出来
  brokenThumbs.value = {};
  loadedThumbs.value = {};
  try {
    const response = await fetch('/api/stats/dashboard', {
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
    });
    if (!response.ok) {
      if (response.status === 401) {
        Message.error('登录已过期，请重新登录');
        return;
      }
      throw new Error('加载统计数据失败');
    }
    const result = await response.json();
    stats.value = { ...stats.value, ...(result.data || {}) };
  } catch (error) {
    console.error('加载统计数据错误:', error);
    Message.error(error.message || '加载统计数据失败');
  } finally {
    loading.value = false;
  }
};

onMounted(loadStats);
</script>
