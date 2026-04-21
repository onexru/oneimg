<template>
    <div class="page-shell text-gray-800 dark:text-gray-200">
        <section class="page-header">
            <div>
                <h1 class="page-title">账户设置</h1>
            </div>
        </section>

        <!-- 主要内容 -->
        <div class="pb-16">
            <div class="grid grid-cols-[repeat(auto-fit,minmax(320px,1fr))] gap-6">

                <div class="section-card mx-auto overflow-hidden w-full m-4">
                    <div class="panel-content p-6 md:p-8">
                        <h2 class="panel-title flex items-center text-xl font-semibold mb-8">
                            <span class="panel-icon mr-2 text-2xl">
                                <i class="ri-shield-user-line"></i>
                            </span>
                            账户设置
                        </h2>
                        
                        <!-- 账户修改表单 -->
                        <form @submit.prevent="updateAccount" class="account-form space-y-6">
                            <!-- 新用户名 -->
                            <div class="setting-group">
                                <label 
                                    class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" 
                                    for="newUsername"
                                >
                                    新用户名（留空则不修改）
                                </label>
                                <input 
                                    id="newUsername"
                                    v-model="accountForm.newUsername"
                                    type="text" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="留空则不修改用户名"
                                    minlength="3"
                                    maxlength="20"
                                />
                            </div>
                            
                            <!-- 当前密码 -->
                            <div class="setting-group">
                                <label 
                                    class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" 
                                    for="currentPassword"
                                >
                                    当前密码 <span class="text-red-500">*</span>
                                </label>
                                <input 
                                    id="currentPassword"
                                    v-model="accountForm.currentPassword"
                                    type="password" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="请输入当前密码以确认修改"
                                    required
                                />
                            </div>
                            
                            <!-- 新密码 -->
                            <div class="setting-group">
                                <label 
                                    class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" 
                                    for="newPassword"
                                >
                                    新密码（留空则不修改）
                                </label>
                                <input 
                                    id="newPassword"
                                    v-model="accountForm.newPassword"
                                    type="password" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="留空则不修改密码（至少6位）"
                                    minlength="6"
                                />
                            </div>
                            
                            <!-- 确认新密码 -->
                            <div class="setting-group">
                                <label 
                                    class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" 
                                    for="confirmPassword"
                                >
                                    确认新密码
                                </label>
                                <input 
                                    id="confirmPassword"
                                    v-model="accountForm.confirmPassword"
                                    type="password" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="请再次输入新密码"
                                />
                            </div>
                            
                            <!-- 提交按钮 -->
                            <div class="setting-group pt-2">
                                <button 
                                    type="submit" 
                                    :disabled="isUpdatingAccount"
                                    class="setting-btn accent w-full py-3 px-6 bg-primary hover:bg-primary/90 text-white font-medium rounded-lg transition-colors flex items-center justify-center gap-2 focus:ring-2 focus:ring-primary/50 focus:outline-none"
                                >
                                    <span v-if="isUpdatingAccount" class="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin"></span>
                                    <span>保存修改</span>
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
                <!-- Github版本卡片 -->
                <div class="section-card mx-auto overflow-hidden w-full m-4">
                    <div class="panel-content p-6 md:p-8">
                        <div class="flex items-start justify-between gap-4 border-b border-slate-200/70 pb-4 dark:border-white/10">
                            <div>
                                <h2 class="panel-title flex items-center text-xl font-semibold">
                            <span class="panel-icon mr-2 text-2xl">
                                <i class="ri-github-fill"></i>
                            </span>
                            版本信息
                                </h2>
                            </div>
                            <span class="inline-flex shrink-0 items-center rounded-full border border-slate-200/80 bg-slate-50 px-3 py-1 text-xs text-slate-500 dark:border-white/10 dark:bg-slate-950 dark:text-slate-400">GitHub Release</span>
                        </div>
                        <!-- 加载中状态 -->
                        <div v-if="isLoadingVersion" class="py-10 text-center">
                            <span class="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin inline-block"></span>
                            <p class="mt-3 text-gray-600 dark:text-gray-400">正在检查最新版本...</p>
                        </div>
                        <!-- 版本加载成功 -->
                        <div v-else-if="latestVersion" class="space-y-4 pt-5">
                            <div class="grid gap-3 sm:grid-cols-3">
                                <div class="rounded-[18px] border border-slate-200/80 bg-slate-50 px-4 py-3 dark:border-white/10 dark:bg-slate-950">
                                    <p class="text-xs uppercase tracking-[0.18em] text-slate-400 dark:text-slate-500">版本号</p>
                                    <p class="mt-2 text-2xl font-semibold text-primary">{{ latestVersion.tag_name }}</p>
                                </div>
                                <div class="rounded-[18px] border border-slate-200/80 bg-slate-50 px-4 py-3 dark:border-white/10 dark:bg-slate-950">
                                    <p class="text-xs uppercase tracking-[0.18em] text-slate-400 dark:text-slate-500">发布名称</p>
                                    <p class="mt-2 truncate text-sm font-medium text-slate-700 dark:text-slate-200">{{ latestVersion.name || '最新稳定版本' }}</p>
                                </div>
                                <div class="rounded-[18px] border border-slate-200/80 bg-slate-50 px-4 py-3 dark:border-white/10 dark:bg-slate-950">
                                    <p class="text-xs uppercase tracking-[0.18em] text-slate-400 dark:text-slate-500">发布时间</p>
                                    <p class="mt-2 text-sm font-medium text-slate-700 dark:text-slate-200">{{ formatReleaseDate(latestVersion.published_at) }}</p>
                                </div>
                            </div>
                            <div class="rounded-[20px] border border-dashed border-slate-300/90 bg-slate-50/80 p-4 dark:border-white/10 dark:bg-slate-950/70">
                                <div class="flex items-center justify-between gap-3">
                                    <p class="text-sm font-medium text-slate-800 dark:text-slate-100">更新日志</p>
                                    <a 
                                        :href="latestVersion.html_url" 
                                        target="_blank" 
                                        rel="noopener noreferrer"
                                        class="inline-flex items-center rounded-full border border-primary/20 px-3 py-1.5 text-xs font-medium text-primary transition-colors hover:bg-primary hover:text-white"
                                    >
                                        前往更新
                                    </a>
                                </div>
                                <div class="mt-3 max-h-[320px] overflow-auto rounded-[16px] bg-white px-4 py-3 text-sm leading-6 text-slate-600 dark:bg-slate-900 dark:text-slate-300">
                                    <pre class="whitespace-pre-wrap break-words font-sans">{{ latestVersion.body || '暂无更新日志' }}</pre>
                                </div>
                            </div>
                        </div>
                        <!-- 加载失败状态 -->
                        <div v-else class="py-10 text-center text-gray-600 dark:text-gray-400">
                            <i class="ri-error-warning-line text-2xl mb-2"></i>
                            <p>版本信息加载失败，请稍后重试</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import message from '@/utils/message.js'

const router = useRouter()

// 表单数据
const accountForm = ref({
    newUsername: '',
    currentPassword: '',
    newPassword: '',
    confirmPassword: ''
})

// 加载状态
const isUpdatingAccount = ref(false)
const isLoadingVersion = ref(true)
const latestVersion = ref(null)

// 格式化发布时间
const formatReleaseDate = (dateStr) => {
    if (!dateStr) return ''
    const date = new Date(dateStr)
    return date.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' })
}

// 获取Github最新版本信息
const getLatestVersion = async () => {
    try {
        const res = await fetch('https://api.github.com/repos/onexru/oneimg/releases/latest')
        if (res.ok) {
            const data = await res.json()
            latestVersion.value = data
        } else {
            throw new Error('版本接口请求失败')
        }
    } catch (err) {
        message.error('版本信息加载失败')
        console.error('版本请求异常：', err)
    } finally {
        isLoadingVersion.value = false
    }
}

// 页面挂载时自动请求版本信息
onMounted(() => {
    getLatestVersion()
})

// 更新账户信息
const updateAccount = async () => {
    const { newUsername, currentPassword, newPassword, confirmPassword } = accountForm.value
    
    // 检查是否有任何修改
    const hasUsernameChange = newUsername && newUsername.trim() !== ''
    const hasPasswordChange = newPassword && newPassword.trim() !== ''
    
    if (!hasUsernameChange && !hasPasswordChange) {
        message.error('请输入要修改的用户名或密码')
        return
    }
    
    // 验证用户名（如果要修改）
    if (hasUsernameChange) {
        if (newUsername.length < 3) {
            message.error('用户名长度至少为3位')
            return
        }
        
        if (newUsername.length > 20) {
            message.error('用户名长度不能超过20位')
            return
        }
    }
    
    // 验证密码（如果要修改）
    if (hasPasswordChange) {
        if (newPassword.length < 6) {
            message.error('新密码长度至少为6位')
            return
        }
        
        if (newPassword !== confirmPassword) {
            message.error('两次输入的新密码不一致')
            return
        }
    }
    
    // 验证当前密码
    if (!currentPassword) {
        message.error('请输入当前密码以确认修改')
        return
    }
    
    try {
        isUpdatingAccount.value = true
        
        const response = await fetch('/api/account/change', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${localStorage.getItem('authToken')}`
            },
            body: JSON.stringify({
                new_username: newUsername,
                current_password: currentPassword,
                new_password: newPassword
            })
        })
        
        const result = await response.json()
        
        if (!response.ok || !result.success) {
            // 未授权处理
            if (response.status === 401) {
                localStorage.removeItem('authToken')
                router.push('/login')
                return message.error('登录已过期，请重新登录')
            }
            throw new Error(result.message || '修改失败')
        }
        
        message.success('修改成功')

        // 清空表单
        accountForm.value = {
            newUsername: '',
            currentPassword: '',
            newPassword: '',
            confirmPassword: ''
        }
        
        // 刷新页面
        setTimeout(() => {
            window.location.reload();
        }, 1000)

    } catch (error) {
        message.error(error.message || '更新失败')
    } finally {
        isUpdatingAccount.value = false
    }
}
</script>
