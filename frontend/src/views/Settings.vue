<template>
    <div class="page-shell text-gray-800 dark:text-gray-200">
        <section v-if="settingsNotice" class="surface" role="status">
            <p class="text-sm text-secondary">{{ settingsNotice }}</p>
            <router-link v-if="settingsState === 'unauthenticated'" to="/login" class="soft-button mt-3">重新登录</router-link>
            <button v-if="canReadSettings && settingsState !== 'loading'" type="button" class="soft-button mt-3" @click="fetchSystemSettings">重新加载</button>
        </section>

        <!-- 分类页签：吸顶跟随。
             设置页内容动辄一两千像素（安全页的 OIDC/CAS），页签滚出视野后
             就只能靠浏览器回顶，切换分类非常别扭，所以让它贴在顶栏下方。
             右侧原本单独一行的「默认存储 / API 状态」统计也并进来，
             全局上下文跟着页签一起移动，省掉一整条 36px 高的空行。 -->
        <section
            v-if="settingsTabs.length"
            class="settings-tabbar surface sticky top-[var(--app-header-height-mobile)] z-20 md:top-[var(--app-header-height)]"
            aria-label="系统设置分类"
        >
            <div class="flex min-w-0 flex-1 items-center gap-1.5 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden" role="tablist" aria-label="设置分类">
                <button
                    v-for="(tab, index) in settingsTabs"
                    :id="`settings-tab-${tab.key}`"
                    :key="tab.key"
                    type="button"
                    role="tab"
                    aria-controls="settings-tab-content"
                    :aria-selected="activeSettingsTab === tab.key"
                    :tabindex="activeSettingsTab === tab.key ? 0 : -1"
                    class="settings-tab inline-flex min-h-9 shrink-0 items-center gap-1.5 px-3 py-1.5 text-sm font-medium"
                    :class="activeSettingsTab === tab.key ? 'settings-tab-active' : ''"
                    @click="applyTab(tab.key)"
                    @keydown="handleSettingsTabKeydown($event, index)"
                >
                    <i :class="tab.icon" aria-hidden="true"></i>
                    <span>{{ tab.label }}</span>
                </button>
            </div>

            <div v-if="settingsState === 'ready' || settingsState === 'readonly'" class="hidden shrink-0 items-center gap-2 lg:flex">
                <span class="stat-inline">默认存储 <strong class="max-w-[9rem] truncate">{{ presetBuckets.find(bucket => bucket.id == systemSettings.default_storage)?.name || '未选择' }}</strong></span>
                <span class="stat-inline">API <strong>{{ systemSettings.start_api ? '已启用' : '未启用' }}</strong></span>
            </div>
        </section>

        <!-- 主要内容。
             storage/image/notifications/api 是「表单 + 开关」两栏：左侧表单字段多、
             需要输入宽度；右侧每个开关只有一句说明，窄列足够。
             改版前是反过来的（表单 511px / 开关 625px），输入框被挤得比说明文字还窄。
             seo/system 单列铺满（内容本身就是一整列，分栏反而空）。 -->
        <div v-if="settingsTabs.length" id="settings-tab-content" role="tabpanel" :aria-labelledby="`settings-tab-${activeSettingsTab}`">
            <div :class="isSingleColumnTab
                ? 'flex flex-col gap-4 md:gap-5'
                : 'grid gap-4 md:gap-5 xl:grid-cols-[minmax(0,1.55fr)_minmax(300px,1fr)]'">

                <!-- 系统配置表单 -->
                <SettingsFieldsPanel />

                <!-- 面板纵向排列用 flex + gap，而不是每个面板自带 mb：
                     面板靠 v-show 切换，display:none 的元素仍占用 DOM 兄弟位，
                     用 space-y 或逐个 mb 都会让第一块可见面板莫名带上顶部间距，
                     或让最后一块留下多余空白。gap 只作用于生成盒子的子项。 -->
                <div class="flex w-full min-w-0 flex-col gap-4 md:gap-5">

                    <!-- 系统：版本信息。没有对应的配置表单，单列铺满即可 -->
                    <SettingsVersionPanel v-if="activeSettingsTab === 'system'" />

                    <!-- SEO设置独立面板 -->
                    <SettingsSeoPanel />

                    <!-- 安全页两张卡纵向排列：
                         改版前是左右二分，但两块高度差近 1000px（准入策略 482 vs
                         身份提供方 1473），左栏下方整片空白；而并排会各自压到 420px 宽，
                         Turnstile 的密钥输入框挤成一团。改为全宽堆叠，由各面板内部
                         自己分两列，字段宽约 500px，既够填长地址也不会拉成 1100px。 -->
                    <SettingsSecurityPanel />
                    <SettingsIdentityPanel />

                    <!-- 通用开关面板 -->
                    <SettingsSwitchPanel />
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, provide, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { settingsContext, settingsAccess, visibleSettingsTabs } from '@/utils/settingsAccess.js'
import { createRandomGraphSettings } from '@/utils/randomGraphSettings.js'
import { readApiResponse } from '@/utils/apiFeedback.js'
import { copyToClipboard } from '@/utils/clipboard.js'
import Message from '@/utils/message.js'
import { createDialogScope } from '@/utils/dialogScope.js'
const { Dialog: PopupModal, dispose: disposeDialogs } = createDialogScope()
import SettingsFieldsPanel from '@/components/settings/SettingsFieldsPanel.vue'
import SettingsSeoPanel from '@/components/settings/SettingsSeoPanel.vue'
import SettingsIdentityPanel from '@/components/settings/SettingsIdentityPanel.vue'
import SettingsSecurityPanel from '@/components/settings/SettingsSecurityPanel.vue'
import SettingsSwitchPanel from '@/components/settings/SettingsSwitchPanel.vue'
import SettingsVersionPanel from '@/components/settings/SettingsVersionPanel.vue'


const initialSettings = ref({})
const route = useRoute()
const router = useRouter()
// 初始值取自 URL，刷新/分享链接后能回到同一分类
const activeSettingsTab = ref(typeof route.query.tab === 'string' ? route.query.tab : 'storage')
const systemSettings = ref({})
const presetBuckets = ref([])
const mySettingPerms = ref([])
const settingsNotice = ref('正在加载设置…')
// Permission authority is the Cookie-authenticated backend, never a stale localStorage snapshot.
const settingsState = ref('loading')
const canReadSettings = computed(() => !['denied', 'unauthenticated'].includes(settingsState.value))
const generatedApiToken = ref('')
const tokenGenerating = ref(false)
// 复制成功只给 2 秒反馈，避免按钮一直停在「已复制」
const tokenCopied = ref(false)
let tokenCopiedTimer

const settingsTabs = computed(() => visibleSettingsTabs(mySettingPerms.value));

// security 外层走单列（它自带「验证 | 登录」二分），seo/system 内容本身就是一整列。
const SINGLE_COLUMN_TABS = ['security', 'seo', 'system'];
const isSingleColumnTab = computed(() => SINGLE_COLUMN_TABS.includes(activeSettingsTab.value));

const activeSettingsTabLabel = computed(() => {
    return settingsTabs.value.find(t => t.key === activeSettingsTab.value)?.label || ''
})

// 窄屏页签栏是横向滚动的。通过 ?tab=system 直达时选中项可能落在可视区外
// （实测 390px 宽下 scrollLeft 仍是 0，选中项在 x=626 完全看不见），
// 用户会以为没生效。这里把它滚进视野 —— inline:'nearest' 保证已可见时不乱动。
const revealActiveTab = () => {
    requestAnimationFrame(() => {
        const el = document.getElementById(`settings-tab-${activeSettingsTab.value}`)
        el?.scrollIntoView({ block: 'nearest', inline: 'nearest' })
    })
}

// ---- 页签与 URL 同步 ----
// 目标：切换页签写进 ?tab=xxx，刷新后停在原位，链接可直接分享给同事。
// 用 replace 而非 push：页签属于同一页面的视图状态，不该在浏览器历史里
// 堆出一串倒退记录（点十次页签要按十次返回才能离开设置页）。
// 同步时机放在「切换动作」里而不是 watch(activeSettingsTab)：
// 加载完成后有个「当前页签无权限就退回第一项」的自动修正，那属于纠正而非
// 用户导航，不该触发一次 URL 写入（否则刚进来就多一条历史/一次 replace）。
const applyTab = (key) => {
    if (!key || key === activeSettingsTab.value) return
    activeSettingsTab.value = key
    revealActiveTab()
    // 保留其它 query（如以后可能加的锚点参数），只改 tab
    const query = { ...route.query }
    if (key === 'storage') delete query.tab
    else query.tab = key
    router.replace({ query }).catch(() => {})
}

// 浏览器前进/后退或外部改 query 时，把页签跟回去
watch(() => route.query.tab, (tab) => {
    const key = typeof tab === 'string' && tab ? tab : 'storage'
    if (key !== activeSettingsTab.value) {
        activeSettingsTab.value = key
        revealActiveTab()
    }
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
            // 权限列表到手后才能判断 URL 里的 tab 是否可见（例如普通用户被分享
            // 了一个 ?tab=seo 的链接，但他没有 seo 权限）。此处用直接赋值而非
            // applyTab：这是纠正而非用户导航，不该把「被纠正后的值」再写回 URL
            // 掩盖掉对方的原始链接；地址栏保留 ?tab=seo，页面回落到第一个可见项。
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

// 键盘切换也走 applyTab，保证方向键切页签同样写入 URL，
// 并且切换后焦点跟到新页签上（roving tabindex 下不移动焦点会导致
// 再按方向键时事件源仍是旧按钮，index 对不上）。
const handleSettingsTabKeydown = (event, index) => {
    let next = null
    if (event.key === 'ArrowRight' && index < settingsTabs.value.length - 1) next = settingsTabs.value[index + 1].key
    else if (event.key === 'ArrowLeft' && index > 0) next = settingsTabs.value[index - 1].key
    if (!next) return
    event.preventDefault()
    applyTab(next)
    requestAnimationFrame(() => {
        document.getElementById(`settings-tab-${next}`)?.focus()
    })
}

// 生成 API Token。
// 首次生成没有「旧凭据会被撤销」这回事，文案和按钮都按是否已配置区分，
// 否则第一次点「生成 Token」会弹一个说「此前凭据将被撤销」的确认框，容易误解。
const generateApiToken = () => {
    const isRegenerate = !!systemSettings.value.api_token_configured
    const modal = new PopupModal({
        title: isRegenerate ? '重新生成 API 凭据' : '生成 API 凭据',
        content: isRegenerate
            ? '<p>此前 API 凭据将被撤销，新凭据仅显示一次。确定继续吗？</p>'
            : '<p>新凭据仅显示一次，请生成后立即保存。确定继续吗？</p>',
        buttons: [
            { text: '取消', callback: modal => modal.close() },
            { text: isRegenerate ? '重新生成' : '生成', type: 'primary', callback: async modal => {
                if (tokenGenerating.value) return
                tokenGenerating.value = true
                try {
                    const response = await fetch('/api/settings/regenerate', { method: 'POST' })
                    const result = await response.json()
                    if (!response.ok || result.code !== 200 || !result.data?.api_token) throw new Error(result.message || '生成失败')
                    generatedApiToken.value = result.data.api_token
                    tokenCopied.value = false
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

// 复制刚生成的凭据。本来只能靠手动选中输入框里的文本，
// 而凭据是 64 字符的十六进制串，肉眼选中极易漏字符。
const copyApiToken = async () => {
    if (!generatedApiToken.value) return
    const ok = await copyToClipboard(generatedApiToken.value)
    if (!ok) { Message.error('复制失败，请手动选中后复制'); return }
    tokenCopied.value = true
    clearTimeout(tokenCopiedTimer)
    tokenCopiedTimer = setTimeout(() => { tokenCopied.value = false }, 2000)
    Message.success('已复制到剪贴板')
}

const generateRandomGraph = createRandomGraphSettings();

onBeforeUnmount(() => { clearTimeout(tokenCopiedTimer); disposeDialogs(); generateRandomGraph.dispose() })

// 6. 初始化
provide(settingsContext, { systemSettings, activeSettingsTab, activeSettingsTabLabel, presetBuckets, supportsPublicImageDomain, hasPublicImageDomain, publicImageDomainUnavailable, publicImageDomainInputDisabled, publicImageDomainHint, handleFieldBlur, handleSelectChange, handleSwitchChange, generatedApiToken, tokenGenerating, tokenCopied, copyApiToken, generateApiToken, generateRandomGraph })
onMounted(async () => {
    await fetchSystemSettings()
    // 页签渲染出来之后再把 URL 指定的那一项滚进视野（窄屏横向滚动区）
    if (settingsTabs.value.length) revealActiveTab()
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
