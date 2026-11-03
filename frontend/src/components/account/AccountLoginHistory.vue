<template>
  <!-- h-full + flex 列：和「登录会话」并排时，两栏高度由网格拉平，
       列表用 flex-1 吃掉剩余空间，内容自然铺满卡片，而不是顶部挤一堆、底部留白。 -->
  <section class="surface flex h-full min-h-0 flex-col" aria-labelledby="login-history-title" :aria-busy="busy">
    <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h2 id="login-history-title" class="surface-title">
          <i class="ri-history-line text-primary" aria-hidden="true"></i>
          登录历史
          <span v-if="rows.length" class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-normal text-slate-500 dark:bg-slate-800 dark:text-slate-400">{{ rows.length }}</span>
        </h2>
        <p class="surface-subtitle">最近 30 天内的成功登录，最多 100 条。会话续期不会新增记录。</p>
      </div>
      <button type="button" class="soft-button shrink-0" :disabled="busy" @click="load">
        <i :class="busy ? 'ri-loader-4-line animate-spin' : 'ri-refresh-line'" aria-hidden="true"></i>
        {{ busy ? '加载中…' : '刷新' }}
      </button>
    </div>

    <p v-if="error" role="alert" class="flex items-start gap-2 rounded-[var(--radius-sm)] border border-red-200 bg-red-50 px-3 py-2.5 text-sm text-red-700 dark:border-red-400/20 dark:bg-red-400/10 dark:text-red-300">
      <i class="ri-error-warning-line mt-0.5 shrink-0" aria-hidden="true"></i>
      <span>{{ error }}</span>
    </p>
    <p v-else-if="busy && !rows.length" role="status" class="flex items-center gap-2 py-6 text-sm text-slate-500 dark:text-slate-400">
      <i class="ri-loader-4-line animate-spin" aria-hidden="true"></i>正在加载登录历史…
    </p>
    <p v-else-if="!rows.length" role="status" class="rounded-[var(--radius-sm)] border border-dashed border-slate-200 px-4 py-6 text-center text-sm text-slate-500 dark:border-white/10 dark:text-slate-400">
      最近 30 天暂无登录记录。
    </p>

    <!-- 时间线：
         竖线画在 li 的伪元素上（::before），而不是给每个 li 加 border-left ——
         用 border 时圆点要手动 -5px 对齐、且每行之间有断口；
         伪元素可以从上到下连成一条完整竖线，最后一项截断到圆点处。
         列表 flex-1 + min-h-0 才能让 max-h 生效的同时撑满卡片高度。 -->
    <!-- 窄屏（堆叠）时给列表一个固定 max-h，避免 100 条把页面撑到几千像素；
         xl 起改为 flex-1 吃满卡片高度（此时卡片高度由外层 grid 决定）。
         两者不能同时用：光有 flex-1 而卡片没有确定高度时，列表会无限展开。 -->
    <ol v-else class="timeline max-h-80 overflow-y-auto overscroll-contain pr-1 xl:max-h-none xl:min-h-0 xl:flex-1" aria-label="最近成功登录">
      <li
        v-for="(row, index) in decorated"
        :key="`${row.created_at}-${index}`"
        class="timeline-item"
        :class="{ 'timeline-item-current': row.current, 'timeline-item-last': index === decorated.length - 1 }"
      >
        <span class="timeline-dot" aria-hidden="true"></span>
        <div class="flex min-w-0 flex-1 flex-wrap items-center justify-between gap-x-3 gap-y-1">
          <div class="flex min-w-0 flex-wrap items-center gap-2">
            <span class="text-sm font-medium text-slate-800 dark:text-slate-100">{{ row.label }}</span>
            <span
              v-if="index === 0"
              class="rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-medium text-primary"
            >最近一次</span>
          </div>
          <time :datetime="row.raw" class="shrink-0 text-xs tabular-nums text-slate-500 dark:text-slate-400">
            {{ row.timeLabel }}
          </time>
        </div>
      </li>
    </ol>
  </section>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import { readApiResponse } from '@/utils/apiFeedback.js';

const rows = ref([]);
const busy = ref(false);
const error = ref('');
let request;
let disposed = false;

const METHOD_LABELS = { password: '密码登录', oidc: 'OIDC 登录', cas: 'CAS 登录' };

const formatDateTime = value => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '时间不可用';
  return date.toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  });
};

// 预先生成展示字段，避免模板里逐行调用格式化函数
const decorated = computed(() => rows.value.map(row => ({
  raw: row.created_at,
  label: METHOD_LABELS[row.method] || '账户登录',
  timeLabel: formatDateTime(row.created_at),
  current: row.created_at === rows.value[0]?.created_at,
})));

async function load() {
  if (busy.value || disposed) return;
  busy.value = true;
  error.value = '';
  const controller = new AbortController();
  request = controller;
  try {
    const response = await readApiResponse(await fetch('/api/account/login-history', {
      credentials: 'same-origin', signal: controller.signal,
    }), '获取登录历史失败，请重试');
    if (!Array.isArray(response.data)) throw new Error('登录历史响应格式无效，请重试');
    if (!disposed) rows.value = response.data.slice(0, 100);
  } catch (failure) {
    if (!disposed && failure.name !== 'AbortError') {
      rows.value = [];
      error.value = failure.message || '获取登录历史失败，请重试';
    }
  } finally {
    if (!disposed) busy.value = false;
    if (request === controller) request = undefined;
  }
}

onMounted(load);
onBeforeUnmount(() => { disposed = true; request?.abort(); });
</script>

<style scoped>
/* ===== 时间线 =====
   竖线用 ::before 画在每一项上，首尾两项各截一半，拼起来是连续一整条；
   圆点绝对定位压在竖线上，天然居中，不用靠 -5px 之类的魔法数字对齐。 */
.timeline {
  position: relative;
  /* 与卡片内边距对齐；右侧留出滚动条空间（overflow-y-auto 时）
     25px 让圆点（li 内 left:-21px）距容器左缘 4px，不会被 overflow 裁到 */
  padding-left: 25px;
  padding-right: 4px;
}

.timeline-item {
  position: relative;
  padding: 9px 0;
}

/* 竖线：从每一项的上边缘画到下边缘，靠 -left 对齐圆点中心 */
.timeline-item::before {
  content: '';
  position: absolute;
  left: -16px;
  top: 0;
  bottom: 0;
  width: 2px;
  background: rgb(226 232 240 / 1);
  border-radius: 1px;
}

/* 首项从圆点处起画，末项画到圆点处，避免竖线探出圆点两端 */
.timeline-item:first-child::before { top: 18px; }
.timeline-item-last::before { bottom: calc(100% - 20px); }

.dark .timeline-item::before { background: rgb(255 255 255 / 0.10); }

/* 当前（最近一次）那一段竖线用主色，形成视觉起点 */
.timeline-item-current::before { background: rgb(37 99 235 / 0.35); }
.dark .timeline-item-current::before { background: rgb(96 165 250 / 0.35); }

.timeline-dot {
  position: absolute;
  left: -21px;
  top: 50%;
  width: 12px;
  height: 12px;
  margin-top: -6px;
  border-radius: 999px;
  background: #fff;
  border: 2px solid rgb(203 213 225);
  box-sizing: border-box;
}

.dark .timeline-dot {
  background: rgb(15 23 42);
  border-color: rgb(71 85 105);
}

/* 最新一条：实心主色圆点 + 外圈光晕，一眼能定位「最新」 */
.timeline-item-current .timeline-dot {
  background: #2563eb;
  border-color: #2563eb;
  box-shadow: 0 0 0 3px rgb(37 99 235 / 0.15);
}

.dark .timeline-item-current .timeline-dot {
  background: #60a5fa;
  border-color: #60a5fa;
  box-shadow: 0 0 0 3px rgb(96 165 250 / 0.18);
}

/* 相邻项之间用极淡的虚线分隔，时间线不至于粘成一片 */
.timeline-item + .timeline-item {
  border-top: 1px dashed rgb(226 232 240 / 0.7);
}

.dark .timeline-item + .timeline-item {
  border-top-color: rgb(255 255 255 / 0.06);
}
</style>
