<template>
<div v-show="activeSettingsTab === 'seo'" class="surface space-y-4 p-3.5 sm:p-4 md:space-y-5 md:p-6">                        <h2 class="panel-title">
                            <span class="panel-icon"><i class="ri-seo-line"></i></span>SEO 设置
                        </h2>
                        <!-- 单列铺满时输入框会拉到 1100px 宽，一行只放一个短标题太浪费；
                             标题/备案号这类短字段并成两列，描述和关键词是长文本仍占整行。 -->
                        <div class="grid gap-4 md:grid-cols-2 md:gap-5">
                            <div class="setting-group"><label class="field-label" for="seo_title">网站标题</label><input id="seo_title" v-model="systemSettings.seo_title" type="text" class="input-modern" placeholder="请输入网站标题" @blur="handleFieldBlur('seo_title', systemSettings.seo_title)" /></div>
                            <div class="setting-group"><label class="field-label" for="seo_icon">网站小图标</label><input id="seo_icon" v-model="systemSettings.seo_icon" type="text" class="input-modern" placeholder="请输入网站小图标 URL" @blur="handleFieldBlur('seo_icon', systemSettings.seo_icon)" /><div class="field-hint">输入小图标 URL 会替换默认图标</div></div>
                        </div>
                        <div class="setting-group"><label class="field-label" for="seo_description">网站描述</label><textarea id="seo_description" v-model="systemSettings.seo_description" type="text" class="input-modern min-h-[96px]" rows="3" placeholder="请输入网站描述" @blur="handleFieldBlur('seo_description', systemSettings.seo_description)"></textarea></div>
                        <div class="setting-group"><label class="field-label" for="seo_keywords">网站关键词</label><textarea id="seo_keywords" v-model="systemSettings.seo_keywords" type="text" class="input-modern min-h-[96px]" rows="3" placeholder="请输入网站关键词" @blur="handleFieldBlur('seo_keywords', systemSettings.seo_keywords)"></textarea></div>
                        <div class="grid gap-4 md:grid-cols-2 md:gap-5">
                            <div class="setting-group"><label class="field-label" for="seo_icp">网站备案号</label><input id="seo_icp" v-model="systemSettings.seo_icp" type="text" class="input-modern" placeholder="请输入网站备案号" @blur="handleFieldBlur('seo_icp', systemSettings.seo_icp)" /><div class="field-hint">填写后会在页面底部显示备案信息</div></div>
                            <div class="setting-group"><label class="field-label" for="public_security">网站公安备案号</label><input id="public_security" v-model="systemSettings.public_security" type="text" class="input-modern" placeholder="请输入网站公安备案号" @blur="handleFieldBlur('public_security', systemSettings.public_security)" /><div class="field-hint">填写后会在页面底部显示公安备案信息</div></div>
                        </div>
                    </div>
</template>
<script setup>
import { inject } from 'vue';
import { settingsContext } from '@/utils/settingsAccess.js';
const { systemSettings, activeSettingsTab, activeSettingsTabLabel, presetBuckets, supportsPublicImageDomain, hasPublicImageDomain, publicImageDomainInputDisabled, publicImageDomainHint, handleFieldBlur, handleSelectChange, handleSwitchChange, generatedApiToken, tokenGenerating, generateApiToken, generateRandomGraph } = inject(settingsContext);
</script>
