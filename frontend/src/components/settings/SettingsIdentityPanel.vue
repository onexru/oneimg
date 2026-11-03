<template>
  <!-- OIDC / CAS 两张卡：字段各自成 2 列网格。
       单列铺满时一个 URL 输入框会拉到 1072px，实际没有任何信息量；
       拆两列后每格约 500px，既够填长地址，也让整卡高度从 1651px 降到一半左右。 -->
  <div v-show="activeSettingsTab === 'security'" class="surface space-y-4 p-3.5 sm:p-4 md:space-y-5 md:p-6">
    <div>
      <h2 class="panel-title">
        <span class="panel-icon"><i class="ri-shield-user-line"></i></span>
        登录与单点登录
      </h2>
      <p class="mt-1.5 text-xs leading-5 text-slate-500 dark:text-slate-400">
        配置第三方身份提供方。两者可同时启用，登录页会并排显示对应按钮。
      </p>
    </div>

    <!-- ========== OIDC ========== -->
    <div class="auth-card">
      <div class="auth-card-head">
        <div class="min-w-0">
          <p class="auth-card-title">OIDC 登录</p>
          <p class="auth-card-desc">通过支持 OpenID Connect 的身份提供方登录。</p>
        </div>
        <label class="auth-switch">
          <input type="checkbox" v-model="systemSettings.oidc_enable" class="sr-only peer" @change="handleSwitchChange('oidc_enable', systemSettings.oidc_enable)">
          <div class="switch-track"></div>
          <div class="switch-thumb"></div>
        </label>
      </div>

      <div class="grid gap-4 md:grid-cols-2">
        <div class="setting-group">
          <label class="field-label" for="oidc_issuer">Issuer URL</label>
          <input id="oidc_issuer" v-model="systemSettings.oidc_issuer" type="url" class="input-modern" placeholder="https://id.example.com" @blur="handleFieldBlur('oidc_issuer', systemSettings.oidc_issuer)" />
          <div class="field-hint">发行方地址，系统将通过它发现授权端点。</div>
        </div>
        <div class="setting-group">
          <label class="field-label" for="oidc_client_id">Client ID</label>
          <input id="oidc_client_id" v-model="systemSettings.oidc_client_id" type="text" class="input-modern" placeholder="请输入 OIDC Client ID" @blur="handleFieldBlur('oidc_client_id', systemSettings.oidc_client_id)" />
        </div>
        <div class="setting-group md:col-span-2">
          <label class="field-label" for="oidc_client_secret">Client Secret</label>
          <input id="oidc_client_secret" v-model="systemSettings.oidc_client_secret" type="password" class="input-modern" :placeholder="systemSettings.oidc_client_secret_configured ? '已配置，留空表示不修改' : '未配置，请输入 Client Secret'" autocomplete="new-password" @blur="handleFieldBlur('oidc_client_secret', systemSettings.oidc_client_secret)" />
          <div class="field-hint">{{ systemSettings.oidc_client_secret_configured ? '已配置，留空表示不修改' : '启用 OIDC 登录前必须配置' }}</div>
        </div>
        <div class="setting-group">
          <label class="field-label" for="oidc_scopes">Scopes</label>
          <input id="oidc_scopes" v-model="systemSettings.oidc_scopes" type="text" class="input-modern" placeholder="openid profile email" @blur="handleFieldBlur('oidc_scopes', systemSettings.oidc_scopes)" />
        </div>
        <div class="setting-group">
          <label class="field-label" for="oidc_username_claim">用户名 Claim</label>
          <input id="oidc_username_claim" v-model="systemSettings.oidc_username_claim" type="text" class="input-modern" placeholder="preferred_username" @blur="handleFieldBlur('oidc_username_claim', systemSettings.oidc_username_claim)" />
        </div>
        <div class="setting-group">
          <label class="field-label" for="oidc_display_name">登录按钮名称</label>
          <input id="oidc_display_name" v-model="systemSettings.oidc_display_name" type="text" class="input-modern" placeholder="OIDC 登录" @blur="handleFieldBlur('oidc_display_name', systemSettings.oidc_display_name)" />
        </div>
        <div class="setting-group">
          <label class="field-label" for="oidc_super_admin_username">映射超级管理员用户名</label>
          <input id="oidc_super_admin_username" v-model="systemSettings.oidc_super_admin_username" type="text" maxlength="255" class="input-modern" placeholder="留空表示不映射" @blur="handleFieldBlur('oidc_super_admin_username', systemSettings.oidc_super_admin_username)" />
          <div class="field-hint">校验通过后用户名与此值完全一致时，登录本地超管（区分大小写）。</div>
        </div>
        <div class="setting-group md:col-span-2">
          <label class="field-label" for="oidc_redirect_url">回调 URL</label>
          <input id="oidc_redirect_url" v-model="systemSettings.oidc_redirect_url" type="url" class="input-modern" placeholder="https://img.example.com/api/auth/oidc/callback" @blur="handleFieldBlur('oidc_redirect_url', systemSettings.oidc_redirect_url)" />
          <div class="field-hint">需与身份提供方登记的回调地址完全一致。</div>
          <div class="effective-url">
            <span>当前有效回调地址</span>
            <code>{{ systemSettings.oidc_redirect_url_effective || '尚未生成' }}</code>
          </div>
        </div>
      </div>

      <div class="setting-row mt-4">
        <div><p class="setting-row-title">首次登录自动创建用户</p><p class="setting-row-hint">关闭后，尚未绑定本地账户的 OIDC 用户无法登录。</p></div>
        <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.oidc_auto_provision" class="sr-only peer" @change="handleSwitchChange('oidc_auto_provision', systemSettings.oidc_auto_provision)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
      </div>
    </div>

    <!-- ========== CAS ========== -->
    <div class="auth-card">
      <div class="auth-card-head">
        <div class="min-w-0">
          <p class="auth-card-title">CAS 登录</p>
          <p class="auth-card-desc">CAS 3.0 协议，固定使用 <code>/p3/serviceValidate</code> 校验 XML 响应。</p>
        </div>
        <label class="auth-switch">
          <input type="checkbox" v-model="systemSettings.cas_enable" class="sr-only peer" @change="handleSwitchChange('cas_enable', systemSettings.cas_enable)">
          <div class="switch-track"></div>
          <div class="switch-thumb"></div>
        </label>
      </div>

      <div class="grid gap-4 md:grid-cols-2">
        <div class="setting-group md:col-span-2">
          <label class="field-label" for="cas_server_url">CAS Server URL</label>
          <input id="cas_server_url" v-model="systemSettings.cas_server_url" type="url" class="input-modern" placeholder="https://cas.example.com/cas" @blur="handleFieldBlur('cas_server_url', systemSettings.cas_server_url)" />
          <div class="field-hint">填写 CAS 服务根地址，无需附加 <code>/login</code> 或 <code>/p3/serviceValidate</code>。</div>
        </div>
        <div class="setting-group">
          <label class="field-label" for="cas_display_name">登录按钮名称</label>
          <input id="cas_display_name" v-model="systemSettings.cas_display_name" type="text" class="input-modern" placeholder="CAS 登录" @blur="handleFieldBlur('cas_display_name', systemSettings.cas_display_name)" />
        </div>
        <div class="setting-group">
          <label class="field-label" for="cas_super_admin_username">映射超级管理员用户名</label>
          <input id="cas_super_admin_username" v-model="systemSettings.cas_super_admin_username" type="text" maxlength="255" class="input-modern" placeholder="留空表示不映射" @blur="handleFieldBlur('cas_super_admin_username', systemSettings.cas_super_admin_username)" />
          <div class="field-hint">CAS3 XML 的 <code>&lt;cas:user&gt;</code> 与此值一致时登录本地超管（区分大小写）。</div>
        </div>
        <div class="setting-group md:col-span-2">
          <label class="field-label" for="cas_service_url">Service URL</label>
          <input id="cas_service_url" v-model="systemSettings.cas_service_url" type="url" class="input-modern" placeholder="https://img.example.com/api/auth/cas/callback" @blur="handleFieldBlur('cas_service_url', systemSettings.cas_service_url)" />
          <div class="field-hint">需在 CAS 服务端的允许列表中登记该地址。</div>
          <div class="effective-url">
            <span>当前有效 Service 地址</span>
            <code>{{ systemSettings.cas_service_url_effective || '尚未生成' }}</code>
          </div>
        </div>
      </div>

      <div class="setting-row mt-4">
        <div><p class="setting-row-title">首次登录自动创建用户</p><p class="setting-row-hint">关闭后，尚未绑定本地账户的 CAS 用户无法登录。</p></div>
        <label class="relative inline-flex cursor-pointer items-center self-end md:self-center"><input type="checkbox" v-model="systemSettings.cas_auto_provision" class="sr-only peer" @change="handleSwitchChange('cas_auto_provision', systemSettings.cas_auto_provision)"><div class="switch-track"></div><div class="switch-thumb"></div></label>
      </div>
    </div>
  </div>
</template>

<script setup>
import { inject } from 'vue';
import { settingsContext } from '@/utils/settingsAccess.js';

const { systemSettings, activeSettingsTab, handleFieldBlur, handleSwitchChange } = inject(settingsContext);
</script>

<style scoped>
/* 认证卡片：比外层 surface 再浅一层，用来区分「一张卡里有两个独立提供方」。
   左边框用主色细线标记，比整块背景染色更轻，也不影响内部输入框的对比度。 */
.auth-card {
  @apply space-y-4 p-3.5 sm:p-4;
  border: 1px solid rgb(226 232 240 / 0.8);
  border-radius: var(--radius-md);
  background: rgb(248 250 252);
}
.dark .auth-card {
  border-color: rgb(255 255 255 / 0.1);
  background: rgb(2 6 23);
}

.auth-card-head {
  @apply flex flex-col items-start gap-3 pb-3.5 sm:flex-row sm:items-center sm:justify-between;
  border-bottom: 1px solid rgb(226 232 240 / 0.8);
}
.dark .auth-card-head { border-bottom-color: rgb(255 255 255 / 0.1); }

.auth-card-title {
  @apply text-sm font-semibold text-slate-900 dark:text-white;
}

.auth-card-desc {
  @apply mt-1 text-xs leading-5 text-slate-500 dark:text-slate-400;
}

.auth-switch {
  @apply relative inline-flex shrink-0 cursor-pointer items-center self-end sm:self-center;
}

/* 有效地址回显：等宽字体 + 可换行，长 URL 不会把卡片撑破 */
.effective-url {
  @apply mt-2 flex flex-wrap items-baseline gap-x-2 gap-y-1 px-3 py-2 text-xs leading-5;
  border-radius: var(--radius-sm);
  background: rgb(241 245 249);
}
.dark .effective-url { background: rgb(255 255 255 / 0.05); }

.effective-url span {
  @apply shrink-0 font-medium text-slate-600 dark:text-slate-300;
}

.effective-url code {
  @apply break-all text-slate-700 dark:text-slate-200;
}
</style>
