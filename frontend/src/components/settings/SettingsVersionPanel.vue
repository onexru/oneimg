<template>
  <!-- 「系统」页签：版本信息。
       原本挂在账户设置页，但它是整站级别的信息（版本、更新），与「我的账号」
       无关，且普通用户看不到却要陪着一块空白卡片，因此迁到系统设置。 -->
  <div class="surface space-y-4 p-3.5 sm:p-4 md:space-y-5 md:p-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 class="panel-title">
          <span class="panel-icon"><i class="ri-information-line"></i></span>
          版本信息
        </h2>
        <p class="mt-1.5 text-xs leading-5 text-slate-500 dark:text-slate-400">
          当前部署版本与 GitHub 最新发布。检查更新只读取发布信息，不会自动升级。
        </p>
      </div>
      <a
        v-if="currentVersion"
        :href="`https://github.com/${repository}/releases`"
        target="_blank" rel="noopener noreferrer"
        class="inline-flex shrink-0 items-center gap-1.5 text-xs font-medium text-slate-500 transition-colors hover:text-primary dark:text-slate-400"
      >
        <i class="ri-github-fill" aria-hidden="true"></i>{{ repository }}
      </a>
    </div>

    <!-- 当前版本：这一块不依赖网络，永远可读 -->
    <div class="rounded-[18px] border border-slate-200/80 bg-slate-50 px-4 py-3.5 dark:border-white/10 dark:bg-slate-950">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div class="min-w-0">
          <p class="text-xs uppercase tracking-[0.18em] text-slate-400 dark:text-slate-500">当前版本</p>
          <div class="mt-1.5 flex flex-wrap items-baseline gap-2">
            <p class="text-2xl font-semibold text-slate-900 dark:text-slate-100">{{ currentVersion || '—' }}</p>
            <span
              v-if="statusBadge"
              class="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs"
              :class="statusBadge.class"
            >
              <i :class="statusBadge.icon" aria-hidden="true"></i>{{ statusBadge.label }}
            </span>
          </div>
        </div>
        <button
          type="button"
          class="soft-button shrink-0"
          :disabled="checking"
          @click="checkForUpdate"
        >
          <i :class="checking ? 'ri-loader-4-line animate-spin' : 'ri-refresh-line'" aria-hidden="true"></i>
          <span>{{ checking ? '检查中…' : '检查更新' }}</span>
        </button>
      </div>
    </div>

    <!-- 检查结果 -->
    <p v-if="errorText" class="flex items-start gap-2 rounded-xl border border-amber-200/80 bg-amber-50 px-3 py-2.5 text-sm text-amber-800 dark:border-amber-400/20 dark:bg-amber-400/10 dark:text-amber-200">
      <i class="ri-error-warning-line mt-0.5 shrink-0" aria-hidden="true"></i>
      <span>{{ errorText }}</span>
    </p>

    <div v-if="latest" class="space-y-3">
      <div class="grid gap-3 sm:grid-cols-2">
        <div class="rounded-[18px] border border-slate-200/80 bg-slate-50 px-4 py-3 dark:border-white/10 dark:bg-slate-950">
          <p class="text-xs uppercase tracking-[0.18em] text-slate-400 dark:text-slate-500">最新版本</p>
          <p class="mt-2 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ latest.tag_name }}</p>
        </div>
        <div class="rounded-[18px] border border-slate-200/80 bg-slate-50 px-4 py-3 dark:border-white/10 dark:bg-slate-950">
          <p class="text-xs uppercase tracking-[0.18em] text-slate-400 dark:text-slate-500">发布时间</p>
          <p class="mt-2 text-sm font-semibold text-slate-800 dark:text-slate-100">{{ formatDate(latest.published_at) || '—' }}</p>
        </div>
      </div>

      <div class="rounded-[20px] border border-dashed border-slate-300/90 bg-slate-50/80 p-4 dark:border-white/10 dark:bg-slate-950/70">
        <div class="flex items-center justify-between gap-3">
          <p class="text-sm font-medium text-slate-800 dark:text-slate-100">更新日志</p>
          <a
            :href="latest.html_url" target="_blank" rel="noopener noreferrer"
            class="inline-flex items-center rounded-full border border-primary/20 px-3 py-1.5 text-xs font-medium text-primary transition-colors hover:bg-primary hover:text-white"
          >
            前往更新<i class="ri-arrow-right-up-line ml-0.5" aria-hidden="true"></i>
          </a>
        </div>
        <div class="mt-3 max-h-[320px] overflow-auto rounded-[16px] bg-white px-4 py-3 text-sm leading-6 text-slate-600 dark:bg-slate-900 dark:text-slate-300">
          <pre class="whitespace-pre-wrap break-words font-sans">{{ latest.body || '暂无更新日志' }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue';
import { readApiResponse } from '@/utils/apiFeedback.js';

const currentVersion = ref('');
const repository = ref('onexru/oneimg');
const latest = ref(null);
const updateAvailable = ref(false);
const errorText = ref('');
const checking = ref(false);
// 打开页面只取当前版本（不联网），用户点「检查更新」才查 GitHub
const checkedOnce = ref(false);

const statusBadge = computed(() => {
  if (!currentVersion.value) return null;
  if (!checkedOnce.value) return null;
  if (errorText.value) return null;
  if (updateAvailable.value) {
    return {
      label: '有新版本',
      icon: 'ri-arrow-up-circle-line',
      class: 'border-primary/30 bg-primary/10 text-primary',
    };
  }
  return {
    label: '已是最新',
    icon: 'ri-checkbox-circle-line',
    class: 'border-emerald-300/60 bg-emerald-50 text-emerald-700 dark:border-emerald-400/20 dark:bg-emerald-400/10 dark:text-emerald-300',
  };
});

const formatDate = (value) => {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' });
};

const loadVersion = async (check) => {
  if (check) checking.value = true;
  try {
    const response = await fetch(`/api/system/version${check ? '?check=1' : ''}`, {
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
    });
    const result = await readApiResponse(response, '获取版本信息失败');
    if (!response.ok || result.code !== 200) {
      throw new Error(result.message || '获取版本信息失败');
    }
    const data = result.data || {};
    currentVersion.value = data.current_version || '';
    repository.value = data.repository || 'onexru/oneimg';
    if (check) {
      checkedOnce.value = true;
      // 网络失败时后端给 release_error，此时不清掉上次成功的结果之外的任何东西，
      // 但要如实提示，不能假装「已是最新」。
      if (data.release_error) {
        errorText.value = data.release_error;
        latest.value = null;
        updateAvailable.value = false;
      } else {
        errorText.value = '';
        latest.value = data.latest || null;
        updateAvailable.value = data.update_available === true;
      }
    }
  } catch (error) {
    if (check) {
      checkedOnce.value = true;
      errorText.value = error.message || '获取版本信息失败';
      latest.value = null;
      updateAvailable.value = false;
    }
  } finally {
    if (check) checking.value = false;
  }
};

const checkForUpdate = () => loadVersion(true);

onMounted(() => loadVersion(false));
</script>
