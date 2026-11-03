<template>
  <div class="page-shell">
    <section class="page-header">
      <div class="page-actions">
        <span class="stat-inline">当前账号 <strong>{{ currentUsername || '—' }}</strong></span>
      </div>
    </section>

    <!-- 改密表单：单列铺满，与下方「登录会话」「登录历史」同宽，整页对齐 -->
    <div class="surface">
      <div class="mb-4 flex flex-wrap items-start justify-between gap-2">
        <div>
          <h2 class="surface-title">
            <i class="ri-shield-user-line text-primary" aria-hidden="true"></i>
            账户设置
          </h2>
          <p class="surface-subtitle">修改登录用户名或密码。保存后需要重新登录。</p>
        </div>
      </div>

      <form class="space-y-4" @submit.prevent="updateAccount">
        <!-- 新用户名：仅管理员可见（普通用户无改名权限） -->
        <div v-if="isAdmin">
          <label class="field-label" for="newUsername">新用户名</label>
          <input
            id="newUsername"
            v-model="accountForm.newUsername"
            type="text"
            class="input-modern"
            placeholder="留空则不修改"
            minlength="3"
            maxlength="20"
            autocomplete="username"
          />
          <p class="field-hint">3–20 个字符，留空表示不修改当前用户名（{{ currentUsername || '—' }}）。</p>
        </div>

        <!-- 当前密码：任何修改都必须先验证 -->
        <div>
          <label class="field-label" for="currentPassword">
            当前密码 <span class="text-red-500" aria-hidden="true">*</span>
          </label>
          <input
            id="currentPassword"
            v-model="accountForm.currentPassword"
            type="password"
            class="input-modern"
            placeholder="请输入当前密码以确认修改"
            autocomplete="current-password"
            required
          />
        </div>

        <!-- 新密码 + 确认：并排，窄屏堆叠 -->
        <div class="grid gap-4 sm:grid-cols-2">
          <div>
            <label class="field-label" for="newPassword">新密码</label>
            <input
              id="newPassword"
              v-model="accountForm.newPassword"
              type="password"
              class="input-modern"
              placeholder="留空则不修改"
              minlength="6"
              autocomplete="new-password"
            />
            <p class="field-hint">至少 8 个字符。</p>
          </div>
          <div>
            <label class="field-label" for="confirmPassword">确认新密码</label>
            <input
              id="confirmPassword"
              v-model="accountForm.confirmPassword"
              type="password"
              class="input-modern"
              placeholder="再次输入新密码"
              autocomplete="new-password"
            />
            <!-- 只有两栏都填了且不一致时才提示，避免还没输入就报错 -->
            <p v-if="passwordMismatch" class="field-hint text-amber-600 dark:text-amber-400">
              两次输入的新密码不一致。
            </p>
          </div>
        </div>

        <div class="flex flex-wrap items-center justify-between gap-3 border-t border-slate-200/70 pt-4 dark:border-white/10">
          <p class="text-xs text-slate-500 dark:text-slate-400">留空的项目不会被修改。</p>
          <button type="submit" class="primary-button" :disabled="isUpdatingAccount || !canSubmit">
            <i v-if="isUpdatingAccount" class="ri-loader-4-line animate-spin" aria-hidden="true"></i>
            <span>{{ isUpdatingAccount ? '保存中…' : '保存修改' }}</span>
          </button>
        </div>
      </form>
    </div>

    <!-- 会话与历史：桌面并排，避免纵向拉得过长；窄屏自动堆叠。
         两块都由组件自带 .surface，这里只负责栅格，不再额外加 mt-*。

         高度约束是必须的：两个子组件都是 `flex h-full flex-col` + 列表 `flex-1`，
         如果这一行没有确定高度，flex-1 会让列表把所有 100 条全铺开、整页被撑到几千像素。
         但**只在并排时**加高度：堆叠成两行时，同一高度会被两块分掉，各自只剩一半，
         列表会被压到 0 高度。所以这里用 xl: 前缀，窄屏交给内容自然高度。 -->
    <div class="grid gap-4 xl:h-[min(34rem,max(22rem,calc(100vh-24rem)))] xl:grid-cols-2">
      <AccountSessions />
      <AccountLoginHistory />
    </div>
  </div>
</template>

<script setup>
import AccountSessions from '@/components/account/AccountSessions.vue';
import AccountLoginHistory from '@/components/account/AccountLoginHistory.vue';
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import Message from '@/utils/message.js';

const router = useRouter();

// 读取一次即可：只在挂载时用于展示与角色判断，不需要响应式跟随
const storedUser = JSON.parse(localStorage.getItem('userInfo') || '{}');
const currentUsername = storedUser?.username || '';
// 游客 role 为 2，进不到本页；管理员 role 为 1
const isAdmin = computed(() => storedUser?.role === 1);

const accountForm = ref({
  newUsername: '',
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
});
const isUpdatingAccount = ref(false);

const passwordMismatch = computed(() => {
  const { newPassword, confirmPassword } = accountForm.value;
  return Boolean(newPassword) && Boolean(confirmPassword) && newPassword !== confirmPassword;
});

// 有任意一项改动且必填项已填，才允许提交（避免点了才发现没填当前密码）
const canSubmit = computed(() => {
  const { newUsername, currentPassword, newPassword } = accountForm.value;
  const hasChange = Boolean(newUsername.trim()) || Boolean(newPassword);
  return hasChange && Boolean(currentPassword);
});

onMounted(() => {
  // 游客没有账户设置页
  if (Number(storedUser?.role) === 2) router.replace('/');
});

const resetForm = () => {
  accountForm.value = {
    newUsername: '',
    currentPassword: '',
    newPassword: '',
    confirmPassword: '',
  };
};

const updateAccount = async () => {
  const { newUsername, currentPassword, newPassword, confirmPassword } = accountForm.value;
  const hasUsernameChange = newUsername && newUsername.trim() !== '';
  const hasPasswordChange = newPassword && newPassword.trim() !== '';

  if (!hasUsernameChange && !hasPasswordChange) {
    Message.error('请输入要修改的用户名或密码');
    return;
  }
  if (hasUsernameChange) {
    if (newUsername.length < 3) {
      Message.error('用户名长度至少为 3 位');
      return;
    }
    if (newUsername.length > 20) {
      Message.error('用户名长度不能超过 20 位');
      return;
    }
  }
  if (hasPasswordChange) {
    if (newPassword.length < 8) {
      Message.error('新密码长度至少为 8 位');
      return;
    }
    if (newPassword !== confirmPassword) {
      Message.error('两次输入的新密码不一致');
      return;
    }
  }
  if (!currentPassword) {
    Message.error('请输入当前密码以确认修改');
    return;
  }

  isUpdatingAccount.value = true;
  try {
    const response = await fetch('/api/account/change', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest',
      },
      body: JSON.stringify({
        new_username: newUsername,
        current_password: currentPassword,
        new_password: newPassword,
      }),
    });

    let result = {};
    try {
      result = await response.json();
    } catch {
      result = {};
    }

    if (!response.ok || (result.code !== 200 && result.success !== true)) {
      if (response.status === 401) {
        Message.error(result.message || '当前密码错误或登录已过期，请确认后重试');
        return;
      }
      if (response.status === 403) {
        Message.error(result.message || '无权执行此操作');
        return;
      }
      throw new Error(result.message || '修改失败');
    }

    // 改密/改名后旧会话在服务端已失效，本地身份必须清掉重新登录
    localStorage.removeItem('userInfo');
    localStorage.removeItem('authToken');
    Message.success(result.message || '修改成功，请重新登录');
    resetForm();
    setTimeout(() => router.push('/login'), 1000);
  } catch (error) {
    Message.error(error.message || '更新失败');
  } finally {
    isUpdatingAccount.value = false;
  }
};
</script>
