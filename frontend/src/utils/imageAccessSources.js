import { getStorageDisplayName } from './storageStatus.js';
export const getSelectedAccessBucketId = (image) => {
  const selected = Number(image?.access_bucket_id || 0);
  if (selected > 0) return selected;
  const localSource = Array.isArray(image?.storage_statuses)
    ? image.storage_statuses.find(source => source?.status === 'success' && source?.bucket_type === 'default' && !source?.bucket_disabled)
    : null;
  return Number(localSource?.bucket_id || image?.bucket_id || 0);
};

export const getAccessSourceOptions = (image, buckets = []) => {
  const statuses = Array.isArray(image?.storage_statuses) ? image.storage_statuses : [];
  const options = statuses
    .filter(source => source?.bucket_id && source.status === 'success')
    .map(source => ({ ...source }));
  const selectedBucketId = getSelectedAccessBucketId(image);

  if (!options.some(source => Number(source.bucket_id) === selectedBucketId)) {
    const selectedStatus = statuses.find(source => Number(source?.bucket_id) === selectedBucketId);
    if (selectedStatus) {
      options.push({ ...selectedStatus, access_unavailable: true });
    }
  }

  if (options.length === 0 && image?.bucket_id) {
    const bucket = buckets.find(item => Number(item.id) === Number(image.bucket_id));
    options.push({
      bucket_id: Number(image.bucket_id),
      bucket_name: bucket?.name || `存储源 #${image.bucket_id}`,
      bucket_type: bucket?.type || image.storage,
      bucket_disabled: bucket?.disabled === true,
      status: 'success',
    });
  }

  const unique = new Map();
  options.forEach(source => unique.set(Number(source.bucket_id), source));
  return Array.from(unique.values()).sort((left, right) => {
    if (left.bucket_type === 'default' && right.bucket_type !== 'default') return -1;
    if (left.bucket_type !== 'default' && right.bucket_type === 'default') return 1;
    return Number(left.bucket_id) - Number(right.bucket_id);
  });
};

export const getAccessSourceOptionLabel = (source) => {
  const name = source?.bucket_type === 'default'
    ? `${source?.bucket_name || '本机'}（默认）`
    : getStorageDisplayName(source);
  if (source?.bucket_disabled) return `${name}（已停用，回退本机）`;
  if (source?.access_unavailable) return `${name}（不可用，回退本机）`;
  return name;
};
