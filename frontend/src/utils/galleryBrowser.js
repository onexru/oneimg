// Keep personal ownership and management filtering separate, including direct URLs.
export const GALLERY_SORT_FIELDS = ['created_at', 'filename', 'file_size'];
export const GALLERY_ROLES = ['all', 'admin', 'user', 'guest'];
export function galleryFolderFromQuery(query = {}, authenticated = true) {
  if (!authenticated) return '';
  if (query.folder_id === '') return ''; // explicit All Images shortcut
  const value = query.folder_id;
  return typeof value === 'string' && /^(0|[1-9]\d*)$/.test(value) && Number.isSafeInteger(Number(value)) ? value : '0';
}
export function galleryQuery(query = {}, management = false) {
  return {
    filename: typeof query.search === 'string' ? query.search : '',
    sortBy: GALLERY_SORT_FIELDS.includes(query.sort_by) ? query.sort_by : 'created_at',
    sortOrder: query.sort_order === 'asc' ? 'asc' : 'desc',
    role: management && GALLERY_ROLES.includes(query.role) ? query.role : 'all',
    bucket: typeof query.bucket === 'string' && /^(0|[1-9]\d*)$/.test(query.bucket) ? query.bucket : '',
    deletion: ['all', 'active', 'deleting'].includes(query.deletion) ? query.deletion : 'all',
    tags: typeof query.tags === 'string' ? [...new Set(query.tags.split(',').filter(value => /^(0|[1-9]\d*)$/.test(value)).map(Number).filter(Number.isSafeInteger))] : [],
  };
}
export function galleryImageParams({ management = false, authenticated = false, folder = '0', page = 1, limit = 30, ...filters }) {
  const query = galleryQuery({ search: filters.filename, sort_by: filters.sortBy, sort_order: filters.sortOrder, role: filters.role, bucket: String(filters.bucket ?? ''), deletion: filters.deletion, tags: String(filters.tags ?? '') }, management);
  const params = new URLSearchParams({ scope: management ? 'all' : 'mine', page: String(page), limit: String(limit), sort_by: query.sortBy, sort_order: query.sortOrder, deletion: query.deletion });
  if (query.filename.trim()) params.set('search', query.filename.trim());
  if (query.bucket) params.set('bucket', query.bucket);
  if (query.tags.length) params.set('tags', query.tags.join(','));
  if (management) params.set('role', query.role);
  else if (authenticated) {
    const folderId = galleryFolderFromQuery({ folder_id: folder });
    if (folderId !== '') params.set('folder_id', folderId);
  }
  return params;
}
export function galleryViewPreference(storage, management = false) {
  try { return storage.getItem(`oneimg-gallery-view-${management ? 'all' : 'mine'}`) === 'grid' ? 'grid' : 'list'; }
  catch { return 'list'; }
}
export function galleryUploader(image) {
  if (image.uploader_name || image.username) return image.uploader_name || image.username;
  const role = Number(image.uploader_role);
  if (role === 2 || !Number(image.user_id)) return '游客';
  return `${role === 1 ? '管理员' : '用户'} #${image.user_id}`;
}
export function parseGalleryPage(data) {
  if (!data || !Number.isSafeInteger(data.total) || data.total < 0 ||
      !(Array.isArray(data.images) || data.images === null && data.total === 0) ||
      !Number.isSafeInteger(data.total_pages) || data.total_pages < 0 ||
      (Array.isArray(data.images) && (data.images.length > data.total || data.total === 0 && data.images.length > 0))) {
    throw new Error('图片列表响应异常，请重试');
  }
  return { images: data.images || [], total: data.total, totalPages: Math.max(1, data.total_pages) };
}
// Abort superseded work AND fence response.json(), which may outlive abort.
export function createGalleryRequests() {
  let generation = 0, controller, stopped = false;
  function cancel() { generation++; controller?.abort(); controller = undefined; }
  return {
    cancel,
    start() {
      cancel();
      controller = new AbortController();
      const current = generation, signal = controller.signal;
      return { signal, isCurrent: () => !stopped && current === generation && !signal.aborted };
    },
    dispose() { stopped = true; cancel(); },
  };
}
