<template>
<!-- h-full + flex 列只在 xl（并排）时启用：那时两栏高度由外层 grid 拉平，
     列表用 flex-1 吸收剩余空间，卡片底部不会留白。
     窄屏堆叠时没有「同行等高」这回事，h-full 会让两块互相牵制，故去掉。 -->
<section class="surface flex flex-col xl:h-full xl:min-h-0" aria-label="登录会话" :aria-busy="busy">
  <div class="mb-4 flex flex-wrap items-start justify-between gap-3">
    <div class="min-w-0">
      <h2 class="surface-title">
        <i class="ri-computer-line text-primary" aria-hidden="true"></i>
        登录会话
        <span class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-normal text-slate-500 dark:bg-slate-800 dark:text-slate-400">{{ rows.length }}</span>
      </h2>
      <p class="surface-subtitle">最近 100 条未过期会话，每页 10 条；不含已结束的登录历史。</p>
    </div>
    <div class="flex flex-wrap items-center gap-2">
      <button type="button" class="soft-button" :disabled="busy" @click="load(true)">
        <i class="ri-refresh-line" aria-hidden="true"></i>刷新
      </button>
      <button type="button" class="soft-button" :disabled="busy || !hasOthers" @click="revokeOthers">
        <i class="ri-logout-box-r-line" aria-hidden="true"></i>退出其他会话
      </button>
      <button type="button" class="danger-button" :disabled="busy" @click="revokeAll">
        <i class="ri-shut-down-line" aria-hidden="true"></i>退出全部
      </button>
    </div>
  </div>

  <p v-if="error" role="alert" class="mb-3 flex items-start gap-2 rounded-[var(--radius-sm)] border border-red-200 bg-red-50 px-3 py-2.5 text-sm text-red-700 dark:border-red-400/20 dark:bg-red-400/10 dark:text-red-300">
    <i class="ri-error-warning-line mt-0.5 shrink-0" aria-hidden="true"></i>
    <span>{{ error }}</span>
  </p>

  <p v-if="busy && !rows.length" role="status" class="flex items-center gap-2 py-6 text-sm text-slate-500 dark:text-slate-400">
    <i class="ri-loader-4-line animate-spin" aria-hidden="true"></i>正在加载会话…
  </p>
  <p v-else-if="!rows.length && !error" role="status" class="rounded-[var(--radius-sm)] border border-dashed border-slate-200 px-4 py-6 text-center text-sm text-slate-500 dark:border-white/10 dark:text-slate-400">
    暂无未过期会话。
  </p>

  <!-- 用 div 而不是 <template v-if>：需要一个真实盒子来承载 flex-1，
       让「列表 + 分页」一起吸收卡片剩余高度（仅 xl 并排时生效）。 -->
  <div v-if="rows.length" class="flex flex-col xl:min-h-0 xl:flex-1">
    <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
      <p class="text-xs text-slate-500 dark:text-slate-400" aria-live="polite">
        第 {{ rangeStart }}–{{ rangeEnd }} 条，共 {{ rows.length }} 条
      </p>
      <p class="text-xs text-slate-400 dark:text-slate-500">会话到期后需重新登录</p>
    </div>

    <ul ref="listElement" class="max-h-80 space-y-2 overflow-y-auto overscroll-contain pr-1 xl:max-h-none xl:min-h-0 xl:flex-1" aria-label="当前页登录会话">
      <li
        v-for="row in pagedRows"
        :key="row.id"
        class="flex flex-wrap items-center justify-between gap-3 rounded-[var(--radius-md)] border px-3 py-2.5"
        :class="row.current
          ? 'border-primary/30 bg-primary/5'
          : 'border-slate-200 dark:border-white/10'"
      >
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <span
              class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium"
              :class="row.current
                ? 'bg-primary/10 text-primary'
                : 'bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400'"
            >
              <i :class="row.current ? 'ri-checkbox-circle-line' : 'ri-computer-line'" aria-hidden="true"></i>
              {{ row.current ? '当前会话' : '其他会话' }}
            </span>
            <!-- 剩余有效期比「到期时间」更容易判断：临期会话一眼能看出来 -->
            <span
              class="text-[11px] tabular-nums"
              :class="row.expiringSoon ? 'text-amber-600 dark:text-amber-400' : 'text-slate-400 dark:text-slate-500'"
            >
              {{ row.expiresLabel }}
            </span>
          </div>
          <p class="mt-1 truncate text-xs text-slate-500 dark:text-slate-400" :title="`创建于 ${row.createdLabel}`">
            创建于 {{ row.createdLabel }}
          </p>
        </div>
        <button
          v-if="!row.current"
          type="button"
          class="soft-button shrink-0 !min-h-0 h-8 px-2.5 text-xs"
          :disabled="busy"
          @click="revoke(row.id)"
        >
          撤销
        </button>
      </li>
    </ul>

    <nav v-if="pageData.pages > 1" class="mt-3 flex flex-wrap items-center justify-between gap-2" aria-label="会话分页">
      <span class="text-xs text-slate-500 dark:text-slate-400">第 {{ pageData.page }} / {{ pageData.pages }} 页</span>
      <div class="flex gap-2">
        <button type="button" class="soft-button !min-h-0 h-8 px-2.5 text-xs" :disabled="busy || pageData.page === 1" @click="changePage(pageData.page - 1)">上一页</button>
        <button type="button" class="soft-button !min-h-0 h-8 px-2.5 text-xs" :disabled="busy || pageData.page === pageData.pages" @click="changePage(pageData.page + 1)">下一页</button>
      </div>
    </nav>
  </div>
</section>
</template>

<script setup>
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue';
import { readApiResponse } from '@/utils/apiFeedback.js';
import { selectorPage } from '@/utils/renderBounds.js';

const PAGE_SIZE = 10;
const rows = ref([]);
const page = ref(1);
const busy = ref(false);
const error = ref('');
const listElement = ref(null);
const pageData = computed(() => selectorPage(rows.value, page.value, PAGE_SIZE));
const rangeStart = computed(() => (pageData.value.page - 1) * PAGE_SIZE + 1);
const rangeEnd = computed(() => rangeStart.value + pageData.value.items.length - 1);
// 没有「其他会话」时，「退出其他会话」按钮没有意义，直接禁用
const hasOthers = computed(() => rows.value.some(row => !row.current));
let request;
let disposed = false;

const formatDateTime = value => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '时间不可用';
  return date.toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  });
};

// 把「到期时间」换算成人能直接判断的剩余时长
const remainingLabel = expiresAt => {
  const date = new Date(expiresAt);
  if (Number.isNaN(date.getTime())) return { text: '到期时间不可用', soon: false };
  const ms = date.getTime() - Date.now();
  if (ms <= 0) return { text: '已过期', soon: true };
  const hours = Math.floor(ms / 3600000);
  if (hours < 1) return { text: `剩余 ${Math.max(1, Math.floor(ms / 60000))} 分钟`, soon: true };
  if (hours < 24) return { text: `剩余 ${hours} 小时`, soon: true };
  return { text: `剩余 ${Math.floor(hours / 24)} 天`, soon: false };
};

// 视图模型：把格式化与「临期」判断放到 computed 里，
// 模板只做展示，避免在 template 里调函数反复 new Date。
const decorated = computed(() => rows.value.map(row => {
  const remain = remainingLabel(row.expires_at);
  return {
    ...row,
    createdLabel: formatDateTime(row.created_at),
    expiresLabel: remain.text,
    expiringSoon: remain.soon,
  };
}));

const pagedRows = computed(() => {
  // 直接按同一区间切片，避免用 id 建 Map 再查一遍（也避免 id 类型不一致的风险）
  const start = (pageData.value.page - 1) * PAGE_SIZE;
  return decorated.value.slice(start, start + PAGE_SIZE);
});

async function changePage(value) {
  page.value = selectorPage(rows.value, value, PAGE_SIZE).page;
  await nextTick();
  if (listElement.value) listElement.value.scrollTop = 0;
}

async function fetchRows(resetPage = false) {
  const response = await readApiResponse(await fetch('/api/account/sessions', {
    credentials: 'same-origin', signal: request.signal,
  }), '获取会话失败，请重试');
  if (!Array.isArray(response.data)) throw new Error('会话响应格式无效，请重试');
  if (disposed) return;
  rows.value = response.data.slice(0, 100);
  await changePage(resetPage ? 1 : page.value);
}

async function perform(action) {
  if (busy.value || disposed) return;
  busy.value = true;
  error.value = '';
  const controller = new AbortController();
  request = controller;
  try {
    await action(controller.signal);
  } catch (failure) {
    if (!disposed && failure.name !== 'AbortError') error.value = failure.message || '操作失败，请重试';
  } finally {
    if (!disposed) busy.value = false;
    if (request === controller) request = undefined;
  }
}

const load = (resetPage = false) => perform(() => fetchRows(resetPage));
const revoke = id => perform(async signal => {
  await readApiResponse(await fetch('/api/account/sessions/' + encodeURIComponent(id), {
    method: 'DELETE', credentials: 'same-origin', signal,
  }));
  if (!disposed) await fetchRows();
});
const revokeOthers = () => perform(async signal => {
  await readApiResponse(await fetch('/api/account/sessions/revoke', {
    method: 'POST', credentials: 'same-origin', signal,
    headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ scope: 'others' }),
  }));
  if (!disposed) await fetchRows(true);
});
function revokeAll() {
  if (busy.value || disposed || !window.confirm('确认退出本账户全部会话？当前浏览器也会退出。')) return;
  return perform(async signal => {
    await readApiResponse(await fetch('/api/account/sessions/revoke', {
      method: 'POST', credentials: 'same-origin', signal,
      headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ scope: 'all' }),
    }));
    if (disposed) return;
    localStorage.removeItem('userInfo');
    location.assign('/login');
  });
}

onMounted(() => load());
onBeforeUnmount(() => { disposed = true; request?.abort(); });
</script>
