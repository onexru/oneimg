import { selectorPage } from './renderBounds.js';
import { escapeHtml } from './html.js';
export function renderPreviewTags(tags, imageId, editable = true, page = 1) {
  const state = selectorPage(tags, page);
  const chips = state.items.map(tag => `<div class="px-2 py-0.5 rounded bg-primary/10 dark:bg-primary/20 text-primary text-xs" data-tag-id="${Number(tag.id)}" data-image-id="${Number(imageId)}"><span>${escapeHtml(tag.name)}</span>${editable ? `<button aria-label="删除标签" data-preview-action="deleteImageTag" data-preview-args="${escapeHtml(JSON.stringify(['event', String(Number(imageId)), String(Number(tag.id))]))}" class="ml-1 text-primary/70 hover:text-primary/30"><i class="ri-close-line text-xs"></i></button>` : ''}</div>`).join('');
  const controls = state.pages > 1 ? `<div class="inline-flex items-center gap-2 text-xs text-secondary"><button type="button" class="soft-button px-2 py-1" data-preview-tag-page="${state.page - 1}" ${state.page === 1 ? 'disabled' : ''}>上一页</button><span>${state.page} / ${state.pages}</span><button type="button" class="soft-button px-2 py-1" data-preview-tag-page="${state.page + 1}" ${state.page === state.pages ? 'disabled' : ''}>下一页</button></div>` : '';
  return { html: chips + controls, ...state };
}
