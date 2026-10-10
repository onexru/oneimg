// Presentation only: server summary is authoritative; never infer grants from UI counts.
export function userStorageSummary(user) {
  const s = user?.access_summary;
  if (!s || !Number.isSafeInteger(s.storage_count) || s.storage_count < 0) return { label: '存储信息待刷新', hint: '未获取有效存储信息，请刷新重试' };
  if (s.storage_mode === 'multi') return {
    label: s.local_storage_included ? (s.storage_count ? `本机 + ${s.storage_count} 个同步源` : '仅本机存储') : `${s.storage_count} 个同步源`,
    hint: '多存储模式先保存到本机；这里只统计角色继承与个人额外分配中当前可用的远程同步目标。',
  };
  if (s.storage_mode !== 'single') return { label: '存储信息待刷新', hint: '存储模式未知，请刷新重试' };
  return { label: `${s.storage_count} 个可用存储`, hint: `${s.default_storage_included ? '包含系统默认存储。' : ''}按当前上传可用范围统计，已排除停用或不可选的存储；不等于额外授权数量。` };
}
export function userPermissionSummary(user) {
  const s = user?.access_summary;
  if (!s || !Number.isSafeInteger(s.additional_permission_count) || s.additional_permission_count < 0) return { label: '权限信息待刷新', hint: '未获取有效权限信息，请刷新重试' };
  if (s.all_management_permissions) return { label: '全部管理权限', hint: '由超级管理员身份或全局管理授权提供，不依赖逐项勾选的额外权限。' };
  const base = Number(user.role) === 1 ? '管理员基础权限' : '基础权限';
  return { label: s.additional_permission_count ? `${base} + ${s.additional_permission_count} 项授权` : base,
    hint: `包含上传及管理本人图片的基础能力${Number(user.role) === 1 ? '、管理员查看入口' : ''}；${s.additional_permission_count ? `另有 ${s.additional_permission_count} 项额外功能授权。` : '未额外分配功能权限，不代表没有使用权限。'}` };
}
