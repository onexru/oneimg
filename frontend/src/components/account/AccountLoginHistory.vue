<template>
    <section class="section-card mt-6 p-6 md:p-8" aria-labelledby="login-history-title" :aria-busy="busy">
        <div class="flex flex-wrap items-center justify-between gap-3">
            <h2 id="login-history-title" class="section-title">登录历史</h2>
            <button type="button" class="soft-button" :disabled="busy" @click="load">
                {{ busy ? '加载中…' : '刷新历史' }}
            </button>
        </div>
        <p class="mt-2 text-sm text-secondary">仅显示最近 30 天内的成功登录，最多 100 条。会话续期不会新增记录。</p>
        <p v-if="error" role="alert" class="mt-4 text-sm text-red-700 dark:text-red-400">{{ error }}</p>
        <p v-else-if="busy" role="status" class="mt-4 text-sm text-secondary">正在加载登录历史…</p>
        <p v-else-if="!rows.length" role="status" class="mt-4 text-sm text-secondary">最近 30 天暂无登录记录。</p>
        <ol v-else class="mt-4 max-h-80 overflow-y-auto divide-y divide-slate-200 dark:divide-white/10" aria-label="最近成功登录">
            <li v-for="(row, index) in rows" :key="`${row.created_at}-${index}`" class="flex flex-wrap items-center justify-between gap-2 py-3 text-sm">
                <span class="font-medium">{{ methodLabel(row.method) }}</span>
                <time :datetime="row.created_at" class="text-secondary">{{ formatDate(row.created_at) }}</time>
            </li>
        </ol>
    </section>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue';
import { readApiResponse } from '@/utils/apiFeedback.js';

const rows = ref([]);
const busy = ref(false);
const error = ref('');
let request;
let disposed = false;

const methodLabel = method => ({ password: '密码登录', oidc: 'OIDC 登录', cas: 'CAS 登录' })[method] || '账户登录';
const formatDate = value => {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? '时间不可用' : date.toLocaleString();
};

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
