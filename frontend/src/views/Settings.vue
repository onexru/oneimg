<template>
    <div class="page-shell text-gray-800 dark:text-gray-200">
        <section class="page-header border-b border-slate-200/70 pb-4 dark:border-white/10">
            <div>
                <h1 class="page-title">系统设置</h1>
            </div>
            <div class="grid w-full gap-2.5 sm:w-auto sm:grid-cols-2">
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

        <section class="section-card p-2 sm:p-2.5" aria-label="系统设置分类">
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
        <div id="settings-tab-content" class="pb-8 md:pb-10" role="tabpanel" :aria-labelledby="`settings-tab-${activeSettingsTab}`">
            <div class="grid gap-4 md:gap-5 xl:grid-cols-[minmax(0,1.1fr)_minmax(340px,0.9fr)]">
                
                <!-- 系统配置卡片 (左侧/右侧详情) -->
                <div v-if="activeSettingsTab !== 'seo'" class="order-1 md:order-2 w-full p-0 mx-auto">
                    <div class="section-card p-3.5 sm:p-4 md:p-6">
                        <h2 class="panel-title mb-4 flex items-center text-lg font-semibold sm:text-xl md:mb-5">
                            <span class="panel-icon mr-2 text-2xl"><i class="ri-list-settings-line"></i></span>
                            {{ activeSettingsTabLabel }}
                        </h2>
                        
                        <div class="account-form space-y-4 md:space-y-5">
                            <!-- ========== 上传与存储 ========== -->
                            <div v-show="activeSettingsTab === 'storage'" class="setting-group">
                                <label class="field-label" for="default_storage">系统默认存储</label>
                                <select id="default_storage" v-model="systemSettings.default_storage" class="input-modern" @change="handleSelectChange('default_storage', systemSettings.default_storage)">
                                    <option v-for="bucket in presetBuckets" :key="bucket.id" :value="bucket.id">{{ bucket.name }} ({{ bucket.type }})</option>
                                </select>
                                <div class="field-hint">选择后系统将使用该存储作为默认存储，游客仅能使用该存储</div>
                            </div>

                            <div v-show="activeSettingsTab === 'storage'" class="setting-group">
                                <label class="field-label" for="public_image_domain">
                                    图片直链域名
                                </label>
                                <input
                                    id="public_image_domain"
                                    v-model="systemSettings.public_image_domain"
                                    type="text"
                                    class="input-modern"
                                    :class="{ 'cursor-not-allowed opacity-60': publicImageDomainInputDisabled }"
                                    :disabled="publicImageDomainInputDisabled"
                                    placeholder="例如 https://img.example.com"
                                    @blur="handleFieldBlur('public_image_domain', systemSettings.public_image_domain)"
                                />
                                <div
                                    class="field-hint"
                                    :class="{ 'text-amber-600 dark:text-amber-300': publicImageDomainUnavailable || hasPublicImageDomain }"
                                >
                                    {{ publicImageDomainHint }}
                                </div>
                            </div>

                            <div v-show="activeSettingsTab === 'storage'" class="setting-group">
                                <label class="field-label" for="default_path">默认存储路径</label>
                                <input id="default_path" v-model="systemSettings.default_path" type="text" class="input-modern" placeholder="默认存储路径，默认 /uploads/{year}/{moon}" @blur="handleFieldBlur('default_path', systemSettings.default_path)" />
                                <div class="field-hint">默认上传路径，魔法变量 {year} 年 {month} 月 {day} 日 {hour} 小时 {minute} 分钟 {random} 随机 {uuid} UUID</div>
                            </div>

                            <div v-show="activeSettingsTab === 'storage'" class="setting-group">
                                <label class="field-label" for="file_name">上传文件名称</label>
                                <input id="file_name" v-model="systemSettings.file_name" type="text" class="input-modern" placeholder="上传文件名称，默认 {random}" @blur="handleFieldBlur('file_name', systemSettings.file_name)" />
                                <div class="field-hint">上传文件名称，魔法变量 {random} 随机数 {year} 年 {month} 月 {day} 日 {hour} 小时 {minute} 分钟 {second} 秒</div>
                            </div>

                            <div v-show="activeSettingsTab === 'storage'" class="setting-group">
                                <label class="field-label" for="max_file_size">允许最大上传大小</label>
                                <input id="max_file_size" v-model="systemSettings.max_file_size" type="number" class="input-modern" placeholder="允许最大上传大小" @blur="handleFieldBlur('max_file_size', systemSettings.max_file_size)" />
                                <div class="field-hint">大小单位：字节，默认10mb</div>
                            </div>

                            <div v-show="activeSettingsTab === 'storage'" class="setting-group">
                                <label class="field-label" for="allowed_types">允许上传的图片类型</label>
                                <input id="allowed_types" v-model="systemSettings.allowed_types" type="text" class="input-modern" placeholder="允许上传的图片类型" @blur="handleFieldBlur('allowed_types', systemSettings.allowed_types)" />
                            </div>

                            <!-- ========== 通知 ========== -->
                            <div v-show="activeSettingsTab === 'notifications'" class="setting-group">
                                <label class="field-label" for="tg_bot_token">TG Bot Token</label>
                                <input id="tg_bot_token" v-model="systemSettings.tg_bot_token" type="text" class="input-modern" :placeholder="systemSettings.tg_bot_token_configured ? '已配置，留空表示不修改' : '未配置，请输入 Bot Token'" @blur="handleFieldBlur('tg_bot_token', systemSettings.tg_bot_token)" />
                                <div class="field-hint">{{ systemSettings.tg_bot_token_configured ? '已配置，留空表示不修改' : '发送Telegram通知时必填' }}</div>
                            </div>
                            
                            <div v-show="activeSettingsTab === 'notifications'" class="setting-group">
                                <label class="field-label" for="tg_receivers">TG 通知接收者</label>
                                <input id="tg_receivers" v-model="systemSettings.tg_receivers" type="text" class="input-modern" placeholder="接收通知的TG用户ID" @blur="handleFieldBlur('tg_receivers', systemSettings.tg_receivers)" />
                                <div class="field-hint">发送Telegram通知时必填</div>
                            </div>
                            
                            <div v-show="activeSettingsTab === 'notifications'" class="setting-group">
                                <label class="field-label" for="tg_notice_text">TG 通知文本</label>
                                <input id="tg_notice_text" v-model="systemSettings.tg_notice_text" type="text" class="input-modern" placeholder="自定义TG通知文本" @blur="handleFieldBlur('tg_notice_text', systemSettings.tg_notice_text)" />
                                <div class="field-hint">默认模板：{username} {date} 上传了图片 {filename}，存储容器[{StorageType}]</div>
                            </div>

                            <!-- ========== 图片处理 ========== -->
                            <div v-show="activeSettingsTab === 'image'" class="setting-group">
                                <label class="field-label" for="watermark_text">图片水印文本</label>
                                <input id="watermark_text" v-model="systemSettings.watermark_text" type="text" class="input-modern" :class="{ 'cursor-not-allowed opacity-60': hasPublicImageDomain }" :disabled="hasPublicImageDomain" placeholder="图片水印文本" @blur="handleFieldBlur('watermark_text', systemSettings.watermark_text)" />
                                <div v-if="hasPublicImageDomain" class="field-hint text-amber-600 dark:text-amber-300">已配置图片直链域名，图片水印文本不会生效，请先清空图片直链域名再修改。</div>
                            </div>

                            <div v-show="activeSettingsTab === 'image'" class="setting-group">
                                <label class="field-label" for="watermark_size">图片水印大小</label>
                                <input id="watermark_size" v-model="systemSettings.watermark_size" type="text" class="input-modern" :class="{ 'cursor-not-allowed opacity-60': hasPublicImageDomain }" :disabled="hasPublicImageDomain" placeholder="图片水印大小" @blur="handleFieldBlur('watermark_size', systemSettings.watermark_size)" />
                            </div>

                            <div v-show="activeSettingsTab === 'image'" class="setting-group">
                                <label class="field-label" for="watermark_color">图片水印字体颜色</label>
                                <input id="watermark_color" v-model="systemSettings.watermark_color" type="text" class="input-modern" :class="{ 'cursor-not-allowed opacity-60': hasPublicImageDomain }" :disabled="hasPublicImageDomain" placeholder="图片水印字体颜色" @blur="handleFieldBlur('watermark_color', systemSettings.watermark_color)" />
                                <div class="field-hint">默认值为 #000000 黑色</div>
                            </div>

                            <div v-show="activeSettingsTab === 'image'" class="setting-group">
                                <label class="field-label" for="watermark_opac">图片水印透明度</label>
                                <input id="watermark_opac" v-model="systemSettings.watermark_opac" type="text" class="input-modern" :class="{ 'cursor-not-allowed opacity-60': hasPublicImageDomain }" :disabled="hasPublicImageDomain" placeholder="图片水印透明度" @blur="handleFieldBlur('watermark_opac', systemSettings.watermark_opac)" />
                                <div class="field-hint">默认值：0.5</div>
                            </div>

                            <div v-show="activeSettingsTab === 'image'" class="setting-group">
                                <label class="field-label" for="watermark_pos">图片水印位置</label>
                                <select id="watermark_pos" v-model="systemSettings.watermark_pos" class="input-modern" :class="{ 'cursor-not-allowed opacity-60': hasPublicImageDomain }" :disabled="hasPublicImageDomain" @change="handleSelectChange('watermark_pos', systemSettings.watermark_pos)">
                                    <option value="" disabled>请选择图片水印位置</option>
                                    <option value="top-left">左上角</option>
                                    <option value="top-right">右上角</option>
                                    <option value="bottom-left">左下角</option>
                                    <option value="bottom-right">右下角</option>
                                    <option value="center">居中</option>
                                </select>
                                <div class="field-hint">系统默认右下角</div>
                            </div>

                            <!-- ========== 安全与登录 (表单部分) ========== -->
                            <div v-show="activeSettingsTab === 'security'" class="setting-group">
                                <label class="field-label" for="referer_white_list">Referer来源白名单</label>
                                <textarea id="referer_white_list" v-model="systemSettings.referer_white_list" type="password" class="input-modern min-h-[112px] leading-6" :class="{ 'cursor-not-allowed opacity-60': hasPublicImageDomain }" :disabled="hasPublicImageDomain" placeholder="Referer来源白名单，多个以英文逗号分隔" @blur="handleFieldBlur('referer_white_list', systemSettings.referer_white_list)" rows="4"></textarea>
                                <div class="field-hint">1. 仅需填写域名（支持主域名），多个以英文逗号分隔；<br>2. 无需填写协议，无需填写端口；<br>3. 如果开启了来源白名单，那么仅能从这些来源访问图片资源（直接打开不受限制）</div>
                                <div v-if="hasPublicImageDomain" class="field-hint text-amber-600 dark:text-amber-300">已配置图片直链域名，直接访问不会经过系统代理，来源白名单不会生效。</div>
                            </div>

                            <!-- ========== API ========== -->
                            <div v-show="activeSettingsTab === 'api'" class="setting-group">
                                <label class="field-label" for="api_token">API Token</label>
                                <div class="flex flex-col gap-2 sm:relative sm:block sm:w-full">
                                    <input id="api_token" v-model="systemSettings.api_token" type="text" class="input-modern sm:pr-20" :placeholder="systemSettings.api_token_configured ? '已配置，留空表示不修改' : '未配置，请输入 API Token'" @blur="handleFieldBlur('api_token', systemSettings.api_token)" />
                                    <button type="button" class="inline-flex h-10 items-center justify-center rounded-xl bg-slate-900 px-3.5 text-sm font-medium text-white transition hover:bg-slate-700 sm:absolute sm:right-1 sm:top-1 sm:h-[calc(100%-8px)] dark:bg-white dark:text-slate-900 dark:hover:bg-slate-200" @click="generateApiToken">生成</button>
                                </div>
                                <div class="field-hint">1. 用于调用 API 接口，在请求头 Authorization 字段中添加 oneimg_token={API Token}；<br>2. 仅在首次设置时显示，刷新后将再不显示，请注意保存；<br>3. {{ systemSettings.api_token_configured ? '当前已配置' : '当前未配置' }}</div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- 开关与特定面板 (右侧/左侧开关) -->
                <div class="order-2 md:order-1 w-full p-0 mx-auto" :class="{ 'xl:col-span-2': activeSettingsTab === 'seo' }">
                    
                    <!-- SEO设置独立面板 -->
                    <div v-show="activeSettingsTab === 'seo'" class="section-card mb-3.5 space-y-4 p-3.5 sm:p-4 md:space-y-5 md:p-6">
                        <h2 class="panel-title mb-4 flex items-center text-lg font-semibold sm:text-xl md:mb-5">
                            <span class="panel-icon mr-2 text-2xl"><i class="ri-seo-line"></i></span>SEO 设置
                        </h2>
                        <div class="setting-group"><label class="field-label" for="seo_title">网站标题</label><input id="seo_title" v-model="systemSettings.seo_title" type="text" class="input-modern" placeholder="请输入网站标题" @blur="handleFieldBlur('seo_title', systemSettings.seo_title)" /></div>
                        <div class="setting-group"><label class="field-label" for="seo_description">网站描述</label><textarea id="seo_description" v-model="systemSettings.seo_description" type="text" class="input-modern min-h-[96px]" rows="3" placeholder="请输入网站描述" @blur="handleFieldBlur('seo_description', systemSettings.seo_description)"></textarea></div>
                        <div class="setting-group"><label class="field-label" for="seo_keywords">网站关键词</label><textarea id="seo_keywords" v-model="systemSettings.seo_keywords" type="text" class="input-modern min-h-[96px]" rows="3" placeholder="请输入网站关键词" @blur="handleFieldBlur('seo_keywords', systemSettings.seo_keywords)"></textarea></div>
                        <div class="setting-group"><label class="field-label" for="seo_icp">网站备案号</label><input id="seo_icp" v-model="systemSettings.seo_icp" type="text" class="input-modern" placeholder="请输入网站备案号" @blur="handleFieldBlur('seo_icp', systemSettings.seo_icp)" /><div class="field-hint">输入网站备案号会在页面底部显示备案信息</div></div>
                        <div class="setting-group"><label class="field-label" for="public_security">网站公安备案号</label><input id="public_security" v-model="systemSettings.public_security" type="text" class="input-modern" placeholder="请输入网站公安备案号" @blur="handleFieldBlur('public_security', systemSettings.public_security)" /><div class="field-hint">输入网站公安备案号会在页面底部显示公安备案信息</div></div>
                        <div class="setting-group"><label class="field-label" for="seo_icon">网站小图标</label><input id="seo_icon" v-model="systemSettings.seo_icon" type="text" class="input-modern" placeholder="请输入网站小图标" @blur="handleFieldBlur('seo_icon', systemSettings.seo_icon)" /><div class="field-hint">输入网站小图标URL会替换默认的小图标</div></div>
                    </div>

                    <!-- 安全与登录面板 (OIDC/CAS) -->
                    <div v-show="activeSettingsTab === 'security'" class="section-card mb-3.5 space-y-4 p-3.5 sm:p-4 md:space-y-5 md:p-6">
                        <h2 class="panel-title mb-4 flex items-center text-lg font-semibold sm:text-xl md:mb-5">
                            <span class="panel-icon mr-2 text-2xl"><i class="ri-shield-user-line"></i></span>登录与单点登录
                        </h2>
                        <!-- OIDC 卡片 -->
                        <div class="rounded-[18px] border border-slate-200/80 bg-slate-50 p-3.5 dark:border-white/10 dark:bg-slate-950 sm:p-4">
                            <div class="mb-4 flex flex-col items-start gap-3 border-b border-slate-200/80 pb-4 dark:border-white/10 sm:flex-row sm:items-center sm:justify-between">
                                <div><p class="text-sm font-semibold text-slate-900 dark:text-white">OIDC 登录</p><p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">通过支持 OpenID Connect 的身份提供方登录。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end sm:self-center"><input type="checkbox" v-model="systemSettings.oidc_enable" class="sr-only peer" @change="handleSwitchChange('oidc_enable', systemSettings.oidc_enable)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
                            </div>
                            <div class="space-y-4">
                                <div class="setting-group"><label class="field-label" for="oidc_issuer">Issuer URL</label><input id="oidc_issuer" v-model="systemSettings.oidc_issuer" type="url" class="input-modern" placeholder="https://id.example.com" @blur="handleFieldBlur('oidc_issuer', systemSettings.oidc_issuer)" /><div class="field-hint">OIDC 发行方地址，系统将通过该地址发现授权端点。</div></div>
                                <div class="setting-group"><label class="field-label" for="oidc_client_id">Client ID</label><input id="oidc_client_id" v-model="systemSettings.oidc_client_id" type="text" class="input-modern" placeholder="请输入 OIDC Client ID" @blur="handleFieldBlur('oidc_client_id', systemSettings.oidc_client_id)" /></div>
                                <div class="setting-group"><label class="field-label" for="oidc_client_secret">Client Secret</label><input id="oidc_client_secret" v-model="systemSettings.oidc_client_secret" type="password" class="input-modern" :placeholder="systemSettings.oidc_client_secret_configured ? '已配置，留空表示不修改' : '未配置，请输入 Client Secret'" autocomplete="new-password" @blur="handleFieldBlur('oidc_client_secret', systemSettings.oidc_client_secret)" /><div class="field-hint">{{ systemSettings.oidc_client_secret_configured ? '已配置，留空表示不修改' : '启用 OIDC 登录前必须配置' }}</div></div>
                                <div class="setting-row"><div><p class="setting-row-title">首次登录自动创建用户</p><p class="setting-row-hint">关闭后，尚未绑定本地账户的 OIDC 用户无法登录。</p></div><label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.oidc_auto_provision" class="sr-only peer" @change="handleSwitchChange('oidc_auto_provision', systemSettings.oidc_auto_provision)"><div class="switch-track"></div><div class="switch-thumb"></div></label></div>
                                <div class="setting-group"><label class="field-label" for="oidc_super_admin_username">映射超级管理员用户名</label><input id="oidc_super_admin_username" v-model="systemSettings.oidc_super_admin_username" type="text" maxlength="255" class="input-modern" placeholder="留空表示不映射" @blur="handleFieldBlur('oidc_super_admin_username', systemSettings.oidc_super_admin_username)" /><div class="field-hint">OIDC 校验成功后，最终用户名与此值完全一致时登录本地超级管理员账户（区分大小写）。</div></div>
                                <div class="setting-group"><label class="field-label" for="oidc_redirect_url">回调 URL</label><input id="oidc_redirect_url" v-model="systemSettings.oidc_redirect_url" type="url" class="input-modern" placeholder="https://img.example.com/api/auth/oidc/callback" @blur="handleFieldBlur('oidc_redirect_url', systemSettings.oidc_redirect_url)" /><div class="field-hint">需与 OIDC 身份提供方中登记的回调地址完全一致。</div><div class="field-hint rounded-xl bg-slate-100 px-3 py-2 dark:bg-white/5"><span class="font-medium text-slate-600 dark:text-slate-300">当前有效回调地址：</span><code class="break-all">{{ systemSettings.oidc_redirect_url_effective || '尚未生成' }}</code></div></div>
                                <div class="grid gap-4 lg:grid-cols-2">
                                    <div class="setting-group"><label class="field-label" for="oidc_scopes">Scopes</label><input id="oidc_scopes" v-model="systemSettings.oidc_scopes" type="text" class="input-modern" placeholder="openid profile email" @blur="handleFieldBlur('oidc_scopes', systemSettings.oidc_scopes)" /></div>
                                    <div class="setting-group"><label class="field-label" for="oidc_username_claim">用户名 Claim</label><input id="oidc_username_claim" v-model="systemSettings.oidc_username_claim" type="text" class="input-modern" placeholder="preferred_username" @blur="handleFieldBlur('oidc_username_claim', systemSettings.oidc_username_claim)" /></div>
                                </div>
                                <div class="setting-group"><label class="field-label" for="oidc_display_name">登录按钮名称</label><input id="oidc_display_name" v-model="systemSettings.oidc_display_name" type="text" class="input-modern" placeholder="OIDC 登录" @blur="handleFieldBlur('oidc_display_name', systemSettings.oidc_display_name)" /></div>
                            </div>
                        </div>
                        <!-- CAS 卡片 -->
                        <div class="rounded-[18px] border border-slate-200/80 bg-slate-50 p-3.5 dark:border-white/10 dark:bg-slate-950 sm:p-4">
                            <div class="mb-4 flex flex-col items-start gap-3 border-b border-slate-200/80 pb-4 dark:border-white/10 sm:flex-row sm:items-center sm:justify-between">
                                <div><p class="text-sm font-semibold text-slate-900 dark:text-white">CAS 登录</p><p class="mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400">CAS 3.0 协议，固定使用 <code>/p3/serviceValidate</code> 校验 XML 响应。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end sm:self-center"><input type="checkbox" v-model="systemSettings.cas_enable" class="sr-only peer" @change="handleSwitchChange('cas_enable', systemSettings.cas_enable)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
                            </div>
                            <div class="space-y-4">
                                <div class="setting-group"><label class="field-label" for="cas_server_url">CAS Server URL</label><input id="cas_server_url" v-model="systemSettings.cas_server_url" type="url" class="input-modern" placeholder="https://cas.example.com/cas" @blur="handleFieldBlur('cas_server_url', systemSettings.cas_server_url)" /><div class="field-hint">填写 CAS 服务根地址，无需附加 <code>/login</code> 或 <code>/p3/serviceValidate</code>。</div></div>
                                <div class="setting-row"><div><p class="setting-row-title">首次登录自动创建用户</p><p class="setting-row-hint">关闭后，尚未绑定本地账户的 CAS 用户无法登录。</p></div><label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.cas_auto_provision" class="sr-only peer" @change="handleSwitchChange('cas_auto_provision', systemSettings.cas_auto_provision)"><div class="switch-track"></div><div class="switch-thumb"></div></label></div>
                                <div class="setting-group"><label class="field-label" for="cas_super_admin_username">映射超级管理员用户名</label><input id="cas_super_admin_username" v-model="systemSettings.cas_super_admin_username" type="text" maxlength="255" class="input-modern" placeholder="留空表示不映射" @blur="handleFieldBlur('cas_super_admin_username', systemSettings.cas_super_admin_username)" /><div class="field-hint">CAS3 XML 的 &lt;cas:user&gt; 与此值完全一致时登录本地超级管理员账户（区分大小写）。</div></div>
                                <div class="setting-group"><label class="field-label" for="cas_service_url">Service URL</label><input id="cas_service_url" v-model="systemSettings.cas_service_url" type="url" class="input-modern" placeholder="https://img.example.com/api/auth/cas/callback" @blur="handleFieldBlur('cas_service_url', systemSettings.cas_service_url)" /><div class="field-hint">需在 CAS 服务端允许列表中登记该地址。</div><div class="field-hint rounded-xl bg-slate-100 px-3 py-2 dark:bg-white/5"><span class="font-medium text-slate-600 dark:text-slate-300">当前有效 Service 地址：</span><code class="break-all">{{ systemSettings.cas_service_url_effective || '尚未生成' }}</code></div></div>
                                <div class="setting-group"><label class="field-label" for="cas_display_name">登录按钮名称</label><input id="cas_display_name" v-model="systemSettings.cas_display_name" type="text" class="input-modern" placeholder="CAS 登录" @blur="handleFieldBlur('cas_display_name', systemSettings.cas_display_name)" /></div>
                            </div>
                        </div>
                    </div>

                    <!-- 通用开关面板 -->
                    <div v-show="activeSettingsTab !== 'seo'" class="section-card p-3.5 sm:p-4 md:p-6">
                        <h2 class="panel-title mb-4 flex items-center text-lg font-semibold sm:text-xl md:mb-5">
                            <span class="panel-icon mr-2 text-2xl"><i class="ri-settings-2-line"></i></span>
                            {{ activeSettingsTabLabel }}开关
                        </h2>
                        
                        <div class="account-form space-y-4 md:space-y-5">
                            <!-- 上传与存储开关 -->
                            <div v-show="activeSettingsTab === 'storage'" class="setting-row">
                                <div><p class="setting-row-title">多存储同步</p><p class="setting-row-hint">开启后文件先保存到本机，再由后台同步到用户配置的多个存储源；关闭时保持原有单存储上传方式。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.multi_storage_sync" class="sr-only peer" @change="handleSwitchChange('multi_storage_sync', systemSettings.multi_storage_sync)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
                            </div>

                            <div v-show="activeSettingsTab === 'storage'" class="setting-row">
                                <div><p class="setting-row-title">加密存储</p><p class="setting-row-hint">开启后，新上传的原图和缩略图会以 AES-256-GCM 密文保存到本地及所有远端存储，访问时由程序统一解密后返回明文图片。历史文件保持原格式；请勿更换 CONFIG_SECRET。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.encrypted_storage" class="sr-only peer" @change="handleSwitchChange('encrypted_storage', systemSettings.encrypted_storage)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
                            </div>

                            <!-- 图片处理开关 -->
                            <div v-show="activeSettingsTab === 'image'" class="setting-row">
                                <div><p class="setting-row-title">压缩图片</p><p class="setting-row-hint">开启后，上传的图片将自动进行无损或轻度有损压缩。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.compress_image" class="sr-only peer" @change="handleSwitchChange('compress_image', systemSettings.compress_image)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
                            </div>

                            <!-- 安全与登录开关 -->
                            <div v-show="activeSettingsTab === 'security'" class="setting-row">
                                <div><p class="setting-row-title">PoW 验证</p><p class="setting-row-hint">开启后，登录时需要完成工作量证明，有效防止暴力破解。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.pow_verify" class="sr-only peer" @change="handleSwitchChange('pow_verify', systemSettings.pow_verify)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
                            </div>

                            <div v-show="activeSettingsTab === 'security'" class="setting-row">
                                <div><p class="setting-row-title">允许游客访问</p><p class="setting-row-hint">关闭后，未登录的游客无法查看图床上的任何图片。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.tourist" class="sr-only peer" @change="handleSwitchChange('tourist', systemSettings.tourist)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
                            </div>

                            <div v-show="activeSettingsTab === 'security'" class="setting-row">
                                <div><p class="setting-row-title">开放注册</p><p class="setting-row-hint">关闭后，将停止新用户自行注册。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.start_register" class="sr-only peer" @change="handleSwitchChange('start_register', systemSettings.start_register)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
                            </div>

                            <div v-show="activeSettingsTab === 'security'" class="setting-row">
                                <div><p class="setting-row-title">启用防盗链</p><p class="setting-row-hint">开启后，仅允许白名单内的域名引用图片资源。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.referer_white_enable" class="sr-only peer" @change="handleSwitchChange('referer_white_enable', systemSettings.referer_white_enable)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
                            </div>

                            <!-- 通知开关 -->
                            <div v-show="activeSettingsTab === 'notifications'" class="setting-row">
                                <div><p class="setting-row-title">启用 TG 通知</p><p class="setting-row-hint">开启后，上传图片等操作会通过 Telegram Bot 发送通知。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.tg_notice" class="sr-only peer" @change="handleSwitchChange('tg_notice', systemSettings.tg_notice)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
                            </div>

                            <!-- API开关 -->
                            <div v-show="activeSettingsTab === 'api'" class="setting-row">
                                <div><p class="setting-row-title">启用 API</p><p class="setting-row-hint">开启后，允许通过 API Token 调用上传等接口。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.start_api" class="sr-only peer" @change="handleSwitchChange('start_api', systemSettings.start_api)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
                            </div>
                            <div v-show="activeSettingsTab === 'storage'" class="setting-row">
                                <div>
                                    <p class="setting-row-title">保存源文件名</p>
                                    <p class="setting-row-hint">启用保存原图功能时将不自动重命名，”上传文件名称”设置也将失效。</p>
                                </div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.save_original_name"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('save_original_name', systemSettings.save_original_name)"
                                    >
                                    <div class="switch-track"></div>
                                    <div class="switch-thumb"></div>
                                </label>
                            </div>
                            <div v-show="activeSettingsTab === 'image'" class="setting-row">
                                <div>
                                    <p class="setting-row-title">保存 WEBP 格式</p>
                                </div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.save_webp"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('save_webp', systemSettings.save_webp)"
                                    >
                                    <div class="switch-track"></div>
                                    <div class="switch-thumb"></div>
                                </label>
                            </div>
                            <div v-show="activeSettingsTab === 'image'" class="setting-row">
                                <div>
                                    <p class="setting-row-title">生成缩略图</p>
                                    <p class="setting-row-hint">生成缩略图，可提升后台预览速度，上传速度稍慢。</p>
                                </div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center">
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.thumbnail"
                                        class="sr-only peer"
                                        @change="handleSwitchChange('thumbnail', systemSettings.thumbnail)"
                                    >
                                    <div class="switch-track"></div>
                                    <div class="switch-thumb"></div>
                                </label>
                            </div>
                            <div v-show="activeSettingsTab === 'image'" class="setting-row">
                                <div>
                                    <p class="setting-row-title">开启图片水印</p>
                                    <p class="setting-row-hint">
                                        {{ hasPublicImageDomain ? '已配置图片直链域名，图片水印不会生效。' : '新上传的图片自动添加水印，历史图片不会补加。' }}
                                    </p>
                                </div>
                                <label
                                    class="relative inline-flex items-center self-end md:self-center"
                                    :class="hasPublicImageDomain ? 'cursor-not-allowed opacity-60' : 'cursor-pointer'"
                                >
                                    <input 
                                        type="checkbox" 
                                        v-model="systemSettings.watermark_enable"
                                        class="sr-only peer"
                                        :disabled="hasPublicImageDomain"
                                        @change="handleSwitchChange('watermark_enable', systemSettings.watermark_enable)"
                                    >
                                    <div class="switch-track"></div>
                                    <div class="switch-thumb"></div>
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
import { ref, computed, onMounted } from 'vue'

const allSettingsTabs = [
    { key: 'storage', label: '上传与存储', icon: 'ri-upload-cloud-2-line', perm: 'setting:upload' },
    { key: 'image', label: '图片处理', icon: 'ri-image-line', perm: 'setting:image' },
    { key: 'security', label: '安全与登录', icon: 'ri-shield-keyhole-line', perm: 'setting:security' },
    { key: 'notifications', label: '通知', icon: 'ri-notification-3-line', perm: 'setting:notification' },
    { key: 'api', label: 'API', icon: 'ri-code-s-slash-line', perm: 'setting:api' },
    { key: 'seo', label: '站点SEO', icon: 'ri-seo-line', perm: 'setting:seo' },
]
const initialSettings = ref({})
const activeSettingsTab = ref('storage')
const systemSettings = ref({})
const presetBuckets = ref([])
const mySettingPerms = ref([])

const settingsTabs = computed(() => {
    if (mySettingPerms.value.length === 0) return []
    if (mySettingPerms.value.length === allSettingsTabs.length) return allSettingsTabs
    return allSettingsTabs.filter(tab => mySettingPerms.value.includes(tab.perm))
})

const activeSettingsTabLabel = computed(() => {
    return settingsTabs.value.find(t => t.key === activeSettingsTab.value)?.label || ''
})

const publicDomainStorageTypes = ['s3', 'r2']

const currentDefaultBucket = computed(() => {
    return presetBuckets.value.find(bucket => String(bucket.id) === String(systemSettings.default_storage))
})

const supportsPublicImageDomain = computed(() => {
    return publicDomainStorageTypes.includes(currentDefaultBucket.value?.type)
})

const hasPublicImageDomain = computed(() => !!systemSettings.value.public_image_domain)
const publicImageDomainUnavailable = computed(() => {
    return !supportsPublicImageDomain.value
})
const publicImageDomainInputDisabled = computed(() => {
    return (systemSettings.encrypted_storage || publicImageDomainUnavailable.value) && !hasPublicImageDomain.value
})
const publicImageDomainHint = computed(() => {
    if (systemSettings.encrypted_storage) {
        return '加密存储已开启，图片必须通过程序解密后访问，不能使用存储服务直链域名。'
    }
    if (!supportsPublicImageDomain.value) {
        return '当前默认存储不支持图片直链域名，仅 S3/R2 存储可用。'
    }
    if (hasPublicImageDomain.value) {
        return '启用后图片链接将直接使用该域名，图片水印文本、来源白名单等依赖系统代理的功能不会生效。'
    }
    return '填写 S3/R2 绑定的直链域名后，返回给用户的图片链接会直接使用该域名。'
})

const fetchSystemSettings = async () => {
    try {
        const response = await fetch('/api/settings/get', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({}) 
        })
        const res = await response.json()
        
        if (response.ok && res.code === 200) {
            mySettingPerms.value = res.setting_permissions || []
            systemSettings.value = res.data || {}
            initialSettings.value = JSON.parse(JSON.stringify(res.data || {}))
            if (settingsTabs.value.length > 0 && !settingsTabs.value.find(t => t.key === activeSettingsTab.value)) {
                activeSettingsTab.value = settingsTabs.value[0].key
            }
        } else {
            console.error('获取设置失败:', res.message)
            Message.error(res.message || '获取设置失败')
        }
    } catch (err) {
        console.error('请求错误:', err)
        Message.error('请求失败')
    }
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
const handleSelectChange = (key, value) => handleFieldBlur(key, value)
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
    const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789'
    let result = ''
    for (let i = 0; i < 32; i++) {
        result += chars.charAt(Math.floor(Math.random() * chars.length))
    }
    systemSettings.value.api_token = result
    handleFieldBlur('api_token', result)
}

// 6. 初始化
onMounted(() => {
    fetchSystemSettings()
    fetch('/api/buckets/list').then(res => res.json()).then(res => { if(res.code === 200) presetBuckets.value = res.data })
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
