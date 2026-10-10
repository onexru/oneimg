/** Single lazy preview renderer for Home and Gallery; all external text/URLs escaped here. */
import { renderPreviewTags } from './previewTags.js';
import { escapeHtml, safeResourceUrl } from './html.js';
import { getStorageSyncSummary, renderStorageStatusesHtml } from './storageStatus.js';
const STORAGE_MAP = { default: '本地' };
export function renderImagePreview(image, { multiStorageSync = false, buckets = [], imageUrl = '', errorImg = '',
  editableTags = true, headerTitle = false, showLink = false, formatFileSize = String, formatDate = String } = {}) {
  const roleClass = image.user_id == '1'
    ? 'background-color: #e0f2fe; color: #0369a1; dark:background-color: #075985; dark:color: #bae6fd;'
    : 'background-color: #dcfce7; color: #166534; dark:background-color: #14532d; dark:color: #bbf7d0;';
  const syncSummary = getStorageSyncSummary(image);
  const tagsHtml = renderPreviewTags(image.tags, image.id, editableTags).html;
  const headerStorageHtml = multiStorageSync ? `
    <span class="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs ${syncSummary.badgeClass}">
      <i class="${syncSummary.icon}"></i>${syncSummary.label}
    </span>
  ` : `
    <span class="rounded bg-success px-2 py-0.5 text-xs text-white">
      ${escapeHtml(buckets.find(bucket => bucket.id == image.bucket_id)?.name || '未知存储')}
    </span>
  `;
  const syncStatusHtml = multiStorageSync ? `
    <div class="mt-3 border-t border-slate-200/70 pt-3 dark:border-white/10">
      <div class="mb-2 flex items-center gap-2 text-xs font-semibold text-slate-700 dark:text-slate-200">
        <i class="ri-cloud-line"></i>存储同步状态
      </div>
      <div class="grid gap-2 sm:grid-cols-2">
        <div class="rounded-xl border border-emerald-200 bg-emerald-50 px-3 py-2 dark:border-emerald-500/20 dark:bg-emerald-500/10">
          <div class="flex items-center justify-between gap-2 text-xs">
            <span class="font-medium text-emerald-800 dark:text-emerald-200">本机</span>
            <span class="inline-flex items-center gap-1 text-emerald-700 dark:text-emerald-300"><i class="ri-checkbox-circle-line"></i>已保存</span>
          </div>
        </div>
        ${renderStorageStatusesHtml(image)}
      </div>
    </div>
  ` : '';
  const legacyStorageHtml = !multiStorageSync ? `
    <div class="flex items-center gap-1.5">
      <i class="ri-hard-drive-3-line"></i>
      存储: ${escapeHtml(STORAGE_MAP[image.storage] || image.storage || '未知')}
    </div>
  ` : '';

  const markup = `
    <div data-preview-error-image="${escapeHtml(safeResourceUrl(errorImg))}" class="image-preview-popup w-full max-w-5xl max-h-[85vh] flex flex-col overflow-hidden bg-white dark:bg-dark-200">
      <div class="preview-header bg-light-50 pb-2 flex justify-between items-center">
        ${headerTitle ? `<h3 class="text-xs font-medium truncate max-w-[50%]">${escapeHtml(image.filename)}</h3>` : `<div class="flex min-w-0 items-center gap-2">
          <span class="text-xs px-2 py-0.5 rounded" style="${roleClass}">
            ${image.uploader_role == '1' ? '管理员' : (image.uploader_role == '3' ? '用户' : '游客') }
          </span>
          ${headerStorageHtml}
        </div>`}
        <div class="flex gap-1">
          <button
            class="px-3 py-1.5 text-xs bg-light-100 dark:bg-dark-300 hover:bg-light-200 whitespace-nowrap dark:hover:bg-dark-400 text-secondary rounded-md transition-colors duration-200 flex items-center gap-1"
            data-preview-action="downloadPreviewImage" data-preview-args="${escapeHtml(JSON.stringify([]))}"
          >
            <i class="ri-download-fill text-xs"></i>
            下载
          </button>
          <button
            class="px-3 py-1.5 text-xs bg-danger/10 hover:bg-danger/20 whitespace-nowrap text-danger rounded-md transition-colors duration-200 flex items-center gap-1"
            data-preview-action="deletePreviewImage" data-preview-args="${escapeHtml(JSON.stringify([String(Number(image.id))]))}"
          >
            <i class="ri-delete-bin-fill text-xs"></i>
            删除
          </button>
        </div>
      </div>
      <div class="max-h-[360px] flex-1 overflow-auto flex items-center justify-center">
        <a
          class="spotlight min-w-full max-w-full min-h-[260px] block"
          href="${escapeHtml(safeResourceUrl(imageUrl))}"
          data-description="尺寸: ${escapeHtml(image.width || '未知')}×${escapeHtml(image.height || '未知')} | 大小: ${escapeHtml(formatFileSize(image.file_size || 0))} | 上传日期：${escapeHtml(formatDate(image.created_at))} | 角色：${image.uploader_role == '1' ? '管理员' : (image.uploader_role == '3' ? '用户' : '游客')}"
        >
          <div class="relative max-w-full w-fill max-h-[360px] min-h-[260px] rounded-lg overflow-hidden animate-pulse flex items-center justify-center">
            <div class="absolute inset-0 flex items-center justify-center">
              <svg class="w-10 h-10 text-slate-300 animate-spin loading-svg" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="transform: scaleX(-1) scaleY(-1);">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </div>
            <img loading="lazy" decoding="async"
              src="${escapeHtml(safeResourceUrl(image.thumbnail || imageUrl))}"
              alt="${escapeHtml(image.filename)}"
              class="max-w-full w-fill max-h-[360px] min-h-[260px] object-contain rounded-lg relative z-10 opacity-0 transition-opacity duration-300"


            />
          </div>
        </a>
      </div>
      ${showLink ? `<label class="mt-3 block text-xs text-secondary">
        图片链接
        <input type="text" readonly aria-label="图片链接" value="${escapeHtml(safeResourceUrl(imageUrl))}"
          class="input-modern mt-1 w-full min-w-0 text-xs" />
      </label>` : ''}
      <div class="flex gap-1 flex-wrap items-center w-full mt-3 mb-3">
        <p class="mr-1 text-xs text-secondary font-semibold">复制：</p>
        <button
          data-preview-action="copyPreviewImageLink" data-preview-args="${escapeHtml(JSON.stringify(["'url'"]))}"
          class="px-2 py-1 text-xs bg-primary shadow-md text-white dark:bg-dark-300 hover:bg-blue-800 rounded transition-colors duration-200">
          <i class="ri-link text-xs w-4 text-center text-white"></i> URL
        </button>
        <button
          data-preview-action="copyPreviewImageLink" data-preview-args="${escapeHtml(JSON.stringify(["'html'"]))}"
          class="px-2 py-1 text-xs bg-primary shadow-md text-white dark:bg-dark-300 hover:bg-blue-800 rounded transition-colors duration-200">
          <i class="ri-code-fill text-xs w-4 text-center text-white"></i> HTML
        </button>
        <button
          data-preview-action="copyPreviewImageLink" data-preview-args="${escapeHtml(JSON.stringify(["'markdown'"]))}"
          class="px-2 py-1 text-xs bg-primary shadow-md text-white dark:bg-dark-300 hover:bg-blue-800 rounded transition-colors duration-200">
          <i class="ri-markdown-fill text-xs w-4 text-center text-white"></i> Markdown
        </button>
      </div>
      <div class="pt-2 flex flex-wrap gap-2 items-center">
        <p class="mr-1 text-xs text-secondary font-semibold">Tags：</p>
        <span data-preview-tags class="contents">${tagsHtml}</span>
        ${editableTags ? `<button aria-label="添加标签" data-preview-action="addImageTag" data-preview-args="${escapeHtml(JSON.stringify([String(Number(image.id))]))}"
          class="flex items-center px-2 py-1 bg-success/10 dark:bg-success/20 text-success rounded-full text-xs hover:text-success/30 transition-colors"><i class="ri-add-line"></i></button>` : ''}
      </div>
      ${syncStatusHtml}
      <div class="pt-2 flex flex-wrap gap-2 text-xs text-secondary">
        <div class="flex items-center gap-1.5">
          <i class="ri-ruler-line w-3.5 text-center"></i>
          尺寸: ${escapeHtml(image.width || '未知')}×${escapeHtml(image.height || '未知')}
        </div>
        <div class="flex items-center gap-1.5">
          <i class="ri-image-line w-3.5 text-center"></i>
          大小: ${escapeHtml(formatFileSize(image.file_size || 0))}
        </div>
        ${legacyStorageHtml}
        <div class="flex items-center gap-1.5">
          <i class="ri-user-line"></i>
          角色: ${image.uploader_role == '1' ? '管理员' : (image.uploader_role == '3' ? '用户' : '游客')}
        </div>
      </div>
    </div>
  `;
  return markup;
}

/** Delegation stays on this one preview root; no globals/inline scripts survive CSP. */
export function bindImagePreview(root, actions = {}, { image, editableTags = true } = {}) {
  root.addEventListener('click', event => {
    const pageButton = event.target.closest('[data-preview-tag-page]');
    if (pageButton && image) {
      root.querySelector('[data-preview-tags]').innerHTML = renderPreviewTags(image.tags, image.id, editableTags, Number(pageButton.dataset.previewTagPage)).html;
      return;
    }
    const target = event.target.closest('[data-preview-action]');
    if (!target || !root.contains(target)) return;
    const callback = actions[target.dataset.previewAction];
    if (typeof callback !== 'function') return;
    const args = JSON.parse(target.dataset.previewArgs || '[]').map(part => {
      if (part === 'event') return event;
      if (/^['"].*['"]$/.test(part)) return part.slice(1, -1);
      return Number(part);
    });
    callback(...args);
  });
  root.querySelectorAll('img').forEach(image => {
    const reveal = () => {
      image.classList.remove('opacity-0');
      image.parentElement.classList.remove('animate-pulse');
      image.parentElement.querySelector('.loading-svg')?.classList.add('hidden');
    };
    image.addEventListener('load', reveal);
    image.addEventListener('error', () => {
      reveal();
      if (image.dataset.fallbackApplied) return;
      image.dataset.fallbackApplied = 'true';
      image.src = root.querySelector('[data-preview-error-image]')?.dataset.previewErrorImage || '';
    });
    if (image.complete) reveal();
  });
}
