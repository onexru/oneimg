<template>
<!-- 安全页签的开关已迁到 SettingsSecurityPanel（放在「登录与单点登录」上方），
     这个面板在 security 页签下没有任何内容，直接隐藏以免留下空壳。
     system 页签只有版本信息，同理隐藏。 -->
<div v-show="activeSettingsTab !== 'seo' && activeSettingsTab !== 'security' && activeSettingsTab !== 'system'" class="surface p-3.5 sm:p-4 md:p-6">
                        <h2 class="panel-title mb-4 md:mb-5">
                            <span class="panel-icon"><i class="ri-settings-2-line"></i></span>
                            {{ activeSettingsTabLabel }}开关
                        </h2>

                        <div class="space-y-4 md:space-y-5">
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
                                    <p class="setting-row-hint">开启后图片沿用源文件名，不再追加随机后缀，“上传文件名称”设置失效；同一目录下重名会自动编号（photo、photo-2、photo-3）。</p>
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
