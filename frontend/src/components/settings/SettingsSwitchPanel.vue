<template>
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
                                <div><p class="setting-row-title">人机验证方式</p><p class="setting-row-hint">登录/注册时要求完成的人机验证。在线POW 使用配置的外部验证服务（可启用本地替代）；Turnstile 使用 Cloudflare；cap-pow 为本地自托管验证。</p></div>
                                <select id="verify_method" v-model="systemSettings.verify_method" class="input-modern w-auto self-end md:self-center" @change="handleSelectChange('verify_method', systemSettings.verify_method)">
                                    <option value="none">无验证</option>
                                    <option value="pow">在线POW</option>
                                    <option value="turnstile">Cloudflare Turnstile</option>
                                    <option value="cappow">cap-pow 本地</option>
                                </select>
                            </div>

                            <div v-if="systemSettings.pow_verify_url !== undefined" v-show="activeSettingsTab === 'security' && systemSettings.verify_method === 'pow'" class="setting-group">
                                <label class="field-label" for="pow_verify_url">在线 POW 服务端验证地址</label>
                                <input id="pow_verify_url" v-model="systemSettings.pow_verify_url" type="url" maxlength="2048" class="input-modern" placeholder="https://cha.eta.im/api/validate" @blur="handleFieldBlur('pow_verify_url', systemSettings.pow_verify_url)" />
                                <div class="field-hint">服务器向此地址提交 token 并读取 success；不会自动更改浏览器脚本或 challenge 地址。</div>
                            </div>
                            <div v-if="systemSettings.pow_script_url !== undefined" v-show="activeSettingsTab === 'security' && systemSettings.verify_method === 'pow'" class="setting-group">
                                <label class="field-label" for="pow_script_url">在线 POW 浏览器脚本地址</label>
                                <input id="pow_script_url" v-model="systemSettings.pow_script_url" type="url" maxlength="2048" class="input-modern" placeholder="https://cha.eta.im/static/js/pow.min.js" @blur="handleFieldBlur('pow_script_url', systemSettings.pow_script_url)" />
                                <div class="field-hint">加载可信提供方的 pow-widget 脚本；脚本可在本站执行，请勿填写不可信地址。更换脚本后请刷新登录/注册页面。</div>
                            </div>
                            <div v-if="systemSettings.pow_widget_url !== undefined" v-show="activeSettingsTab === 'security' && systemSettings.verify_method === 'pow'" class="setting-group">
                                <label class="field-label" for="pow_widget_url">在线 POW 浏览器 Challenge 地址</label>
                                <input id="pow_widget_url" v-model="systemSettings.pow_widget_url" type="url" maxlength="2048" class="input-modern" placeholder="https://cha.eta.im/" @blur="handleFieldBlur('pow_widget_url', systemSettings.pow_widget_url)" />
                                <div class="field-hint">作为 pow-widget 的 data-pow-api-endpoint；须与服务端验证地址属于同一套 token 协议，并允许本站跨域访问。三个地址均使用公共 HTTPS（443），不含凭据、查询参数或片段；留空沿用旧提供方默认值。</div>
                            </div>
                            <div v-if="systemSettings.pow_verify_timeout_seconds !== undefined" v-show="activeSettingsTab === 'security' && systemSettings.verify_method === 'pow'" class="setting-group">
                                <label class="field-label" for="pow_verify_timeout_seconds">在线 POW 验证超时（秒）</label>
                                <input id="pow_verify_timeout_seconds" v-model.number="systemSettings.pow_verify_timeout_seconds" type="number" min="1" max="15" step="1" class="input-modern" @blur="handleFieldBlur('pow_verify_timeout_seconds', systemSettings.pow_verify_timeout_seconds)" />
                            </div>
                            <div v-if="systemSettings.pow_local_fallback !== undefined" v-show="activeSettingsTab === 'security'" class="setting-row">
                                <div><p class="setting-row-title">使用本地 POW 替代在线验证</p><p class="setting-row-hint">明确切换到本地 cap-pow，避免依赖外部验证服务；在线服务出错不会跳过验证或自动切换。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.pow_local_fallback" class="sr-only peer" @change="handleSwitchChange('pow_local_fallback', systemSettings.pow_local_fallback)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
                            </div>
                            <!-- Turnstile 配置 -->
                            <div v-if="systemSettings.verify_method === 'turnstile'" v-show="activeSettingsTab === 'security'" class="setting-group">
                                <label class="field-label" for="cloudflare_api_token">Cloudflare API Token（Turnstile:Read）</label>
                                <input id="cloudflare_api_token" v-model="systemSettings.cloudflare_api_token" type="password" class="input-modern" :placeholder="systemSettings.cloudflare_api_token_configured ? '已配置，留空表示不修改' : '未配置，请输入 API Token'" autocomplete="new-password" @blur="handleFieldBlur('cloudflare_api_token', systemSettings.cloudflare_api_token)" />
                                <div class="field-hint">{{ systemSettings.cloudflare_api_token_configured ? '已配置，留空表示不修改' : '保存公钥时校验用。需在 Cloudflare 控制台创建，权限：Account → Turnstile → Read' }}</div>
                            </div>
                            <div v-if="systemSettings.verify_method === 'turnstile'" v-show="activeSettingsTab === 'security'" class="setting-group">
                                <label class="field-label" for="cloudflare_account_id">Cloudflare 账号 ID</label>
                                <input id="cloudflare_account_id" v-model="systemSettings.cloudflare_account_id" type="text" class="input-modern" placeholder="Cloudflare 仪表盘首页右下角" @blur="handleFieldBlur('cloudflare_account_id', systemSettings.cloudflare_account_id)" />
                                <div class="field-hint">Cloudflare 控制台首页右下角可查看账号 ID。</div>
                            </div>
                            <div v-if="systemSettings.verify_method === 'turnstile'" v-show="activeSettingsTab === 'security'" class="setting-group">
                                <label class="field-label" for="turnstile_site_key">Turnstile 站点公钥 (Site Key)</label>
                                <input id="turnstile_site_key" v-model="systemSettings.turnstile_site_key" type="text" class="input-modern" placeholder="0x..." @blur="handleFieldBlur('turnstile_site_key', systemSettings.turnstile_site_key)" />
                                <div class="field-hint">在 Cloudflare 控制台 → Turnstile 创建站点后获取。保存时通过 Cloudflare API 校验公钥是否真实存在。</div>
                            </div>
                            <div v-if="systemSettings.verify_method === 'turnstile'" v-show="activeSettingsTab === 'security'" class="setting-group">
                                <label class="field-label" for="turnstile_secret_key">Turnstile 密钥 (Secret Key)</label>
                                <input id="turnstile_secret_key" v-model="systemSettings.turnstile_secret_key" type="password" class="input-modern" :placeholder="systemSettings.turnstile_secret_key_configured ? '已配置，留空表示不修改' : '未配置，请输入 Secret Key'" autocomplete="new-password" @blur="handleFieldBlur('turnstile_secret_key', systemSettings.turnstile_secret_key)" />
                                <div class="field-hint">{{ systemSettings.turnstile_secret_key_configured ? '已配置，留空表示不修改' : '保存时通过 Cloudflare siteverify 校验密钥有效性' }}</div>
                            </div>

                            <!-- cap-pow 配置 -->
                            <div v-if="systemSettings.verify_method === 'cappow'" v-show="activeSettingsTab === 'security'" class="setting-group">
                                <label class="field-label" for="cappow_difficulty">cap-pow 难度</label>
                                <input id="cappow_difficulty" v-model="systemSettings.cappow_difficulty" type="number" min="1" max="8" class="input-modern" placeholder="4" @blur="handleFieldBlur('cappow_difficulty', systemSettings.cappow_difficulty)" />
                                <div class="field-hint">目标哈希前缀长度（1-8），数值越大越难。默认 4，通常 1-3 秒完成。</div>
                            </div>

                            <div v-show="activeSettingsTab === 'security'" class="setting-row">
                                <div><p class="setting-row-title">允许游客访问</p></div>
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
                            <div v-show="activeSettingsTab === 'api'" class="setting-row">
                                <div><p class="setting-row-title">启用 随机图</p><p class="setting-row-hint">开启后，默认返回系统内全部随机图，可在 “配置随机图” 中设置随机图范围。</p></div>
                                <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.random_graph" class="sr-only peer" @change="handleSwitchChange('random_graph', systemSettings.random_graph)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
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
</template>
<script setup>
import { inject } from 'vue';
import { settingsContext } from '@/utils/settingsAccess.js';
const { systemSettings, activeSettingsTab, activeSettingsTabLabel, presetBuckets, supportsPublicImageDomain, hasPublicImageDomain, publicImageDomainInputDisabled, publicImageDomainHint, handleFieldBlur, handleSelectChange, handleSwitchChange, generatedApiToken, tokenGenerating, generateApiToken, generateRandomGraph } = inject(settingsContext);
</script>
