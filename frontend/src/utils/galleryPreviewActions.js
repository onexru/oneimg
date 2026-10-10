import { readApiResponse } from './apiFeedback.js';
import { createDialogScope } from './dialogScope.js';
import Message from './message.js';
import { escapeHtml, safeResourceUrl } from './html.js';
import { renderImagePreview, bindImagePreview } from './imagePreview.js';
import { copyToClipboard } from './clipboard.js';

export function createGalleryPreviewActions({ API_BASE_URL, images, presetTags, presetBuckets, currentPreviewImage, multiStorageSync, errorImg, formatFileSize, formatDate, getFullUrl, deleteAsync, serializeForm }) {
const { Dialog: PopupModal, dispose } = createDialogScope();
const showFormModal = function(options) { return new PopupModal({ type: 'form', ...options }); };
let previewModal = null;
let disposed = false;
const pustImageTag = async (imageId, values) => {
  const { tag } = values;
  if (tag === '0') {
    Message.warning('请选择Tag标签');
    return;
  }
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/tag`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest'
      },
      body: JSON.stringify({ id: imageId, tag })
    });
    const result = await readApiResponse(response, '请求失败');
    if (response.ok && result.code === 200) {
      const image = images.value.find(item => item.id === imageId);
      if (image) {
        image.tags = image.tags.filter(item => item.id !== 0);
        const newTag = presetTags.value.find(item => item.id === Number(tag));
        if (newTag) image.tags.push(newTag);
        currentPreviewImage.value = image;
        if (image) openPreview(image);
      }
      Message.success(result.message || '添加成功');
    } else {
      openPreview(currentPreviewImage.value);
      throw new Error(result.message || '添加失败');
    }
  } catch (err) {
    Message.error(err.message || '添加失败，请稍后重试');
  }
};

const deleteImageTagAsync = async (imageId, tagId) => {
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/tag`, {
      method: 'DELETE',
      headers: {
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest'
      },
      body: JSON.stringify({ id: imageId, tag: tagId })
    });
    const result = await readApiResponse(response, '请求失败');
    if (response.ok && result.code === 200) {
      Message.success(result.message || '删除成功');
      return true;
    } else {
      Message.error(result.message || '删除失败');
      return false;
    }
  } catch (err) {
    Message.error(`出错了：${err.message}`);
    console.warn(err);
    return false;
  }
};

const openPreview = (image) => {
  if (disposed || !image) return;
  previewModal?.close();
  currentPreviewImage.value = image;
  const previewContent = generatePreviewContent(image);
  const customModal = new PopupModal({
    title: image.filename,
    content: previewContent,
    type: 'default',
    buttons: [
      {
        text: '确定',
        type: 'default',
        callback: (modal) => {
          modal.close();
          cleanPreviewGlobalFunctions();
        }
      }
    ],
    maskClose: true,
    zIndex: 10000,
    maxHeight: '90vh',
    onClose: () => { if (previewModal === customModal) { previewModal = null; currentPreviewImage.value = null; } }
  });
  previewModal = customModal;
  registerPreviewGlobalFunctions(customModal, image.id);
  customModal.open();
};

const generatePreviewContent = image => renderImagePreview(image, {
  multiStorageSync: multiStorageSync.value, buckets: presetBuckets.value,
  imageUrl: getFullUrl(image.url), errorImg, formatFileSize, formatDate,
});
/**
 * 清理预览相关资源
 */
const cleanupPreview = () => { previewModal?.close(); };
const registerPreviewGlobalFunctions = (modal, imageId) => {
  const actions = {};
  actions.copyPreviewImageLink = (type) => {
    if (!currentPreviewImage.value) return;
    const image = currentPreviewImage.value;
    const fullUrl = getFullUrl(image.url);
    let copyText = '';
    switch (type) {
      case 'url': copyText = fullUrl; break;
      case 'html': copyText = `<img loading="lazy" decoding="async" src="${fullUrl}" alt="${escapeHtml(image.filename)}">`; break;
      case 'markdown': copyText = `![${escapeHtml(image.filename)}](${fullUrl})`; break;
      default: copyText = fullUrl;
    }
    copyToClipboard(copyText).then(ok => {
      if (ok) {
        Message.success('已复制到剪贴板');
      } else {
        Message.error('复制失败');
      }
    });
  };

  actions.downloadPreviewImage = () => {
    if (!currentPreviewImage.value) return;
    const a = document.createElement('a');
    const downloadUrl = new URL(getFullUrl(currentPreviewImage.value.url), window.location.origin);
    downloadUrl.searchParams.set('download', '1');
    a.href = downloadUrl.toString();
    a.download = currentPreviewImage.value.filename || 'image';
    a.target = '_blank';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  actions.deletePreviewImage = (id) => {
    actions.closePreviewModal();
    const modal = new PopupModal({
      title: '删除确认',
      content: `
        <div class="flex gap-3">
          <i class="fa fa-exclamation-triangle text-warning text-xl mt-1"></i>
          <div>
            <p>确定要删除这张图片吗？</p>
            <p class="mt-1 text-secondary text-sm">删除后无法恢复</p>
          </div>
        </div>
      `,
      buttons: [
        {
          text: '取消',
          type: 'default',
          callback: (m) => {
            m.close();
            const image = images.value.find(item => item.id === id);
            openPreview(image);
          }
        },
        {
          text: '确认删除',
          type: 'danger',
          callback: async (m) => {
            m.close();
            await deleteAsync(id);
          }
        }
      ],
      maskClose: true
    });
    modal.open();
  };

  actions.deleteImageTag = (event, imageId, tagId) => {
    if (tagId == 0) {
      Message.warning('默认标签不能删除');
      return;
    }
    event.stopPropagation();
    deleteImageTagAsync(imageId, tagId).then(success => {
      if (success) {
        const tagEl = event.target.closest(`[data-tag-id="${tagId}"]`);
        const image = images.value.find(item => item.id === imageId);
        if (image) {
          image.tags = image.tags.filter(item => item.id !== tagId);
          if (image.tags.length === 0) {
            image.tags.push({ id: 0, name: '默认' });
            if (tagEl){
              tagEl.innerHTML = `
                <span>默认</span>
                <button data-preview-action="deleteImageTag" data-preview-args="${escapeHtml(JSON.stringify(['event', String(imageId), '0']))}" class="ml-1 text-primary/70 hover:text-primary/30">
                  <i class="ri-close-line text-xs"></i>
                </button>
              `;
              tagEl.setAttribute('data-tag-id', '0');
            }
          } else {
            if (tagEl) tagEl.remove();
          }
          currentPreviewImage.value = image;
        }
      }
    });
  };

  actions.closePreviewModal = () => {
    if (modal) {
      modal.close();
      cleanupPreview();
    }
  };

  actions.addImageTag = (imageId) => {
    actions.closePreviewModal();
    const tagList = [{ value: "0", label: "请选择Tag", disabled: true }];
    presetTags.value.forEach(tag => {
      tagList.push({ value: tag.id, label: tag.name });
    });
    const modal = new showFormModal({
      title: '添加Tag',
      formFields: [
        {
          name: 'tag',
          label: 'Tag标签',
          type: 'select',
          required: true,
          defaultValue: "0",
          options: tagList
        },
      ],
      buttons: [
        {
          text: '取消',
          type: 'default',
          callback: (m) => {
            m.close();
            const image = images.value.find(item => item.id === imageId);
            openPreview(image);
          }
        },
        {
          text: '添加',
          type: 'primary',
          callback: (m) => {
            const formData = serializeForm(m);
            pustImageTag(imageId, formData);
            m.close();
          }
        }
      ]
    });
    modal.open();
  };
  bindImagePreview(modal.content, actions, { image: images.value.find(item => item.id === imageId) });
};

const cleanPreviewGlobalFunctions = () => {};

return { openPreview, cleanupPreview: () => { dispose(); previewModal = null; } };
}
