<template>
      <div
        class="rounded-[16px] border border-slate-200/80 bg-white shadow-sm dark:bg-slate-900 group relative transition-all duration-300 hover:shadow-lg"
        :data-user-id="user.id" @click="closeDropdown"
      >
        <div class="w-full h-full overflow-hidden rounded-[16px]">
          <!-- 顶部角色标识条 -->
          <div
            class="h-1.5 w-full"
            :class="user.role === 1 ? 'bg-emerald-500' : 'bg-slate-300 dark:bg-slate-600'"
          ></div>
          <div class="p-4 flex flex-col gap-3 h-full">
            <div class="flex items-start justify-between">
              <div class="flex items-center gap-3 min-w-0 flex-1 pr-2">
                <div
                  class="shrink-0 size-11 rounded-full flex items-center justify-center text-white font-semibold text-sm"
                  :class="getAvatarColor(user.id)"
                >
                  {{ getInitials(user.username) }}
                </div>
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2 flex-wrap">
                    <h3 class="font-semibold text-sm text-slate-900 dark:text-white truncate" :title="user.username">
                      {{ user.username }}
                    </h3>
                    <span
                      v-if="user.id === SuperAdminID"
                      class="shrink-0 text-[10px] px-1.5 h-4 leading-4 rounded-full bg-amber-500/15 text-amber-700 dark:text-amber-400 border border-amber-500/20"
                    >超管</span>
                  </div>
                  <p class="text-xs text-slate-400 dark:text-slate-500 mt-0.5">ID: {{ user.id }}</p>
                </div>
              </div>

              <!-- 下拉操作 -->
              <div class="relative shrink-0" :ref="(el) => setDropdownRef(user.id, el)">
                <button
                  class="w-8 h-8 flex items-center justify-center rounded-lg text-slate-400 hover:text-slate-600 hover:bg-slate-100 dark:hover:text-slate-200 dark:hover:bg-slate-800 transition-opacity duration-200 md:group-hover:opacity-100 opacity-100"
                  :aria-label="`${user.username}的操作`" @click.stop="toggleDropdown(user.id)"
                >
                  <i class="ri-more-2-fill text-base"></i>
                </button>
              </div>
            </div>

            <!-- 标签行 -->
            <div class="flex items-center gap-1.5 flex-wrap">
              <span
                class="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full border"
                :class="user.role === 1 ? 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-500/20' : 'bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-400 border-slate-200/80 dark:border-white/10'"
              >
                <i :class="user.role === 1 ? 'ri-shield-star-line' : 'ri-user-line'" class="text-xs"></i>
                {{ user.role === 1 ? "管理员" : "普通用户" }}
              </span>
              <span class="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full border bg-slate-50 dark:bg-slate-800/50 text-slate-500 dark:text-slate-400 border-slate-200/80 dark:border-white/10">
                <i class="ri-folder-3-line text-xs" aria-hidden="true"></i>
                <span data-user-storage>{{ userStorageSummary(user).label }}</span>
              </span>
              <span class="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full border bg-slate-50 dark:bg-slate-800/50 text-slate-500 dark:text-slate-400 border-slate-200/80 dark:border-white/10">
                <i class="ri-key-2-line text-xs" aria-hidden="true"></i>
                <span data-user-permissions>{{ userPermissionSummary(user).label }}</span>
              </span>
            </div>

            <div class="grid grid-cols-2 gap-2" data-user-access-actions>
              <button type="button" class="soft-button px-2 text-xs" @click.stop="openStorageDialog(user)">分配存储源</button>
              <button type="button" class="soft-button px-2 text-xs" @click.stop="openPermissionsDialog(user)">用户权限</button>
            </div>

            <!-- 上传配额：所有角色统一显示，布局完全一致。
                 管理员用「不限」表达豁免，而不是隐藏整块 —— 否则超管卡片
                 会比普通用户矮一截，网格里错位很难看。 -->
            <div v-if="quotaInfo" class="rounded-xl border border-slate-200/80 dark:border-white/10 bg-slate-50/60 dark:bg-slate-800/30 px-3 py-2.5" data-user-quota>
              <div class="flex items-center justify-between gap-2">
                <span class="inline-flex items-center gap-1.5 text-[11px] font-medium text-slate-500 dark:text-slate-400">
                  <i class="ri-hard-drive-2-line text-xs" aria-hidden="true"></i>
                  上传配额
                </span>
                <span class="inline-flex items-center gap-1.5">
                  <span
                    v-if="quotaInfo.quota_custom"
                    class="text-[10px] px-1.5 h-4 leading-4 rounded-full bg-primary/10 text-primary-dark dark:bg-primary/20 dark:text-blue-300 border border-primary/20"
                    title="管理员为该账号单独设置了配额"
                  >自定义</span>
                  <span
                    v-else-if="quotaInfo.admin_exempt"
                    class="text-[10px] px-1.5 h-4 leading-4 rounded-full bg-amber-500/15 text-amber-700 dark:text-amber-400 border border-amber-500/20"
                    title="管理员不受上传配额限制"
                  >豁免</span>
                  <span class="text-[11px] tabular-nums text-slate-600 dark:text-slate-300">
                    {{ quotaInfo.unlimited ? '不限' : formatBytes(quotaInfo.quota_bytes) }}
                  </span>
                </span>
              </div>
              <!-- 三行结构固定：进度条（不限时为等高的灰条占位）+ 用量说明，
                   保证「不限」与「限额」两种卡片的这一块高度相同。 -->
              <div class="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-slate-200 dark:bg-slate-700">
                <div
                  v-if="!quotaInfo.unlimited"
                  class="h-full rounded-full transition-[width] duration-300"
                  :class="quotaBarClass"
                  :style="{ width: quotaInfo.percent + '%' }"
                ></div>
                <div v-else class="h-full w-full rounded-full bg-slate-300/70 dark:bg-slate-600/60"></div>
              </div>
              <p class="mt-1.5 text-[11px] tabular-nums text-slate-400 dark:text-slate-500">
                已用 {{ formatBytes(quotaInfo.used_bytes) }}<template v-if="!quotaInfo.unlimited"> · {{ quotaInfo.percent.toFixed(1) }}%</template>
              </p>
            </div>

            <!-- 创建时间 -->
            <div class="flex items-center gap-1.5 text-xs text-slate-400 dark:text-slate-500 mt-auto pt-1">
              <i class="ri-calendar-line text-xs"></i>
              <span>创建于 {{ formatDate(user.CreatedAt || user.created_at) }}</span>
            </div>
          </div>
        </div>
        <div
          v-if="activeDropdown === user.id"
          class="absolute right-0 top-[55px] right-[20px] mt-1 w-44 z-[60] rounded-xl border border-slate-200/80 dark:border-white/10 bg-white dark:bg-slate-900 shadow-xl py-1.5"
          @click.stop
        >
          <button class="w-full flex items-center gap-2.5 px-3.5 py-2 text-sm text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800 transition text-left" @click="openRoleModal(user)">
            <i class="ri-shield-star-line text-base"></i>
            修改角色
          </button>
          <button class="w-full flex items-center gap-2.5 px-3.5 py-2 text-sm text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800 transition text-left" @click="handleResetPassword(user)">
            <i class="ri-key-2-line text-base"></i>
            重置密码
          </button>
          <button
            v-if="canManageQuota && user.id !== SuperAdminID"
            class="w-full flex items-center gap-2.5 px-3.5 py-2 text-sm text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800 transition text-left"
            @click="openQuotaDialog(user)"
          >
            <i class="ri-hard-drive-2-line text-base"></i>
            上传配额
          </button>
          <div class="my-1.5 border-t border-slate-100 dark:border-white/5"></div>
          <button
            class="w-full flex items-center gap-2.5 px-3.5 py-2 text-sm transition text-left"
            :class="user.id === SuperAdminID ? 'text-slate-300 dark:text-slate-600 cursor-not-allowed bg-slate-50 dark:bg-slate-800/30' : 'text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20'"
            :disabled="user.id === SuperAdminID"
            @click="user.id !== SuperAdminID && openDeleteModal(user)"
          >
            <i class="ri-delete-bin-7-line text-base"></i>
            删除用户
          </button>
        </div>
      </div>
</template>
<script setup>
import { computed } from 'vue';
import { userStorageSummary, userPermissionSummary } from '@/utils/userAccessSummary.js';
import { formatBytes } from '@/utils/byteSize.js';

const props = defineProps(["user", "SuperAdminID", "activeDropdown", "getAvatarColor", "getInitials", "formatDate", "closeDropdown", "setDropdownRef", "toggleDropdown", "openRoleModal", "openStorageDialog", "openPermissionsDialog", "openQuotaDialog", "handleResetPassword", "openDeleteModal", "canManageQuota"])

// 后端对管理员不下发 quota（他们不受配额约束），因此这里只需判空。
const quotaInfo = computed(() => props.user.quota || null)

// 进度条配色分三档：<70 正常、70-90 提醒、>=90 告警。
// 单色进度条无法传达「快满了」，而配额的意义恰恰在于提前预警。
const quotaBarClass = computed(() => {
  const p = props.user.quota?.percent ?? 0
  if (p >= 90) return 'bg-red-500'
  if (p >= 70) return 'bg-amber-500'
  return 'bg-primary'
})
</script>
