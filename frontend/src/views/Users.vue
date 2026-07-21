<template>
  <div class="page-shell">
    <section class="page-header mb-6">
      <div>
        <h1 class="page-title">用户管理</h1>
        <p class="page-subtitle">{{ multiStorageSync ? '管理系统用户账号与后台同步存储源' : '管理系统用户账号与权限' }}</p>
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
      <div
        v-for="user in users"
        :key="user.id"
        class="rounded-[16px] border border-slate-200/80 bg-white shadow-sm dark:bg-slate-900 group relative transition-all duration-300 hover:shadow-lg"
        @click="closeDropdown"
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
                  @click.stop="toggleDropdown(user.id)"
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
                <i class="ri-folder-3-line text-xs"></i>
                {{ getUserBucketCount(user) }} {{ multiStorageSync ? '个同步源' : '个存储桶' }}
              </span>
              <span class="inline-flex items-center gap-1 text-[11px] px-2 py-0.5 rounded-full border bg-slate-50 dark:bg-slate-800/50 text-slate-500 dark:text-slate-400 border-slate-200/80 dark:border-white/10">
                <i class="ri-key-2-line text-xs"></i>
                {{ getUserCodeCount(user) }} 个权限
              </span>
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
          <button class="w-full flex items-center gap-2.5 px-3.5 py-2 text-sm text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800 transition text-left" @click="openProfileModal(user)">
            <i class="ri-shield-keyhole-line text-base"></i>
            {{ multiStorageSync ? '设置同步源' : '设置权限' }}
          </button>
          <button class="w-full flex items-center gap-2.5 px-3.5 py-2 text-sm text-slate-700 dark:text-slate-200 hover:bg-slate-50 dark:hover:bg-slate-800 transition text-left" @click="handleResetPassword(user)">
            <i class="ri-key-2-line text-base"></i>
            重置密码
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
    </div>

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
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import PopupModal from '@/utils/popupModal.js'
import message from '@/utils/message.js'

const SuperAdminID = 1
const RoleAdmin = 1
const RoleUser = 3 
const PAGE_LIMIT = 12

const users = ref([])
const total = ref(0)
const buckets = ref([])
const multiStorageSync = ref(false)
const totalPages = ref(0)
const page = ref(1)
const loading = ref(true)
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

const PERMISSION_GROUPS = [
  {
    title: '用户管理',
    items: [
      { code: 'user:create', name: '添加用户' },
      { code: 'user:delete', name: '删除用户' },
      { code: 'user:role:update', name: '修改角色' },
      { code: 'user:permission:update', name: '编辑权限' },
      { code: 'user:password:reset', name: '重置密码' },
    ]
  },
  {
    title: '内容与标签',
    items: [
      { code: 'tag:create', name: '新增Tag' },
      { code: 'tag:delete', name: '删除Tag' },
      { code: 'tag:update', name: '编辑Tag' },
    ]
  },
  {
    title: '存储管理',
    items: [
      { code: 'storage:create', name: '新增存储' },
      { code: 'storage:update', name: '编辑存储' },
      { code: 'storage:delete', name: '删除存储' },
    ]
  },
  {
    title: '图片管理',
    items: [
      { code: 'image:delete', name: '删除图片' },
      { code: 'image:tag:add', name: '添加图片标签' },
      { code: 'image:tag:delete', name: '删除图片标签' },
      { code: 'image:access:source', name: '图片存储源' },
    ]
  },
  {
    title: '系统设置',
    items: [
      { code: 'setting:upload', name: '上传与存储' },
      { code: 'setting:image', name: '图片处理' },
      { code: 'setting:security', name: '安全与登录' },
      { code: 'setting:notification', name: '通知' },
      { code: 'setting:api', name: 'API' },
      { code: 'setting:seo', name: '站点SEO' },
    ]
  }
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

function getUserBucketCount(user) {
  // 兼容容错：防止后端序列化大写或者小写不一致
  const perms = user.permission || user.Permission || {}
  const userBucketIds = perms.buckets || perms.Buckets || []
  if (!multiStorageSync.value) return userBucketIds.length
  return userBucketIds.filter(id => buckets.value.some(bucket => bucket.id === id && bucket.type !== 'default')).length
}

function getUserCodeCount(user) {
  const perms = user.permission || user.Permission || {}
  const userCodes = perms.codes || perms.Codes || []
  return userCodes.length
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
  loading.value = true
  try {
    const params = new URLSearchParams({
      page: String(page.value),
      limit: String(PAGE_LIMIT),
    })
    if (debouncedSearch.value) params.set('username', debouncedSearch.value)
    if (roleFilter.value !== 'all') params.set('role', roleFilter.value)

    const res = await fetch(`/api/users?${params}`, {
      headers: { 'Authorization': `Bearer ${localStorage.getItem('authToken')}` }
    })
    const result = await res.json()

    if (res.ok && result.code === 200) {
      users.value = result.data.list || []
      total.value = result.data.total || 0
      totalPages.value = Math.ceil(total.value / PAGE_LIMIT) || 1
    } else {
      message.error(result.message || '获取用户列表失败')
    }
  } catch (err) {
    console.error('获取用户列表失败:', err)
    message.error('网络错误，请重试')
  } finally {
    loading.value = false
  }
}

watch([page, debouncedSearch, roleFilter], () => {
  fetchUsers()
})

function openCreateModal() {
  const modal = new PopupModal({
    title: '新增用户',
    type: 'form',
    formFields: [
      {
        name: 'username',
        label: '用户名',
        type: 'text',
        placeholder: '请输入用户名（3-50字符）',
        required: true,
      },
      {
        name: 'password',
        label: '密码',
        type: 'password',
        placeholder: '请输入密码（6-100字符）',
        required: true,
      },
      {
        name: 'role',
        label: '角色',
        type: 'select',
        options: [
          { label: '请选择角色', value: '', disabled: true },
          { label: '管理员', value: '1' },
          { label: '普通用户', value: '3' }, // 这里原本是对的
        ],
        required: true,
      },
    ],
    formSubmit: async (modal, formData) => {
      if (!formData.username || formData.username.length < 3) {
        message.warning('用户名至少3个字符')
        return
      }
      if (!formData.password || formData.password.length < 6) {
        message.warning('密码至少6个字符')
        return
      }

      try {
        const res = await fetch('/api/users/Add', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${localStorage.getItem('authToken')}`,
          },
          body: JSON.stringify({
            username: formData.username,
            password: formData.password,
            role: parseInt(formData.role) || RoleUser,
          }),
        })
        const result = await res.json()

        if (res.ok && result.code === 200) {
          message.success('用户创建成功')
          modal.close()
          fetchUsers()
        } else {
          message.error(result.message || '创建失败')
        }
      } catch (err) {
        console.error('创建用户失败:', err)
        message.error('网络错误，请重试')
      }
    },
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close(),
      },
      {
        text: '创建',
        type: 'primary',
        callback: (modal) => {
          modal.content.querySelector('form').dispatchEvent(
            new Event('submit', { bubbles: true })
          )
        },
      },
    ],
  })
  modal.open()
}

function openDeleteModal(user) {
  closeDropdown()
  const modal = new PopupModal({
    title: '确认删除用户',
    content: `
      <div class="flex items-start gap-3">
        <div class="shrink-0 w-10 h-10 flex items-center justify-center rounded-full bg-red-100 dark:bg-red-900/30">
          <i class="ri-error-warning-fill text-red-500 text-xl"></i>
        </div>
        <div>
          <p class="text-sm text-slate-700 dark:text-slate-200">
            你确定要删除用户 <strong>${user.username}</strong> 吗？
          </p>
          <p class="mt-1.5 text-xs text-slate-500 dark:text-slate-400">
            此操作无法撤销，该用户的所有关联数据将会丢失。
          </p>
        </div>
      </div>
    `,
    type: 'confirm',
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close(),
      },
      {
        text: '确认删除',
        type: 'danger',
        callback: async (modal) => {
          modal.close()
          try {
            const res = await fetch(`/api/users/${user.id}`, {
              method: 'DELETE',
              headers: {
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`,
              },
            })
            const result = await res.json()

            if (res.ok && result.code === 200) {
              message.success('用户已删除')
              fetchUsers()
            } else {
              message.error(result.message || '删除失败')
            }
          } catch (err) {
            console.error('删除用户失败:', err)
            message.error('网络错误，请重试')
          }
        },
      },
    ],
  })
  modal.open()
}

function openProfileModal(user) {
    const bucketOptions = multiStorageSync.value
        ? buckets.value.filter(item => item.type !== 'default')
        : buckets.value
        
    // 兼容容错，获取当前用户的 buckets 和 codes
    const perms = user.permission || user.Permission || {}
    const userBucketIds = perms.buckets || perms.Buckets || []
    const userCodes = perms.codes || perms.Codes || []
    const userRole = user.role || 3;
    
    // 选中的存储桶
    const selectedIds = multiStorageSync.value
        ? userBucketIds.filter(id => bucketOptions.some(item => item.id === id))
        : [...userBucketIds]
        
    // 选中的功能权限码
    const selectedCodes = [...userCodes]

    // 渲染存储桶卡片
    function renderBucketCards() {
        if (bucketOptions.length === 0) {
            const emptyText = multiStorageSync.value ? '暂无可配置的远程存储源' : '暂无可配置的存储桶'
            return `<div class="w-full rounded-xl border border-dashed border-slate-200 px-4 py-5 text-center text-sm text-slate-400 dark:border-white/10 dark:text-slate-500">${emptyText}</div>`
        }
        return bucketOptions.map(item => {
            const isChecked = selectedIds.includes(item.id)
            return `
            <div data-bucket-id="${item.id}" class="bucket-card relative border rounded-2xl px-4 py-2 cursor-pointer transition-all duration-300 shadow-sm select-none border-2 ${isChecked ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30' : 'border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800'}">
                ${isChecked ? `
                <div class="absolute -top-1 -right-1 w-4 h-4 rounded-full bg-blue-500 flex items-center justify-center">
                    <i class="ri-check-line text-white text-[10px]"></i>
                </div>` : ''}
                <div class="text-center">
                    <div class="text-slate-900 dark:text-white text-sm whitespace-nowrap">${item.name}</div>
                </div>
            </div>`
        }).join('')
    }

    // 渲染功能权限区
    function renderCodeCards() {
        return PERMISSION_GROUPS.map(group => `
            <div class="mb-4 last:mb-0">
                <div class="text-xs font-semibold text-slate-400 dark:text-slate-500 mb-2">${group.title}</div>
                <div class="flex flex-wrap gap-2">
                    ${group.items.map(item => {
                        const isChecked = selectedCodes.includes(item.code)
                        return `
                        <div data-code="${item.code}" class="code-card flex items-center gap-1.5 border rounded-lg px-2.5 py-1.5 cursor-pointer transition-all duration-200 select-none ${isChecked ? 'border-emerald-500/50 bg-emerald-50 dark:bg-emerald-900/20 text-emerald-700 dark:text-emerald-400' : 'border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300'}">
                            <i class="${isChecked ? 'ri-checkbox-circle-fill text-emerald-500' : 'ri-checkbox-blank-circle-line text-slate-300 dark:text-slate-600'} text-sm"></i>
                            <span class="text-xs">${item.name}</span>
                        </div>`
                    }).join('')}
                </div>
            </div>
        `).join('')
    }

    const modalContent = `
        <div class="py-1 space-y-6 custom-scrollbar pr-2">
            <p class="text-sm text-slate-600 dark:text-slate-300">
                设置用户 <strong class="text-slate-900 dark:text-white">${user.username}</strong> 的权限节点。
            </p>
            
            <!-- 模块 1: 功能权限 -->
            ${userRole === 3 ? '' : `
            <div>
                <h4 class="text-sm font-medium text-slate-900 dark:text-white mb-3 flex items-center gap-2">
                    <i class="ri-shield-keyhole-line text-blue-500"></i> 功能权限配置
                </h4>
                <div id="codeCardWrap" class="p-3 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-100 dark:border-white/5">
                    ${renderCodeCards()}
                </div>
            </div>
            `}
            <!-- 模块 2: 存储源/桶权限 -->
            <div>
                <h4 class="text-sm font-medium text-slate-900 dark:text-white mb-2 flex items-center gap-2">
                    <i class="ri-hard-drive-2-line text-purple-500"></i> ${multiStorageSync.value ? '同步存储源配置' : '存储桶访问权限'}
                </h4>
                ${multiStorageSync.value ? '<p class="text-xs text-slate-400 dark:text-slate-500 mb-3">文件会始终先保存在本机，此处只配置额外同步目标。</p>' : ''}
                <div id="bucketCardWrap" class="flex flex-wrap gap-3">
                    ${renderBucketCards()}
                </div>
            </div>
        </div>
    `

    const modal = new PopupModal({
        title: '设置用户权限',
        width: '680px',
        content: modalContent,
        buttons: [
            {
                text: '取消',
                type: 'default',
                callback: () => modal.close()
            },
            {
                text: '确认保存',
                type: 'primary',
                callback: async () => {
                    try {
                        const res = await fetch(`/api/users/updatePermission/${user.id}`, {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json',
                                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
                            },
                            body: JSON.stringify({
                                permission: selectedIds,   // 对应后端的 buckets
                                codes: selectedCodes       // 对应后端的 codes
                            })
                        })
                        const data = await res.json()
                        if (data.code === 200) {
                            modal.close()
                            message.success('权限更新成功')
                            fetchUsers()
                        } else {
                            message.error(data.message || '更新失败')
                        }
                    } catch (err) {
                        message.error('网络请求异常')
                    }
                }
            }
        ]
    })
    modal.open()

    // 绑定卡片点击事件
    function bindInteractions() {
        // 绑定 Bucket 卡片
        const bucketWrap = document.getElementById('bucketCardWrap')
        bucketWrap.querySelectorAll('.bucket-card').forEach(card => {
            card.onclick = () => {
                const bid = Number(card.dataset.bucketId)
                const idx = selectedIds.indexOf(bid)
                if (idx > -1) selectedIds.splice(idx, 1)
                else selectedIds.push(bid)
                bucketWrap.innerHTML = renderBucketCards()
                bindInteractions() // 重新绑定
            }
        })

        // 绑定 Code 卡片
        const codeWrap = document.getElementById('codeCardWrap')
        codeWrap.querySelectorAll('.code-card').forEach(card => {
            card.onclick = () => {
                const code = card.dataset.code
                const idx = selectedCodes.indexOf(code)
                if (idx > -1) selectedCodes.splice(idx, 1)
                else selectedCodes.push(code)
                codeWrap.innerHTML = renderCodeCards()
                bindInteractions() // 重新绑定
            }
        })
    }

    setTimeout(() => {
        bindInteractions()
    }, 80)
}

function openRoleModal(user) {
  closeDropdown()
  const currentRole = String(user.role)

  const modal = new PopupModal({
    title: '修改用户角色',
    content: `
      <div class="py-1">
        <p class="text-sm text-slate-600 dark:text-slate-300 mb-1">
          修改用户 <strong class="text-slate-900 dark:text-white">${user.username}</strong> 的角色
        </p>
        <div class="mt-3">
          <label class="field-label block mb-1.5">选择角色</label>
          <select
            name="newRole"
            class="input-modern w-full py-2.5"
          >
            <option value="1" ${currentRole === '1' ? 'selected' : ''}>管理员</option>
            <!-- 修复点：修改普通用户对应的 value 为 3，防止 400 校验错误 -->
            <option value="3" ${currentRole === '3' ? 'selected' : ''}>普通用户</option>
          </select>
        </div>
      </div>
    `,
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close(),
      },
      {
        text: '重置密码',
        type: 'default',
        callback: () => {
          handleResetPassword(user)
        },
      },
      {
        text: '保存',
        type: 'primary',
        callback: async (modal) => {
          const newRoleSelect = modal.content.querySelector('select[name="newRole"]')
          // 修复点：默认缺省也应降级为 3
          const newRole = parseInt(newRoleSelect?.value || '3')

          try {
            const res = await fetch('/api/users/updateRole', {
              method: 'POST',
              headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`,
              },
              body: JSON.stringify({ id: user.id, role: newRole }),
            })
            const result = await res.json()

            if (res.ok && result.code === 200) {
              message.success('角色更新成功')
              modal.close()
              fetchUsers()
            } else {
              message.error(result.message || '更新失败')
            }
          } catch (err) {
            console.error('更新角色失败:', err)
            message.error('网络错误，请重试')
          }
        },
      },
    ],
  })
  modal.open()
}

async function handleResetPassword(user) {
    const modal = new PopupModal({
    title: '重置用户密码',
    content: `
      <div class="flex items-start gap-3">
        <div class="shrink-0 w-10 h-10 flex items-center justify-center rounded-full bg-red-100 dark:bg-red-900/30">
          <i class="ri-error-warning-fill text-red-500 text-xl"></i>
        </div>
        <div>
          <p class="text-sm text-slate-700 dark:text-slate-200">
            你确定要重置用户 <strong>${user.username}</strong> 的密码吗？
          </p>
          <p class="mt-1.5 text-xs text-slate-500 dark:text-slate-400">
            此操作无法撤销，请谨慎操作。
          </p>
        </div>
      </div>
    `,
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => modal.close(),
      },
      {
        text: '确定',
        type: 'primary',
        callback: async () => {
          await resetPassword(user)
        },
      }
    ]
    })
    modal.open()
}

const resetPassword = async (user) => { 
  closeDropdown()
  try {
    const res = await fetch(`/api/users/resetPassword/${user.id}`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`,
      },
    })
    const result = await res.json()

    if (res.ok && result.code === 200 && result.data?.new_password) {
      const newPassword = result.data.new_password
      const modal = new PopupModal({
        title: '密码重置成功',
        content: `
          <div class="py-1">
            <p class="text-sm text-slate-600 dark:text-slate-300 mb-3">
              用户 <strong class="text-slate-900 dark:text-white">${user.username}</strong> 的新密码已生成，请妥善保管。
            </p>
            <div>
              <label class="field-label block mb-1.5">新密码</label>
              <div class="flex items-center gap-2">
                <code class="flex-1 rounded-xl border border-slate-200 dark:border-white/10 bg-slate-50 dark:bg-slate-800 px-3.5 py-2.5 text-sm font-mono tracking-wider break-all text-slate-900 dark:text-white">
                  ${newPassword}
                </code>
                <button
                  id="copy-pwd-btn"
                  class="soft-button min-h-0 py-2.5 px-3 shrink-0"
                  title="复制密码"
                >
                  <i class="ri-file-copy-line text-base"></i>
                </button>
              </div>
            </div>
          </div>
        `,
        buttons: [
          {
            text: '知道了',
            type: 'primary',
            callback: (modal) => modal.close(),
          },
        ],
      })
      modal.open()

      await nextTick()
      const copyBtn = document.getElementById('copy-pwd-btn')
      if (copyBtn) {
        copyBtn.addEventListener('click', async () => {
          try {
            await navigator.clipboard.writeText(newPassword)
            message.success('密码已复制到剪贴板')
            copyBtn.innerHTML = '<i class="ri-check-line text-base text-emerald-500"></i>'
            setTimeout(() => {
              copyBtn.innerHTML = '<i class="ri-file-copy-line text-base"></i>'
            }, 2000)
          } catch {
            message.warning('复制失败，请手动复制')
          }
        })
      }

      fetchUsers()
    } else {
      message.error(result.message || '重置失败')
    }
  } catch (err) {
    console.error('重置密码失败:', err)
    message.error('网络错误，请重试')
  }
}

const GetBuckets = async () => {
  try {
    const response = await fetch('/api/buckets', {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    });
    const result = await response.json();
    if (response.ok && result.code === 200) {
      buckets.value = result.data;
    } else {
      message.error(result.message || '获取存储列表失败');
    }
  } catch (error) {
    console.error('获取存储列表失败:', error);
    message.error('获取存储列表失败，请稍后重试');
  }
};

const getStorageMode = async () => {
  try {
    const response = await fetch('/api/uploadConfig', {
      headers: {
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
      }
    })
    const result = await response.json()
    multiStorageSync.value = response.ok && result.code === 200 && result.data?.multi_storage_sync === true
  } catch (error) {
    console.error('获取多存储模式失败:', error)
    multiStorageSync.value = false
  }
}

onMounted(async () => {
  document.addEventListener('click', handleClickOutside)
  await Promise.all([getStorageMode(), GetBuckets()])
  fetchUsers()
})

onUnmounted(() => {
  if (searchTimer.value) clearTimeout(searchTimer.value)
  document.removeEventListener('click', handleClickOutside)
  dropdownRefs.value.clear()
})
</script>