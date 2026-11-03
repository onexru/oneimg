<template>
  <!-- 「安全与登录」页签顶部的开关组独立成面板：它原本混在通用开关面板里（那个面板要同时
       承载上传/图片/通知/API/SEO 五个页签的开关），导致安全选项被夹在中间。
       现在固定放在「登录与单点登录」上方，顺序符合「先定验证门槛，再配登录方式」。
       标题区别于下方 OIDC/CAS 面板，避免两个「安全与登录」并排。 -->
  <div v-show="activeSettingsTab === 'security'" class="surface space-y-4 p-3.5 sm:p-4 md:space-y-5 md:p-6">
    <h2 class="panel-title">
      <span class="panel-icon"><i class="ri-shield-keyhole-line"></i></span>
      人机验证与访问控制
    </h2>

    <!-- 人机验证方式：只有 none / turnstile / cappow 三种，在线 POW 已移除 -->
    <div class="setting-row">
      <div>
        <p class="setting-row-title">人机验证方式</p>
        <p class="setting-row-hint">登录/注册时要求完成的人机验证。Turnstile 使用 Cloudflare 托管验证；cap-pow 为本地自托管验证，不依赖外部服务。</p>
      </div>
      <select id="verify_method" v-model="systemSettings.verify_method" class="input-modern w-auto self-end md:self-center" @change="handleSelectChange('verify_method', systemSettings.verify_method)">
        <option value="none">无验证</option>
        <option value="turnstile">Cloudflare Turnstile</option>
        <option value="cappow">cap-pow 本地</option>
      </select>
    </div>

    <!-- 选中 Turnstile 但配置不全时，服务端会自动改用 cap-pow。
         这里必须明说，否则管理员以为自己开着 Turnstile，实际跑的是本地验证。 -->
    <div
      v-if="turnstileFallbackActive"
      role="status"
      class="flex items-start gap-2.5 rounded-[var(--radius-sm)] border border-amber-300/70 bg-amber-50 px-3.5 py-3 text-sm text-amber-800 dark:border-amber-400/25 dark:bg-amber-400/10 dark:text-amber-200"
    >
      <i class="ri-alert-line mt-0.5 shrink-0" aria-hidden="true"></i>
      <div class="min-w-0 space-y-1">
        <p class="font-medium">Turnstile 配置不完整，当前实际使用 cap-pow 本地验证</p>
        <p class="text-xs leading-5">缺少：{{ turnstileMissing.join('、') }}。补齐后保存即可自动切回 Turnstile，无需再改「人机验证方式」。</p>
      </div>
    </div>

    <!-- Turnstile 配置：四组字段成 2×2，密钥与公钥分开成对，
         避免四个长输入框各占一整行（1072px）导致纵向拉得很长。 -->
    <div v-if="systemSettings.verify_method === 'turnstile'" class="grid gap-4 md:grid-cols-2">
      <div class="setting-group">
        <label class="field-label" for="turnstile_site_key">Turnstile 站点公钥 (Site Key)</label>
        <input id="turnstile_site_key" v-model="systemSettings.turnstile_site_key" type="text" class="input-modern" placeholder="0x..." @blur="handleFieldBlur('turnstile_site_key', systemSettings.turnstile_site_key)" />
        <div class="field-hint">在 Cloudflare → Turnstile 创建站点后获取。保存时通过 API 校验公钥是否真实存在。</div>
      </div>
      <div class="setting-group">
        <label class="field-label" for="turnstile_secret_key">Turnstile 密钥 (Secret Key)</label>
        <input id="turnstile_secret_key" v-model="systemSettings.turnstile_secret_key" type="password" class="input-modern" :placeholder="systemSettings.turnstile_secret_key_configured ? '已配置，留空表示不修改' : '未配置，请输入 Secret Key'" autocomplete="new-password" @blur="handleFieldBlur('turnstile_secret_key', systemSettings.turnstile_secret_key)" />
        <div class="field-hint">{{ systemSettings.turnstile_secret_key_configured ? '已配置，留空表示不修改' : '保存时通过 Cloudflare siteverify 校验有效性' }}</div>
      </div>
      <div class="setting-group">
        <label class="field-label" for="cloudflare_api_token">Cloudflare API Token（Turnstile:Read）</label>
        <input id="cloudflare_api_token" v-model="systemSettings.cloudflare_api_token" type="password" class="input-modern" :placeholder="systemSettings.cloudflare_api_token_configured ? '已配置，留空表示不修改' : '未配置，请输入 API Token'" autocomplete="new-password" @blur="handleFieldBlur('cloudflare_api_token', systemSettings.cloudflare_api_token)" />
        <div class="field-hint">{{ systemSettings.cloudflare_api_token_configured ? '已配置，留空表示不修改' : '保存公钥时校验用，权限：Account → Turnstile → Read' }}</div>
      </div>
      <div class="setting-group">
        <label class="field-label" for="cloudflare_account_id">Cloudflare 账号 ID</label>
        <input id="cloudflare_account_id" v-model="systemSettings.cloudflare_account_id" type="text" class="input-modern" placeholder="Cloudflare 仪表盘首页右下角" @blur="handleFieldBlur('cloudflare_account_id', systemSettings.cloudflare_account_id)" />
        <div class="field-hint">Cloudflare 控制台首页右下角可查看。</div>
      </div>
    </div>

    <!-- cap-pow 配置 -->
    <div v-if="systemSettings.verify_method === 'cappow'" class="setting-group">
      <label class="field-label" for="cappow_difficulty">cap-pow 难度</label>
      <input id="cappow_difficulty" v-model="systemSettings.cappow_difficulty" type="number" min="1" max="8" class="input-modern" placeholder="4" @blur="handleFieldBlur('cappow_difficulty', systemSettings.cappow_difficulty)" />
      <div class="field-hint">目标哈希前缀长度（1-8），数值越大越难。默认 4，通常 1-3 秒完成。</div>
    </div>

    <!-- 三个开关并成两列：单列铺满时每个开关占满 1072px，右侧开关离文字 900px 远，
         视线要横跨整屏才能确认「这个开关属于哪一行」。两列后每格约 500px。 -->
    <div class="grid gap-3 md:grid-cols-2 md:gap-4">
      <div class="setting-row">
        <div><p class="setting-row-title">允许游客访问</p><p class="setting-row-hint">未登录也可上传与浏览。</p></div>
        <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.tourist" class="sr-only peer" @change="handleSwitchChange('tourist', systemSettings.tourist)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
      </div>

      <div class="setting-row">
        <div><p class="setting-row-title">开放注册</p><p class="setting-row-hint">关闭后停止新用户自行注册。</p></div>
        <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.start_register" class="sr-only peer" @change="handleSwitchChange('start_register', systemSettings.start_register)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
      </div>

      <div class="setting-row md:col-span-2">
        <div><p class="setting-row-title">启用防盗链</p><p class="setting-row-hint">开启后仅允许白名单内的域名引用图片资源。</p></div>
        <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.referer_white_enable" class="sr-only peer" @change="handleSwitchChange('referer_white_enable', systemSettings.referer_white_enable)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
      </div>
    </div>

    <!-- 白名单表单：归属「防盗链」，因此跟随开关出现在同一张卡片里，
         而不是留在另一栏的通用表单中（那样开关和它控制的输入框被拆成两处）。 -->
    <div v-if="systemSettings.referer_white_enable" class="setting-group rounded-[var(--radius-md)] border border-slate-200/80 bg-slate-50 px-3.5 py-3 dark:border-white/10 dark:bg-slate-950">
      <label class="field-label" for="referer_white_list">Referer 来源白名单</label>
      <textarea
        id="referer_white_list"
        v-model="systemSettings.referer_white_list"
        class="input-modern min-h-[96px] leading-6"
        :class="{ 'cursor-not-allowed opacity-60': hasPublicImageDomain }"
        :disabled="hasPublicImageDomain"
        placeholder="example.com, *.example.org"
        rows="3"
        @blur="handleFieldBlur('referer_white_list', systemSettings.referer_white_list)"
      ></textarea>
      <div class="field-hint">多个域名用英文逗号分隔。防盗链只限制浏览器来源，不是私有授权；需要私有访问请使用鉴权保护。</div>
      <label class="flex items-center gap-2 text-sm text-slate-700 dark:text-slate-300">
        <input type="checkbox" v-model="systemSettings.referer_allow_empty" :disabled="hasPublicImageDomain" @change="handleSwitchChange('referer_allow_empty', systemSettings.referer_allow_empty)">
        允许无 Referer 直接访问
      </label>
      <div class="field-hint">默认允许直接打开公开链接。关闭后无来源的请求会被拒绝；Referer 可伪造，不能替代私有访问授权。</div>
      <div v-if="hasPublicImageDomain" class="field-hint text-amber-700 dark:text-amber-300">已配置图片直链域名，直接访问不经过系统代理，来源白名单不会生效。</div>
    </div>
  </div>
</template>

<script setup>
import { computed, inject } from 'vue';
import { settingsContext } from '@/utils/settingsAccess.js';

const { systemSettings, activeSettingsTab, hasPublicImageDomain, handleFieldBlur, handleSelectChange, handleSwitchChange } = inject(settingsContext);

// 选了 Turnstile 但服务端实际按 cap-pow 跑 —— 说明 Turnstile 配置不完整
// （密钥缺失/公钥格式不对）或已被服务端判定密钥无效。
const turnstileFallbackActive = computed(() =>
  systemSettings.value.verify_method === 'turnstile'
  && systemSettings.value.verify_method_effective
  && systemSettings.value.verify_method_effective !== 'turnstile');

// 列出缺什么，避免管理员在四个字段里盲猜
const turnstileMissing = computed(() => {
  const s = systemSettings.value;
  const missing = [];
  if (!String(s.turnstile_site_key || '').trim()) missing.push('站点公钥');
  else if (!String(s.turnstile_site_key).trim().startsWith('0x')) missing.push('站点公钥格式（应以 0x 开头）');
  if (!s.turnstile_secret_key_configured) missing.push('密钥');
  if (!missing.length) missing.push('密钥有效性（Cloudflare 未能识别）');
  return missing;
});
</script>
