<template>
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
</template>
<script setup>
import { inject } from 'vue';
import { settingsContext } from '@/utils/settingsAccess.js';
const { systemSettings, activeSettingsTab, activeSettingsTabLabel, presetBuckets, supportsPublicImageDomain, hasPublicImageDomain, publicImageDomainInputDisabled, publicImageDomainHint, handleFieldBlur, handleSelectChange, handleSwitchChange, generatedApiToken, tokenGenerating, generateApiToken, generateRandomGraph } = inject(settingsContext);
</script>
