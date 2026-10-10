import { readApiResponse } from './apiFeedback.js';

export const FOLDER_LIMIT = 200;
export function folderAccount() {
  try {
    const user = JSON.parse(localStorage.getItem('userInfo') || '{}');
    return [1, 3].includes(Number(user.role)) && Number(user.id) > 0 ? String(user.id) : '';
  } catch { return ''; }
}

// Empty means all images; zero is deliberately distinct and means unfiled.
export function folderFilter(value, fallback = '') {
  if (typeof value !== 'string' && typeof value !== 'number') return fallback;
  const text = String(value);
  return /^(0|[1-9]\d*)$/.test(text) && Number.isSafeInteger(Number(text)) ? text : fallback;
}
export function folderNameError(value, folders = [], excludeId = null) {
  const name = value.trim();
  if (!name || [...name].length > 64) return '文件夹名称需为 1–64 个字符';
  if (/[\\/\u0000-\u001f\u007f-\u009f]/u.test(name) || /^\.+$/.test(name)) return '名称不能包含路径分隔符、控制字符，也不能只包含点';
  if (folders.some(folder => String(folder.id) !== String(excludeId) && folder.name.trim().toLowerCase() === name.toLowerCase())) return '已有同名文件夹';
  return '';
}
export function folderDescriptionError(value = '') {
  if ([...value.trim()].length > 500) return '文件夹描述最多 500 个字符';
  if (/[\u0000-\u0008\u000b-\u001f\u007f-\u009f\p{Cf}]/u.test(value.replaceAll('\r\n', '\n'))) return '描述不能包含不可见控制字符';
  return '';
}
export function folderLabel(folder) {
  return `${folder.name}${folder.deleting ? '（删除未完成）' : ''}`;
}
export function ownImageIds(images, selectedIds, accountId, folders = []) {
  if (!accountId) return [];
  const deleting = new Set(folders.filter(folder => folder.deleting).map(folder => String(folder.id)));
  const own = new Set(images.filter(image => String(image.user_id) === String(accountId) && !image.deleting && !deleting.has(String(image.folder_id))).map(image => String(image.id)));
  return selectedIds.filter(id => own.has(String(id)));
}
// Only the explicitly documented incomplete response may continue the SAME
// confirmed operation. Never retry transport errors, busy leases or storage failures.
export async function continueFolderDeletion(api, id, { confirmName, signal, isCurrent = () => true, onProgress = () => {} }) {
  const confirmed = { deleteImages: true, confirmName, signal };
  let previous, deletedCount = 0;
  const checkCurrent = () => {
    if (signal?.aborted || !isCurrent()) throw new DOMException('删除请求已停止', 'AbortError');
  };
  const report = data => {
    if (Number.isSafeInteger(data?.deleted_count) && data.deleted_count >= 0) deletedCount += data.deleted_count;
    if (data) onProgress({ ...data, total_deleted_count: deletedCount });
  };
  for (;;) {
    checkCurrent();
    let data;
    try {
      data = await api.remove(id, confirmed);
    } catch (error) {
      checkCurrent();
      report(error.data);
      const next = error.data;
      if (error.status !== 409 || error.code !== 'folder_delete_incomplete' || next?.deleted !== false || next?.deleting !== true || next?.retryable !== true || next?.failed_count !== 0) throw error;
      const counts = [next.remaining_count, next.pending_upload_count];
      if (!counts.every(value => Number.isSafeInteger(value) && value >= 0) || !counts.some(value => value > 0)) throw error;
      // An identical/increasing checkpoint is not progress, even if the server
      // repeats a positive per-invocation deleted_count. Stop rather than loop.
      if (previous && (!counts.every((value, index) => value <= previous[index]) || !counts.some((value, index) => value < previous[index]))) {
        error.message += '；剩余数量未减少，已停止自动继续，请检查后重试';
        throw error;
      }
      previous = counts;
      continue;
    }
    checkCurrent();
    report(data);
    if (data?.deleted !== true) {
      const error = new Error('服务器未确认文件夹已删除，请刷新后重试');
      error.data = data;
      throw error;
    }
    return data;
  }
}
export function createFolderApi(baseUrl = '', fetchImpl = (...args) => fetch(...args)) {
  async function request(path, method = 'GET', body, signal) {
    const response = await fetchImpl(`${baseUrl}/api${path}`, {
      method, credentials: 'same-origin', ...(signal ? { signal } : {}),
      headers: { 'X-Requested-With': 'XMLHttpRequest', ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) },
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    });
    return (await readApiResponse(response, '文件夹操作失败，请重试')).data;
  }
  return {
    list: () => request('/folders'),
    create: (name, description) => request('/folders', 'POST', { name: name.trim(), ...(description === undefined ? {} : { description: description.trim() }) }),
    rename: (id, name, description) => request(`/folders/${encodeURIComponent(id)}`, 'PUT', { name: name.trim(), ...(description === undefined ? {} : { description: description.trim() }) }),
    remove: (id, { deleteImages = false, confirmName = '', signal } = {}) => request(`/folders/${encodeURIComponent(id)}`, 'DELETE', deleteImages ? { delete_images: true, confirm_name: confirmName } : undefined, signal),
    move: (imageIds, folderId) => request('/images/folder', 'PUT', { image_ids: [...imageIds], folder_id: Number(folderFilter(folderId, '0')) }),
  };
}
