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

export function galleryPageNumbers(page, total) {
  const current = positiveLimit(page, 1); const last = positiveLimit(total, 1);
  return Array.from({ length: Math.max(0, Math.min(last, current + 2) - Math.max(1, current - 2) + 1) }, (_, i) => Math.max(1, current - 2) + i);
}
