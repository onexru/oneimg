/** Server pagination is the gallery DOM bound; do not silently virtualize selectable rows. */
export const GALLERY_PAGE_SIZE = 30;
export const RECENT_IMAGE_LIMIT = 12;
export const PREVIEW_PAGE_SIZE = 24;
export const positiveLimit = (value, fallback) => {
  const n = Number(value);
  return Number.isSafeInteger(n) && n > 0 ? n : fallback;
};
export function boundedPage(items, limit = GALLERY_PAGE_SIZE) {
  return Array.isArray(items) ? items.slice(0, positiveLimit(limit, GALLERY_PAGE_SIZE)) : [];
}
export function mergeRecentImages(current, incoming, limit = RECENT_IMAGE_LIMIT) {
  const unique = new Map();
  for (const image of [...(incoming || []), ...(current || [])]) {
    if (image?.id != null && !unique.has(String(image.id))) unique.set(String(image.id), image);
  }
  return boundedPage([...unique.values()], limit);
}
/** Independent selector pages keep large preview lists bounded without losing selection. */
export function selectorPage(items, page = 1, size = PREVIEW_PAGE_SIZE) {
  const list = Array.isArray(items) ? items : [];
  const limit = positiveLimit(size, PREVIEW_PAGE_SIZE);
  const pages = Math.max(1, Math.ceil(list.length / limit));
  const current = Math.min(pages, positiveLimit(page, 1));
  return { items: list.slice((current - 1) * limit, current * limit), page: current, pages };
}

// 页码条：首尾 + 当前页左右各一页，中间断开的地方给省略号。
// 只渲染 current±2 的话，几十页的图库永远跳不到最后一页，只能一页页点「下一页」。
export function galleryPageNumbers(page, total) {
  const current = positiveLimit(page, 1); const last = positiveLimit(total, 1);
  if (last <= 7) return Array.from({ length: last }, (_, i) => i + 1);
  const wanted = [...new Set([1, current - 1, current, current + 1, last])]
    .filter(value => value >= 1 && value <= last)
    .sort((a, b) => a - b);
  const out = [];
  wanted.forEach((value, index) => {
    if (index > 0 && value - wanted[index - 1] > 1) out.push('...');
    out.push(value);
  });
  return out;
}
