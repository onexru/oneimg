<template>
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
                                <template v-for="field in [{ key: 'max_upload_files', label: '每批最多上传文件数' }, { key: 'tag_max_length', label: '标签最大字符数' }]" :key="field.key">
  <div v-if="systemSettings[field.key] !== undefined" class="setting-group">
    <label class="field-label" :for="field.key">{{ field.label }}</label>
    <input :id="field.key" v-model.number="systemSettings[field.key]" type="number" min="1" max="50" step="1" class="input-modern" @blur="handleFieldBlur(field.key, systemSettings[field.key])" />
  </div>
</template>
<label class="field-label" for="allowed_types">允许上传的图片类型</label>
                                <input id="allowed_types" v-model="systemSettings.allowed_types" type="text" class="input-modern" placeholder="允许上传的图片类型（SVG 不允许上传）" @blur="handleFieldBlur('allowed_types', systemSettings.allowed_types)" />
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
                                <div class="field-hint">填写允许的来源域名，多个以英文逗号分隔。防盗链仅限制浏览器来源，不是私有图片授权；需要私有访问请使用鉴权保护。</div>
                                <label v-if="systemSettings.referer_white_enable" class="mt-3 flex items-center gap-2 text-sm">
                                    <input type="checkbox" v-model="systemSettings.referer_allow_empty" :disabled="hasPublicImageDomain" @change="handleSwitchChange('referer_allow_empty', systemSettings.referer_allow_empty)">
                                    允许无 Referer 直接访问
                                </label>
                                <p v-if="systemSettings.referer_white_enable" class="field-hint">默认允许直接打开公开链接。关闭后，无来源的请求将被拒绝；Referer 可伪造，不能替代私有访问授权。</p>
                                <div v-if="hasPublicImageDomain" class="field-hint text-amber-600 dark:text-amber-300">已配置图片直链域名，直接访问不会经过系统代理，来源白名单不会生效。</div>
                            </div>

                            <!-- ========== API ========== -->
                            <div v-show="activeSettingsTab === 'api'" class="setting-group">
                                <div v-if="systemSettings.random_image_limit !== undefined" class="setting-group">
  <label class="field-label" for="random_image_limit">随机图最大返回数量</label>
  <input id="random_image_limit" v-model.number="systemSettings.random_image_limit" type="number" min="1" max="100" step="1" class="input-modern" @blur="handleFieldBlur('random_image_limit', systemSettings.random_image_limit)" />
</div>
<div v-if="Array.isArray(systemSettings.api_credentials)" class="mb-4 space-y-2" aria-label="API 凭据使用信息">
  <p v-if="!systemSettings.api_credentials.length" class="field-hint">暂无有效 API 凭据。</p>
  <div v-for="(credential,index) in systemSettings.api_credentials" :key="index" class="rounded-xl border border-slate-200 p-3 text-sm dark:border-white/10">
    <p class="break-words">权限范围：{{ credential.scopes }}</p>
    <p>到期时间：{{ new Date(credential.expires_at).toLocaleString() }}</p>
    <p>最近使用：{{ credential.last_used_at ? new Date(credential.last_used_at).toLocaleString() : '尚未使用' }}</p>
  </div>
</div>
<label class="field-label" for="api_token">API Token</label>
                                <div class="flex flex-col gap-2 sm:relative sm:block sm:w-full">
                                    <input id="api_token" :value="generatedApiToken" readonly :type="generatedApiToken ? 'text' : 'password'" class="input-modern sm:pr-24" :placeholder="systemSettings.api_token_configured ? '已配置（旧凭据不可读取）' : '尚未配置'" autocomplete="off" />
                                    <button type="button" class="inline-flex h-10 items-center justify-center rounded-xl bg-slate-900 px-3.5 text-sm font-medium text-white transition hover:bg-slate-700 sm:absolute sm:right-1 sm:top-1 sm:h-[calc(100%-8px)] dark:bg-white dark:text-slate-900 dark:hover:bg-slate-200" :disabled="tokenGenerating" @click="generateApiToken">{{ tokenGenerating ? '生成中…' : '重新生成' }}</button>
                                </div>
                                <div class="field-hint">凭据仅在生成后显示一次，离开本页后无法读取。请立即保存；重新生成会撤销此前凭据。请求头使用 Authorization: oneimg_token={API Token}。</div>
                            </div>

                            <div v-show="activeSettingsTab === 'api'" class="setting-group">
                                <label class="field-label" for="api_token">配置随机图</label>
                                <button type="button" class="h-10 w-full rounded-xl bg-slate-900 px-3.5 text-sm font-medium text-white transition hover:bg-slate-700 dark:bg-white dark:text-slate-900 dark:hover:bg-slate-200" @click="generateRandomGraph">配置随机图</button>
                                <div class="field-hint">
                                    随机图API接口：<a class="text-blue-600 dark:text-blue-400" href="/api/images/random" target="_blank">/api/images/random</a><br>
                                    随机图参数：<br>
                                    1.tag text 标签分类<br>
                                    2.model json/image 返回数据类型（json、图片流）<br>
                                    3.limit int 返回数量（默认1,最大{{ systemSettings.random_image_limit || 20 }},仅在model为json时生效）<br>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
</template>
<script setup>
import { inject } from 'vue';
import { settingsContext } from '@/utils/settingsAccess.js';
const { systemSettings, activeSettingsTab, activeSettingsTabLabel, presetBuckets, supportsPublicImageDomain, hasPublicImageDomain, publicImageDomainInputDisabled, publicImageDomainHint, handleFieldBlur, handleSelectChange, handleSwitchChange, generatedApiToken, tokenGenerating, generateApiToken, generateRandomGraph } = inject(settingsContext);
</script>
