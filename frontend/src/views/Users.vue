<template>
  <div class="page-shell">
    <section class="page-header mb-6">
      <div>
        <h1 class="page-title">用户管理</h1>
      </div>
      <button class="primary-button" @click="openCreateModal">
        <i class="ri-add-line"></i>
        新增用户
      </button>
    </section>


    <!-- 工具栏 -->
    <div class="toolbar-surface flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="relative flex-1">
        <i class="ri-search-line absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400 text-sm pointer-events-none"></i>
        <input
          v-model="searchInput"
          type="text"
          placeholder="搜索用户名..."
          class="input-modern pl-9 py-2.5 min-h-0 w-full"
          @input="onSearchInput"
        />
        <i
          v-if="searchInput !== debouncedSearch"
          class="ri-loader-2-line absolute right-3.5 top-1/2 -translate-y-1/2 text-slate-400 text-sm animate-spin"
        ></i>
      </div>
      <div class="flex items-center gap-2.5 w-full sm:w-auto">
        <select
          v-model="roleFilter"
          class="input-modern py-2.5 min-h-0 w-full sm:w-[150px]"
          @change="onRoleFilterChange"
        >
          <option value="all">全部角色</option>
          <option value="1">管理员</option>
          <!-- 修复点：后端 role 校验为 oneof=1 3，普通用户的值应为 3 -->
          <option value="3">普通用户</option>
        </select>
        <div class="stat-tile px-3.5 py-2.5 hidden sm:flex items-center gap-2 shrink-0">
          <span class="text-xs text-slate-400 dark:text-slate-500">共</span>
          <span class="text-sm font-semibold text-slate-900 dark:text-white">{{ total }}</span>
          <span class="text-xs text-slate-400 dark:text-slate-500">位用户</span>
        </div>
      </div>
    </div>

    <!-- 移动端统计 -->
    <div class="sm:hidden stat-tile mt-3 px-3.5 py-2.5 flex items-center gap-2">
      <span class="text-xs text-slate-400 dark:text-slate-500">共</span>
      <span class="text-sm font-semibold text-slate-900 dark:text-white">{{ total }}</span>
      <span class="text-xs text-slate-400 dark:text-slate-500">位用户</span>
      <span v-if="debouncedSearch" class="ml-auto text-xs text-slate-400 truncate">
        搜索: <span class="text-slate-700 dark:text-slate-200">{{ debouncedSearch }}</span>
      </span>
    </div>

    <!-- 加载骨架 -->
    <div v-if="loading" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5 gap-4 mt-4">
      <div v-for="i in 8" :key="i" class="section-card h-[190px] animate-pulse overflow-hidden">
        <div class="h-full bg-slate-200 dark:bg-slate-800 rounded-[16px]"></div>
      </div>
    </div>

    <div v-else-if="loadError" class="section-card mt-4 p-5" role="alert">
      <p class="text-sm text-red-600">{{ loadError }}</p>
      <button class="soft-button mt-3" @click="fetchUsers">重试</button>
    </div>
    <!-- 空数据 -->
    <div v-else-if="users.length === 0" class="section-card flex flex-col items-center justify-center py-20 text-center mt-4">
      <div class="flex items-center justify-center size-16 rounded-full bg-slate-100 dark:bg-slate-800 mb-4">
        <i class="ri-user-line text-2xl text-slate-400 dark:text-slate-500"></i>
      </div>
      <h3 class="text-lg font-medium text-slate-800 dark:text-white mb-1">暂无用户数据</h3>
      <p class="text-sm text-slate-500 dark:text-slate-400">
        {{ debouncedSearch ? "没有找到匹配的用户，试试其他关键词" : '点击右上角"新增用户"按钮创建第一个用户' }}
      </p>
    </div>

    <!-- 用户卡片列表 -->
    <div v-else class="grid grid-cols-[repeat(auto-fit,minmax(min(320px,100%),1fr))] gap-6">
      <UserCard v-for="user in users" :key="user.id" v-bind="{ user, SuperAdminID, activeDropdown, getAvatarColor, getInitials, formatDate, closeDropdown, setDropdownRef, toggleDropdown, openRoleModal, openStorageDialog, openPermissionsDialog, handleResetPassword, openDeleteModal }" />
    </div>

    <UserAccessDialog v-if="accessDialog" :key="`${accessDialog.user.id}-${accessDialog.mode}`" v-bind="accessDialog" @close="accessDialog = null" @saved="onAccessSaved" />

    <!-- 分页 -->
    <div v-if="totalPages > 1" class="flex items-center justify-center gap-1.5 mt-10">
      <button
        class="w-8 h-8 flex items-center justify-center rounded-lg border border-slate-200 dark:border-white/10 bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 transition disabled:opacity-40 disabled:cursor-not-allowed"
        :disabled="page <= 1"
        @click="goToPage(page - 1)"
      >
        <i class="ri-arrow-left-s-line text-sm"></i>
      </button>
      <template v-for="p in pageNumbers" :key="p">
        <span v-if="p === '...'" class="px-1.5 text-slate-400 text-sm select-none">...</span>
        <button
          v-else
          class="w-8 h-8 flex items-center justify-center rounded-lg border text-sm font-medium transition"
          :class="page === p ? 'border-slate-900 dark:border-white bg-slate-900 dark:bg-white text-white dark:text-slate-900 shadow-sm' : 'border-slate-200 dark:border-white/10 bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800'"
          @click="goToPage(p)"
        >
          {{ p }}
        </button>
      </template>
      <button
        class="w-8 h-8 flex items-center justify-center rounded-lg border border-slate-200 dark:border-white/10 bg-white dark:bg-slate-900 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-800 transition disabled:opacity-40 disabled:cursor-not-allowed"
        :disabled="page >= totalPages"
        @click="goToPage(page + 1)"
      >
        <i class="ri-arrow-right-s-line text-sm"></i>
      </button>
    </div>
  </div>
</template>

<script setup>

import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import UserCard from '@/components/users/UserCard.vue'
import UserAccessDialog from '@/components/users/UserAccessDialog.vue'
import { createUserManagementActions } from '@/utils/userManagementActions.js'
import message from '@/utils/message.js'
import { readApiResponse } from '@/utils/apiFeedback.js'

const SuperAdminID = 1
const RoleAdmin = 1
const RoleUser = 3 
const PAGE_LIMIT = 12

const users = ref([])
const total = ref(0)
const accessDialog = ref(null)
const multiStorageSync = ref(false)
const totalPages = ref(0)
const page = ref(1)
const loading = ref(true)
const loadError = ref('')
let disposed = false, userRequest = 0
const searchInput = ref('')
const debouncedSearch = ref('')
const roleFilter = ref('all')
const activeDropdown = ref(null)
const searchTimer = ref(null)
const dropdownRefs = ref(new Map())

const AVATAR_COLORS = [
  'bg-rose-500', 'bg-amber-500', 'bg-emerald-500', 'bg-cyan-500',
  'bg-violet-500', 'bg-pink-500', 'bg-teal-500', 'bg-orange-500',
]



function getAvatarColor(id) {
  return AVATAR_COLORS[id % AVATAR_COLORS.length]
}

function getInitials(name) {
  return name.slice(0, 2).toUpperCase()
}

function formatDate(dateStr) {
  if (!dateStr) return '--'
  const d = new Date(dateStr)
  if (isNaN(d.getTime())) return dateStr
  return d.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' })
}





const pageNumbers = computed(() => {
  const total = totalPages.value
  const current = page.value
  if (total <= 5) return Array.from({ length: total }, (_, i) => i + 1)
  const pages = [1]
  if (current > 3) pages.push('...')
  for (let i = Math.max(2, current - 1); i <= Math.min(total - 1, current + 1); i++) {
    pages.push(i)
  }
  if (current < total - 2) pages.push('...')
  if (total > 1) pages.push(total)
  return pages
})

function goToPage(p) {
  if (p >= 1 && p <= totalPages.value) {
    page.value = p
  }
}

function setDropdownRef(userId, el) {
  if (el) dropdownRefs.value.set(userId, el)
  else dropdownRefs.value.delete(userId)
}

function toggleDropdown(userId) {
  activeDropdown.value = activeDropdown.value === userId ? null : userId
}

function closeDropdown() {
  activeDropdown.value = null
}

function handleClickOutside(e) {
  if (!activeDropdown.value) return
  const targetId = activeDropdown.value
  const dom = dropdownRefs.value.get(targetId)
  if (!dom) {
    closeDropdown()
    return
  }
  if (!dom.contains(e.target)) {
    closeDropdown()
  }
}

function onSearchInput() {
  if (searchTimer.value) clearTimeout(searchTimer.value)
  searchTimer.value = setTimeout(() => {
    debouncedSearch.value = searchInput.value
    page.value = 1
  }, 300)
}

function onRoleFilterChange() {
  page.value = 1
}

async function fetchUsers() {
  const request = ++userRequest
  loading.value = true
  loadError.value = ''
  try {
    const params = new URLSearchParams({
      page: String(page.value),
      limit: String(PAGE_LIMIT),
    })
    if (debouncedSearch.value) params.set('username', debouncedSearch.value)
    if (roleFilter.value !== 'all') params.set('role', roleFilter.value)

    const res = await fetch(`/api/users?${params}`, {
      headers: { 'X-Requested-With': 'XMLHttpRequest' }
    })
    const result = await readApiResponse(res, '获取用户列表失败')
    if (disposed || request !== userRequest) return

    if (res.ok && result.code === 200) {
      if (!Array.isArray(result.data?.list) || !Number.isSafeInteger(result.data.total) || typeof result.data.multi_storage_sync !== 'boolean') throw new Error('用户列表响应异常，请重试')
      multiStorageSync.value = result.data.multi_storage_sync
      users.value = result.data.list.slice(0, PAGE_LIMIT)
      total.value = result.data.total || 0
      totalPages.value = Math.ceil(total.value / PAGE_LIMIT) || 1
    } else {
      message.error(result.message || '获取用户列表失败')
    }
  } catch (err) {
    if (disposed || request !== userRequest) return
    loadError.value = err.message || '获取用户列表失败，请重试'
    message.error(loadError.value)
  } finally {
    if (!disposed && request === userRequest) loading.value = false
  }
}

watch([page, debouncedSearch, roleFilter], () => {
  fetchUsers()
})

const { disposeDialogs, openCreateModal, openDeleteModal, openRoleModal, handleResetPassword } = createUserManagementActions({ closeDropdown, fetchUsers });

function openStorageDialog(user) { closeDropdown(); accessDialog.value = { user, mode: 'storage' }; }
function openPermissionsDialog(user) { closeDropdown(); accessDialog.value = { user, mode: 'permissions' }; }
function onAccessSaved(updated) {
  userRequest++; loading.value = false;
  users.value = users.value.map(user => user.id === updated.id ? updated : user);
  accessDialog.value = null;
  message.success('授权已保存并核对，该用户需重新登录生效');
}

onMounted(async () => {
  document.addEventListener('click', handleClickOutside)
  fetchUsers()
})

onUnmounted(() => {
  disposed = true; userRequest++;
  disposeDialogs()
  if (searchTimer.value) clearTimeout(searchTimer.value)
  document.removeEventListener('click', handleClickOutside)
  dropdownRefs.value.clear()
})
</script>