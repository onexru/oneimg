export const DEFAULT_UPLOAD_LIMITS = Object.freeze({ max_upload_files: 10, tag_max_length: 10, random_image_limit: 20 });
export const RASTER_TYPES = Object.freeze(['image/jpeg', 'image/png', 'image/gif', 'image/webp']);
export function uploadLimits(config = {}) {
  const positive = (value, fallback) => Number.isSafeInteger(Number(value)) && Number(value) > 0 ? Number(value) : fallback;
  return Object.fromEntries(Object.entries(DEFAULT_UPLOAD_LIMITS).map(([key, fallback]) => [key, positive(config[key], fallback)]));
}
export function allowedRasterTypes(config = {}) {
  const allowed = Array.isArray(config.allowed_types) ? config.allowed_types : String(config.allowed_types || '').split(',');
  return allowed.length && allowed.some(type => String(type).trim())
    ? RASTER_TYPES.filter(type => allowed.map(value => String(value).trim()).includes(type)) : [...RASTER_TYPES];
}
export function fileValidationMessage(file, config = {}) {
  if (/\.svgz?$/i.test(file.name) || file.type === 'image/svg+xml') return `${file.name}：SVG 不允许上传，请使用 JPG、PNG、GIF 或 WebP`;
  if (!allowedRasterTypes(config).includes(file.type)) return `${file.name}：不支持的图片格式，仅支持服务器允许的 JPG、PNG、GIF 或 WebP`;
  if (Number(config.max_file_size) > 0 && file.size > Number(config.max_file_size)) return `${file.name}：文件大小超过服务器限制`;
  return '';
}
