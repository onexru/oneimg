<template>
    <div class="page-shell text-gray-800 dark:text-gray-200">
        <section class="page-header border-b border-slate-200/70 pb-4 dark:border-white/10">
            <div>
                <h1 class="page-title">系统设置</h1>
            </div>
            <div v-if="settingsState === 'ready' || settingsState === 'readonly'" class="grid w-full gap-2.5 sm:w-auto sm:grid-cols-2">
                <div class="stat-tile min-w-0">
                    <p class="text-xs uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">默认存储</p>
                    <p class="mt-2 text-base font-semibold text-slate-900 dark:text-white">{{ presetBuckets.find(bucket => bucket.id == systemSettings.default_storage)?.name || '未选择' }}</p>
                </div>
                <div class="stat-tile min-w-0">
                    <p class="text-xs uppercase tracking-[0.24em] text-slate-400 dark:text-slate-500">API 状态</p>
                    <p class="mt-2 text-base font-semibold text-slate-900 dark:text-white">{{ systemSettings.start_api ? '已启用' : '未启用' }}</p>
                </div>
            </div>
        </section>

        <section v-if="settingsNotice" class="section-card" role="status">
            <p class="text-sm text-secondary">{{ settingsNotice }}</p>
            <router-link v-if="settingsState === 'unauthenticated'" to="/login" class="soft-button mt-3">重新登录</router-link>
            <button v-if="canReadSettings && settingsState !== 'loading'" type="button" class="soft-button mt-3" @click="fetchSystemSettings">重新加载</button>
        </section>
        <section v-if="settingsTabs.length" class="section-card p-2 sm:p-2.5" aria-label="系统设置分类">
            <div class="flex gap-1.5 overflow-x-auto pb-1" role="tablist" aria-label="设置分类">
                <button
                    v-for="(tab, index) in settingsTabs"
                    :id="`settings-tab-${tab.key}`"
                    :key="tab.key"
                    type="button"
                    role="tab"
                    aria-controls="settings-tab-content"
                    :aria-selected="activeSettingsTab === tab.key"
                    :tabindex="activeSettingsTab === tab.key ? 0 : -1"
                    class="inline-flex min-h-10 shrink-0 items-center gap-2 rounded-xl px-3.5 py-2 text-sm font-medium transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 dark:focus-visible:ring-offset-slate-900"
                    :class="activeSettingsTab === tab.key
                        ? 'bg-slate-900 text-white shadow-sm dark:bg-white dark:text-slate-900'
                        : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900 dark:text-slate-300 dark:hover:bg-white/5 dark:hover:text-white'"
                    @click="activeSettingsTab = tab.key"
                    @keydown="handleSettingsTabKeydown($event, index)"
                >
                    <i :class="tab.icon" aria-hidden="true"></i>
                    <span>{{ tab.label }}</span>
                </button>
            </div>
        </section>

        <!-- 主要内容 -->
        <div v-if="settingsTabs.length" id="settings-tab-content" class="pb-8 md:pb-10" role="tabpanel" :aria-labelledby="`settings-tab-${activeSettingsTab}`">
            <div class="grid gap-4 md:gap-5 xl:grid-cols-[minmax(0,1.1fr)_minmax(340px,0.9fr)]">
                
                <!-- 系统配置卡片 (左侧/右侧详情) -->
                <SettingsFieldsPanel />

                <!-- 开关与特定面板 (右侧/左侧开关) -->
                <div class="order-2 md:order-1 w-full p-0 mx-auto" :class="{ 'xl:col-span-2': activeSettingsTab === 'seo' }">
                    
                    <!-- SEO设置独立面板 -->
                    <SettingsSeoPanel />

                    <!-- 安全与登录面板 (OIDC/CAS) -->
                    <SettingsIdentityPanel />

                    <!-- 通用开关面板 -->
                    <SettingsSwitchPanel />
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, provide } from 'vue'
import { settingsContext, settingsAccess, visibleSettingsTabs } from '@/utils/settingsAccess.js'
import { createRandomGraphSettings } from '@/utils/randomGraphSettings.js'
import { readApiResponse } from '@/utils/apiFeedback.js'
import Message from '@/utils/message.js'
import { createDialogScope } from '@/utils/dialogScope.js'
const { Dialog: PopupModal, dispose: disposeDialogs } = createDialogScope()
import SettingsFieldsPanel from '@/components/settings/SettingsFieldsPanel.vue'
import SettingsSeoPanel from '@/components/settings/SettingsSeoPanel.vue'
import SettingsIdentityPanel from '@/components/settings/SettingsIdentityPanel.vue'
import SettingsSwitchPanel from '@/components/settings/SettingsSwitchPanel.vue'


const initialSettings = ref({})
const activeSettingsTab = ref('storage')
const systemSettings = ref({})
const presetBuckets = ref([])
const mySettingPerms = ref([])
const settingsNotice = ref('正在加载设置…')
// Permission authority is the Cookie-authenticated backend, never a stale localStorage snapshot.
const settingsState = ref('loading')
const canReadSettings = computed(() => !['denied', 'unauthenticated'].includes(settingsState.value))
const generatedApiToken = ref('')
const tokenGenerating = ref(false)

const settingsTabs = computed(() => visibleSettingsTabs(mySettingPerms.value));

const activeSettingsTabLabel = computed(() => {
    return settingsTabs.value.find(t => t.key === activeSettingsTab.value)?.label || ''
})

const publicDomainStorageTypes = ['s3']

const currentDefaultBucket = computed(() => {
    return presetBuckets.value.find(bucket => String(bucket.id) === String(systemSettings.value?.default_storage))
})

const supportsPublicImageDomain = computed(() => {
    return publicDomainStorageTypes.includes(currentDefaultBucket.value?.type)
})

const hasPublicImageDomain = computed(() => !!systemSettings.value.public_image_domain)
const publicImageDomainUnavailable = computed(() => {
    return !supportsPublicImageDomain.value
})
const publicImageDomainInputDisabled = computed(() => {
    return (systemSettings.value.encrypted_storage || publicImageDomainUnavailable.value) && !hasPublicImageDomain.value
})
const publicImageDomainHint = computed(() => {
    if (systemSettings.value.encrypted_storage) {
        return '加密存储已开启，图片必须通过程序解密后访问，不能使用存储服务直链域名。'
    }
    if (!supportsPublicImageDomain.value) {
        return '此项仅用于 S3；Cloudflare R2 请在对应存储源中填写自定义 CDN / 访问域名。'
    }
    if (hasPublicImageDomain.value) {
        return '启用后图片链接将直接使用该域名，图片水印文本、来源白名单等依赖系统代理的功能不会生效。'
    }
    return '填写 S3 绑定的直链域名后，返回给用户的图片链接会直接使用该域名。'
})

let settingsRequestActive = false
const fetchSystemSettings = async () => {
    if (settingsRequestActive) return
    settingsRequestActive = true
    settingsState.value = 'loading'
    settingsNotice.value = '正在加载设置…'
    try {
        const response = await fetch('/api/settings/get', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({}) 
        })
        const res = await response.json()
        const access = settingsAccess(response.status, res)
        settingsState.value = access.state
        settingsNotice.value = access.notice
        if (response.ok && res.code === 200) {
            mySettingPerms.value = res.setting_permissions || []
            settingsNotice.value = access.notice
            systemSettings.value = { ...(res.data || {}), api_token: '' }
            initialSettings.value = JSON.parse(JSON.stringify(res.data || {}))
            if (settingsTabs.value.length > 0 && !settingsTabs.value.find(t => t.key === activeSettingsTab.value)) {
                activeSettingsTab.value = settingsTabs.value[0].key
            }
        } else {
            mySettingPerms.value = []
            settingsNotice.value = access.notice
            console.error('获取设置失败:', res.message)
            Message.error(res.message || '获取设置失败')
        }
    } catch (err) {
        mySettingPerms.value = []
        systemSettings.value = {}
        settingsState.value = 'error'
        settingsNotice.value = '网络请求失败，请重新加载设置'
        console.error('请求错误:', err)
        Message.error('网络请求失败，请重新加载设置')
    } finally { settingsRequestActive = false }
}

const handleFieldBlur = async (key, value) => {
    if (JSON.stringify(value) === JSON.stringify(initialSettings.value[key])) {
        return
    }
    try {
        const response = await fetch('/api/settings/update', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key, value })
        })
        const res = await response.json()
        
        if (response.ok && res.code === 200) {
            initialSettings.value[key] = value
            Message.success('保存成功')
        } else {
            Message.error(res.message || '保存失败')
            fetchSystemSettings()
        }
    } catch (err) {
        Message.error('保存失败')
    }
}
const handleSelectChange = (key, value) => {
    handleFieldBlur(key, value)
}
const handleSwitchChange = (key, value) => handleFieldBlur(key, value)

const handleSettingsTabKeydown = (event, index) => {
    if (event.key === 'ArrowRight' && index < settingsTabs.value.length - 1) {
        activeSettingsTab.value = settingsTabs.value[index + 1].key
    } else if (event.key === 'ArrowLeft' && index > 0) {
        activeSettingsTab.value = settingsTabs.value[index - 1].key
    }
}

// 生成 API Token
const generateApiToken = () => {
    const modal = new PopupModal({
        title: '重新生成 API 凭据',
        content: '<p>此前 API 凭据将被撤销，新凭据仅显示一次。确定继续吗？</p>',
        buttons: [
            { text: '取消', callback: modal => modal.close() },
            { text: '重新生成', type: 'primary', callback: async modal => {
                if (tokenGenerating.value) return
                tokenGenerating.value = true
                try {
                    const response = await fetch('/api/settings/regenerate', { method: 'POST' })
                    const result = await response.json()
                    if (!response.ok || result.code !== 200 || !result.data?.api_token) throw new Error(result.message || '生成失败')
                    generatedApiToken.value = result.data.api_token
                    systemSettings.value.api_credentials = [{ scopes: result.data.scopes, expires_at: result.data.expires_at, last_used_at: null }]
                    systemSettings.value.api_token_configured = true
                    Message.success(result.message || '新凭据仅显示一次，请立即保存')
                    modal.close()
                } catch (error) { Message.error(error.message || '生成失败') }
                finally { tokenGenerating.value = false }
            } },
        ],
    })
    modal.open()
}

const generateRandomGraph = createRandomGraphSettings();

onBeforeUnmount(() => { disposeDialogs(); generateRandomGraph.dispose() })

// 6. 初始化
provide(settingsContext, { systemSettings, activeSettingsTab, activeSettingsTabLabel, presetBuckets, supportsPublicImageDomain, hasPublicImageDomain, publicImageDomainInputDisabled, publicImageDomainHint, handleFieldBlur, handleSelectChange, handleSwitchChange, generatedApiToken, tokenGenerating, generateApiToken, generateRandomGraph })
onMounted(async () => {
    await fetchSystemSettings()
    if (!canReadSettings.value || settingsState.value === 'error') return
    try {
        const response = await fetch('/api/buckets/list', { headers: { 'X-Requested-With': 'XMLHttpRequest' } })
        const result = await readApiResponse(response, '获取存储列表失败')
        presetBuckets.value = Array.isArray(result.data) ? result.data : []
    } catch (error) { console.error(error) }
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
