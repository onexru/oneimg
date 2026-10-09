export const SETTINGS_TABS = Object.freeze([
  { key: 'storage', label: '上传与存储', icon: 'ri-upload-cloud-2-line', perm: 'setting:upload' },
  { key: 'image', label: '图片处理', icon: 'ri-image-line', perm: 'setting:image' },
  { key: 'security', label: '安全与登录', icon: 'ri-shield-keyhole-line', perm: 'setting:security' },
  { key: 'notifications', label: '通知', icon: 'ri-notification-3-line', perm: 'setting:notification' },
  { key: 'api', label: 'API', icon: 'ri-code-s-slash-line', perm: 'setting:api' },
  { key: 'seo', label: '站点SEO', icon: 'ri-seo-line', perm: 'setting:seo' },
]);
export const settingsContext = Symbol('settings');
export const visibleSettingsTabs = permissions => SETTINGS_TABS.filter(tab =>
  Array.isArray(permissions) && permissions.includes(tab.perm));
export function settingsAccess(status, result) {
  if (status === 401) return { state: 'unauthenticated', notice: '登录已过期，请重新登录后查看系统设置。' };
  if (status === 403) return { state: 'denied', notice: result?.message || '无权查看系统设置，请联系管理员授予“查看设置”（setting:list）权限。' };
  if (status < 200 || status >= 300 || result?.code !== 200) return {
    state: 'error', notice: result?.message || '获取设置失败，请重新加载设置。' };
  return visibleSettingsTabs(result.setting_permissions).length
    ? { state: 'ready', notice: '' }
    : { state: 'readonly', notice: '可查看设置，但没有可编辑的设置分类；请联系管理员授予相应权限。' };
}
