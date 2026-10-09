<template>
<section class="section-card mt-6" aria-label="登录会话" :aria-busy="busy">
  <h2 class="section-title">登录会话</h2>
  <p class="text-sm text-secondary">最近 100 条未过期会话，每页 10 条；不包含已结束登录历史。</p>
  <div class="flex flex-wrap gap-2 my-3">
    <button type="button" class="soft-button" :disabled="busy" @click="load(true)">刷新会话</button>
    <button type="button" class="soft-button" :disabled="busy" @click="revokeOthers">退出其他会话</button>
    <button type="button" class="danger-button" :disabled="busy" @click="revokeAll">退出全部会话</button>
  </div>
  <p v-if="error" role="alert" class="mb-3 text-sm text-red-600 dark:text-red-400">{{ error }}</p>
  <p v-if="busy && !rows.length" role="status" class="text-sm text-secondary">正在加载会话…</p>
  <p v-else-if="!rows.length && !error" role="status" class="text-sm text-secondary">暂无未过期会话。</p>
  <template v-if="rows.length">
    <p class="mb-2 text-xs text-secondary" aria-live="polite">第 {{ rangeStart }}–{{ rangeEnd }} 条，共 {{ rows.length }} 条</p>
    <ul ref="listElement" class="max-h-[32rem] overflow-y-auto overscroll-contain space-y-2 pr-1" aria-label="当前页登录会话">
      <li v-for="row in pageData.items" :key="row.id" class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-slate-200 p-3 dark:border-white/10">
        <div class="min-w-0 text-sm">
          <strong>{{ row.current ? '当前会话' : '其他会话' }}</strong>
          <p class="break-words text-secondary">创建：{{ format(row.created_at) }}；到期：{{ format(row.expires_at) }}</p>
        </div>
        <button v-if="!row.current" type="button" class="danger-button" :disabled="busy" @click="revoke(row.id)">撤销会话</button>
      </li>
    </ul>
    <nav v-if="pageData.pages > 1" class="mt-3 flex flex-wrap items-center justify-between gap-2" aria-label="会话分页">
      <span class="text-sm text-secondary">第 {{ pageData.page }} / {{ pageData.pages }} 页</span>
      <div class="flex gap-2">
        <button type="button" class="soft-button" :disabled="busy || pageData.page === 1" @click="changePage(pageData.page - 1)">上一页</button>
        <button type="button" class="soft-button" :disabled="busy || pageData.page === pageData.pages" @click="changePage(pageData.page + 1)">下一页</button>
      </div>
    </nav>
  </template>
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
let request;
let disposed = false;

const format = value => {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '时间不可用' : date.toLocaleString();
};

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
