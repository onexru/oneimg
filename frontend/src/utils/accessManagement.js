import { readApiResponse } from './apiFeedback.js';

const requestOptions = { credentials: 'same-origin', cache: 'no-store' };
export function validateAccessPolicy(data) {
  const bools = ['multi_storage_sync', 'guest_enabled', 'can_edit_guest_storage', 'can_assign_users'];
  const ids = ['guest_storage_id', 'effective_guest_storage_id', 'default_storage_id'];
  if (!data || bools.some(key => typeof data[key] !== 'boolean') ||
      ids.some(key => !Number.isSafeInteger(data[key]) || data[key] < 0) || !Array.isArray(data.buckets) ||
      !Array.isArray(data.roles) || ![1, 3, 2].every(role => data.roles.some(item => item.role === role)) ||
      data.roles.some(item => !['legacy', 'custom'].includes(item.mode) || !Array.isArray(item.bucket_ids) || !Array.isArray(item.effective_bucket_ids) || [...item.bucket_ids, ...item.effective_bucket_ids].some(id => !Number.isSafeInteger(id) || id <= 0)) ||
      data.buckets.some(bucket => !Number.isSafeInteger(bucket.id) || bucket.id <= 0 || typeof bucket.name !== 'string' ||
        typeof bucket.type !== 'string' || typeof bucket.disabled !== 'boolean' ||
        !Number.isFinite(bucket.capacity) || !Number.isFinite(bucket.usage))) {
    throw new Error('访问策略响应异常，请重新加载');
  }
  return data;
}
export async function loadAccessPolicy(fetcher = fetch, signal) {
  const result = await readApiResponse(await fetcher('/api/admin/access-policy', { ...requestOptions, signal }), '获取访问策略失败');
  return validateAccessPolicy(result.data);
}
export async function loadAccessUser(id, fetcher = fetch, signal) {
  const result = await readApiResponse(await fetcher(`/api/users?id=${id}`, { ...requestOptions, signal }), '获取用户权限失败');
  const user = result.data?.list?.find(item => item.id === id);
  if (!user?.permission || (user.permission.buckets != null && !Array.isArray(user.permission.buckets)) ||
      (user.permission.codes != null && !Array.isArray(user.permission.codes))) throw new Error('用户权限响应异常，请重新加载');
  return user;
}
export async function loadAccessActor(fetcher = fetch, signal) {
  const result = await readApiResponse(await fetcher('/api/user/status', { ...requestOptions, signal }), '无法确认当前登录身份');
  const id = result.data?.id ?? result.data?.user_id;
  if (!Number.isSafeInteger(id) || id <= 0 || result.data?.logged_in !== true) throw new Error('当前登录身份无效，请重新登录');
  return { ...result.data, id };
}
export function sameGrants(left, right) {
  const a = [...new Set(left || [])].sort(), b = [...new Set(right || [])].sort();
  return a.length === b.length && a.every((value, index) => value === b[index]);
}
export function userAccessPayload(mode, values) {
  if (mode === 'storage') return { permission: [...new Set(values)] };
  if (mode === 'permissions') return { codes: [...new Set(values)] };
  throw new Error('未知权限操作');
}
export async function writeUserAccess(id, mode, values, fetcher = fetch, signal) {
  await readApiResponse(await fetcher(`/api/users/updatePermission/${id}`, {
    ...requestOptions, signal, method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' },
    body: JSON.stringify(userAccessPayload(mode, values)),
  }), '保存用户授权失败');
}
export async function verifyUserAccess(id, mode, values, fetcher = fetch, signal) {
  const user = await loadAccessUser(id, fetcher, signal);
  if (!sameGrants(mode === 'storage' ? user.permission.buckets : user.permission.codes, values)) {
    throw new Error('读取结果与提交内容不一致，尚未确认保存；请核对后重新打开');
  }
  return user;
}
export function accessReadOnlyReason(mode, user, policy, actor) {
  if (!policy?.can_assign_users) return '当前账号没有编辑用户授权的权限。';
  if (mode === 'permissions') {
    if (user.id === 1 || user.access_summary?.all_management_permissions || user.permission?.codes?.includes('*')) return '全部管理权限由超级管理员身份或全局授权提供，此处只读。';
    if (user.id === actor?.id) return '不能在此修改当前登录账号的功能权限。';
  } else if (!policy.multi_storage_sync) {
    if (Number(user.role) === 1) return '单存储模式下，管理员存储由角色规则决定，请在存储管理中调整。';
    if (user.id === 1 || user.id === actor?.id) return '不能在此修改超级管理员或当前登录账号的存储授权。';
  }
  return '';
}
export function bucketLabel(policy, id) {
  const bucket = policy?.buckets?.find(item => item.id === id);
  return bucket ? `${bucket.name}${bucket.disabled ? '（已停用）' : ''}` : `存储源 #${id}（不可用）`;
}
export function inheritedUserBucketIds(user, policy) {
  if (Array.isArray(user.access_summary?.inherited_bucket_ids)) return user.access_summary.inherited_bucket_ids;
  const role = policy.roles?.find(item => item.role === Number(user.role));
  return role?.effective_bucket_ids || [];
}
export function availableUserBuckets(user, policy) {
  const inherited = new Set(inheritedUserBucketIds(user, policy));
  const extra = new Set(user.permission?.buckets || []);
  const role = policy.roles?.find(item => item.role === Number(user.role));
  const extraAllowed = Number(user.role) === 3 || (policy.multi_storage_sync && Number(user.role) !== 2);
  return policy.buckets.filter(bucket => {
    if (bucket.disabled || (policy.multi_storage_sync && bucket.type === 'default')) return false;
    if (!inherited.has(bucket.id) && !(extraAllowed && extra.has(bucket.id))) return false;
    if (bucket.capacity > 0 && bucket.usage >= bucket.capacity && (role?.mode === 'custom' || (!policy.multi_storage_sync && !inherited.has(bucket.id)))) return false;
    return true;
  });
}
export function userStorageNames(user, policy, error = '') {
  if (!policy) return error ? '存储名称加载失败，请重试访问策略' : '存储名称加载中…';
  const available = availableUserBuckets(user, policy).map(bucket => bucket.name);
  if (policy.multi_storage_sync) {
    const local = policy.buckets.find(bucket => bucket.type === 'default');
    if (local) available.unshift(`本机（${local.name}）`);
  }
  return available.length ? available.join('、') : '暂无可用存储源';
}
export function guestStorageTarget(policy) {
  if (!policy) return '';
  const guest = policy.roles?.find(item => item.role === 2);
  if (!guest) return '角色存储信息不可用，请刷新';
  const names = guest.effective_bucket_ids.map(id => bucketLabel(policy, id));
  if (policy.multi_storage_sync) names.unshift('本机固定保存');
  return `${names.join('、') || '暂无可用存储源'} · ${guest.mode === 'legacy' ? '沿用原有规则' : '自定义多选'}`;
}
