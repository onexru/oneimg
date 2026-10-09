import { readApiResponse } from './apiFeedback.js';
import { sameGrants } from './accessManagement.js';
const options = { credentials: 'same-origin', cache: 'no-store' };
export function validateStorageAssignments(data) {
  if (!data || typeof data.multi_storage_sync !== 'boolean' || typeof data.can_edit !== 'boolean' || typeof data.guest_enabled !== 'boolean' ||
      !Array.isArray(data.buckets) || !Array.isArray(data.roles) || data.roles.length !== 3 ||
      ![1, 3, 2].every(role => data.roles.some(item => item.role === role)) ||
      data.roles.some(item => !['legacy', 'custom'].includes(item.mode) || !Array.isArray(item.bucket_ids) || !Array.isArray(item.effective_bucket_ids) ||
        [...item.bucket_ids, ...item.effective_bucket_ids].some(id => !Number.isSafeInteger(id) || id <= 0)) ||
      data.buckets.some(bucket => !Number.isSafeInteger(bucket.id) || bucket.id <= 0 || typeof bucket.name !== 'string' || typeof bucket.type !== 'string' || typeof bucket.disabled !== 'boolean')) {
    throw new Error('角色存储分配响应异常，请重试');
  }
  return data;
}
export async function loadStorageAssignments(fetcher = fetch, signal) {
  const result = await readApiResponse(await fetcher('/api/admin/storage-assignments', { ...options, signal }), '获取角色存储分配失败');
  return validateStorageAssignments(result.data);
}
export function sameRoleAssignment(left, right) {
  return left?.mode === right?.mode && sameGrants(left?.bucket_ids, right?.bucket_ids);
}
export async function writeRoleAssignment(role, value, fetcher = fetch, signal) {
  if (![1, 3, 2].includes(role) || !['legacy', 'custom'].includes(value.mode) || !Array.isArray(value.bucket_ids)) throw new Error('角色存储分配参数无效');
  return readApiResponse(await fetcher(`/api/admin/storage-assignments/${role}`, {
    ...options, signal, method: 'PUT', headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' },
    body: JSON.stringify({ mode: value.mode, bucket_ids: value.mode === 'legacy' ? [] : [...value.bucket_ids] }),
  }), '保存角色存储分配失败');
}
export async function verifyRoleAssignment(role, expected, fetcher = fetch, signal) {
  const policy = await loadStorageAssignments(fetcher, signal);
  if (!sameRoleAssignment(policy.roles.find(item => item.role === role), expected)) throw new Error('读取的角色分配与提交值不一致，尚未确认保存');
  return policy;
}
