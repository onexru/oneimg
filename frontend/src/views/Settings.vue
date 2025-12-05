<template>
    <div class="text-gray-800 dark:text-gray-200">
        <!-- 页面头部 -->
        <div class="settings-header container mx-auto px-4 py-4">
            <h1 class="page-title flex items-center text-2xl md:text-3xl font-bold">
                设置
            </h1>
            <p class="page-description text-gray-600 dark:text-gray-400 mt-2">管理您的系统设置</p>
        </div>

        <!-- 主要内容 -->
        <div class="container mx-auto px-4 pb-16">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-8">

                <!-- 系统配置卡片 -->
                <div class="order-1 md:order-2 bg-white dark:bg-gray-800 rounded-xl shadow-md overflow-hidden w-full p-0 mx-auto">
                    <div class="panel-content p-6 md:p-8">
                        <h2 class="panel-title flex items-center text-xl font-semibold mb-8">
                            <span class="panel-icon mr-2 text-2xl">
                                <i class="ri-list-settings-line"></i>
                            </span>
                            系统配置
                        </h2>
                        
                        <div class="account-form space-y-6">
                            <!-- TG Bot Token：失去焦点保存 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="tg_bot_token">
                                    TG Bot Token
                                </label>
                                <input 
                                    id="tg_bot_token"
                                    v-model="systemSettings.tg_bot_token"
                                    type="text" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="请输入TG Bot Token"
                                    @blur="handleFieldBlur('tg_bot_token', systemSettings.tg_bot_token)"
                                />
                            </div>
                            
                            <!-- TG 通知接收者：失去焦点保存 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="tg_receivers">
                                    TG 通知接收者
                                </label>
                                <input 
                                    id="tg_receivers"
                                    v-model="systemSettings.tg_receivers"
                                    type="text" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="接收通知的TG用户ID"
                                    @blur="handleFieldBlur('tg_receivers', systemSettings.tg_receivers)"
                                />
                            </div>
                            
                            <!-- TG 通知文本：失去焦点保存 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="tg_notice_text">
                                    TG 通知文本
                                </label>
                                <input 
                                    id="tg_notice_text"
                                    v-model="systemSettings.tg_notice_text"
                                    type="text" 
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    placeholder="自定义TG通知文本"
                                    @blur="handleFieldBlur('tg_notice_text', systemSettings.tg_notice_text)"
                                />
                                <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">
                                    默认模板：{username} {date} 上传了图片 {filename}，存储容器[{StorageType}]
                                </div>
                            </div>
                            
                            <!-- 存储类型：下拉框变更保存 -->
                            <div class="setting-group">
                                <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="storage_type">
                                    存储类型
                                </label>
                                <select 
                                    id="storage_type"
                                    v-model="systemSettings.storage_type"
                                    class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                    @change="handleSelectChange('storage_type', systemSettings.storage_type)"
                                >
                                    <option value="" disabled>请选择存储类型</option>
                                    <option value="default">本地存储</option>
                                    <option value="s3">S3</option>
                                    <option value="r2">R2</option>
                                    <option value="webdav">WebDav</option>
                                </select>
                            </div>
                            
                            <!-- S3/R2配置：失去焦点保存 -->
                            <div v-if="['s3', 'r2'].includes(systemSettings.storage_type)" class="space-y-4 pt-2 border-t border-gray-200 dark:border-gray-700">
                                <h3 class="font-bold text-sm text-gray-800 dark:text-gray-200">S3/R2 配置</h3>
                                
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="s3_endpoint">
                                        S3 Endpoint
                                    </label>
                                    <input 
                                        id="s3_endpoint"
                                        v-model="systemSettings.s3_endpoint"
                                        type="text" 
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="如：s3.us-west-004.backblazeb2.com"
                                        @blur="handleFieldBlur('s3_endpoint', systemSettings.s3_endpoint)"
                                    />
                                </div>
                                
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="s3_access_key">
                                        S3 AccessKey
                                    </label>
                                    <input 
                                        id="s3_access_key"
                                        v-model="systemSettings.s3_access_key"
                                        type="text" 
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="S3访问密钥ID"
                                        @blur="handleFieldBlur('s3_access_key', systemSettings.s3_access_key)"
                                    />
                                </div>
                                
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="s3_secret_key">
                                        S3 SecretKey
                                    </label>
                                    <input 
                                        id="s3_secret_key"
                                        v-model="systemSettings.s3_secret_key"
                                        type="password" 
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="S3私有访问密钥"
                                        @blur="handleFieldBlur('s3_secret_key', systemSettings.s3_secret_key)"
                                    />
                                </div>
                                
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="S3Bucket">
                                        S3 Bucket
                                    </label>
                                    <input 
                                        id="S3Bucket"
                                        v-model="systemSettings.s3_bucket"
                                        type="text" 
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="存储桶名称"
                                        @blur="handleFieldBlur('s3_bucket', systemSettings.s3_bucket)"
                                    />
                                </div>
                            </div>

                            <!-- WebDAV配置：失去焦点保存 -->
                            <div v-if="systemSettings.storage_type === 'webdav'" class="space-y-4 pt-2 border-t border-gray-200 dark:border-gray-700"> 
                                <h3 class="font-bold text-sm text-gray-800 dark:text-gray-200">WebDav 配置</h3>

                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="webdav_url">
                                        WebDav URL
                                    </label>
                                    <input 
                                        id="webdav_url"
                                        v-model="systemSettings.webdav_url"
                                        type="text"
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="请填写 WebDav 地址"
                                        @blur="handleFieldBlur('webdav_url', systemSettings.webdav_url)"
                                    />
                                </div>
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="webdav_user">
                                        WebDav 用户名
                                    </label>
                                    <input 
                                        id="webdav_user"
                                        v-model="systemSettings.webdav_user"
                                        type="text"
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="请填写 WebDav 用户名"
                                        @blur="handleFieldBlur('webdav_user', systemSettings.webdav_user)"
                                    />
                                </div>
                                <div class="setting-group"> 
                                    <label class="setting-label block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1" for="webdav_pass">
                                        WebDav 密码
                                    </label>
                                    <input 
                                        id="webdav_pass"
                                        v-model="systemSettings.webdav_pass"
                                        type="password"
                                        class="setting-input w-full px-4 py-2.5 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 focus:ring-2 focus:ring-primary focus:border-primary dark:focus:ring-primary/70 dark:focus:border-primary/70 transition-colors outline-none"
                                        placeholder="请填写 WebDav 密码"
                                        @blur="handleFieldBlur('webdav_pass', systemSettings.webdav_pass)"
                                    />
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- 系统设置卡片（开关部分不变） -->
                <div class="order-2 md:order-1 bg-white dark:bg-gray-800 rounded-xl shadow-md overflow-hidden w-full p-0 mx-auto">
                    <div class="panel-content p-6 md:p-8">
                        <h2 class="panel-title flex items-center text-xl font-semibold mb-8">
                            <span class="panel-icon mr-2 text-2xl">
                                <i class="ri-settings-2-line"></i>
                            </span>
                            系统设置
                        </h2>
                        
                        <div class="account-form space-y-6">
                            <!-- 是否保存原图 -->
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    保存原图
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.original_image"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('original_image', systemSettings.original_image)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-green-500 dark:peer-checked:bg-green-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            
                            <!-- 其他开关省略（和之前一致） -->
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    保存WEBP格式
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.save_webp"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('save_webp', systemSettings.save_webp)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-green-500 dark:peer-checked:bg-green-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    生成缩略图
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.thumbnail"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('thumbnail', systemSettings.thumbnail)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-green-500 dark:peer-checked:bg-green-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">生成缩略图，可提升后台预览速度，上传速度稍慢</div>
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    允许游客上传
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.tourist"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('tourist', systemSettings.tourist)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-green-500 dark:peer-checked:bg-green-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    启用TG通知
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.tg_notice"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('tg_notice', systemSettings.tg_notice)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-blue-500 dark:peer-checked:bg-blue-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                            <div class="mt-1 text-gray-500 dark:text-gray-400 text-xs">国内服务器不要开启TG通知</div>
                            <div class="setting-group flex items-center justify-between py-2">
                                <label class="setting-label text-sm font-medium text-gray-700 dark:text-gray-300">
                                    启用POW验证
                                </label>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.pow_verify"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('pow_verify', systemSettings.pow_verify)"
                                    >
                                    <div class="w-12 h-6 bg-gray-200 dark:bg-gray-700 rounded-full peer-checked:bg-purple-500 dark:peer-checked:bg-purple-600 switch-transition switch-antialias"></div>
                                    <div class="absolute left-1 top-1 bg-white dark:bg-gray-200 w-4 h-4 rounded-full switch-transition switch-antialias peer-checked:translate-x-6"></div>
                                </label>
                            </div>
                        </div>
                    </div>
                </div>

            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, onMounted, reactive } from 'vue'
import { useRouter } from 'vue-router'
import message from '@/utils/message.js'

const router = useRouter()

const systemSettings = reactive({
    id: 1,
    original_image: false,
    save_webp: false,
    thumbnail: false,
    tourist: false,
    tg_notice: false,
    pow_verify: false,
    tg_bot_token: '',
    tg_receivers: '',
    tg_notice_text: '',
    storage_type: '',
    s3_endpoint: '',
    s3_access_key: '',
    s3_secret_key: '',
    s3_bucket: '',
    webdav_url: '',
    webdav_user: '',
    webdav_pass: '',
})

// 加载状态
const isUpdating = ref(false)
let debounceTimer = null

// 统一请求头配置（复用）
const getRequestHeaders = () => {
    return {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('authToken')}`
    }
}

const saveSetting = async (key, value) => {
    clearTimeout(debounceTimer)
    debounceTimer = setTimeout(async () => {
        try {
            if (isUpdating.value) return
            isUpdating.value = true

            const response = await fetch('/api/settings/update', {
                method: 'POST',
                headers: getRequestHeaders(),
                body: JSON.stringify({
                    key: key,
                    value: value
                })
            })
            
            const result = await response.json()
            
            if (response.ok && result.code === 200) {
                message.success(`更新成功`)
            } else {
                message.error(`更新失败：${result.message || '未知错误'}`)
            }
        } catch (error) {
            console.error(`保存失败:`, error)
            message.error(`更新失败：网络异常`)
        } finally {
            isUpdating.value = false
        }
    }, 300)
}

// 开关状态变更统一处理方法
const handleSwitchChange = (key, value) => {
    saveSetting(key, value)
}

// 输入框失去焦点处理
const handleFieldBlur = (key, value) => {
    saveSetting(key, value)
}

// 下拉框变更处理
const handleSelectChange = (key, value) => {
    saveSetting(key, value)
}

// 获取系统设置
const getSettings = async () => { 
    try {
        const response = await fetch('/api/settings/get', {
            method: 'GET',
            headers: getRequestHeaders(),
        })
        
        if (!response.ok) {
            throw new Error(`请求失败：${response.status}`)
        }
        
        const result = await response.json()
        
        if (result.code === 200 && result.data) {
            Object.assign(systemSettings, result.data)
        } else {
            message.error(result.message || '获取设置失败：无数据')
        }
    } catch (error) {
        console.error('获取设置失败:', error)
        message.error(error.message || '获取设置失败：网络异常')
    }
}

onMounted(() => {
    getSettings()
})
</script>

<style scoped>
.switch-transition {
    transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}
.switch-antialias {
    transform: translateZ(0);
}
</style>