import { createDialogScope } from './dialogScope.js';
import Message from './message.js';
import { renderImagePreview, bindImagePreview } from './imagePreview.js';
import { readApiResponse } from './apiFeedback.js';
import { folderAccount, folderLabel } from './folders.js';
export function createHomeDialogs({ storageConfigLoaded, getUploadFolderId = () => 0, folders = { value: [] }, foldersAuthenticated = { value: false }, presetBuckets, selectedBucket, multiStorageSync, getFullUrl, errorImg, formatFileSize, formatDate, copyImageLink, downloadImage, deleteImage, loadRecentImages, onSingleUpload = () => {}, isDisposed = () => false }) {
const { Dialog: PopupModal, dispose: disposeDialogs } = createDialogScope();
let currentPreviewImage = null;
let previewModalInstance = null;
const previewImage = (image, { title = '图片预览', showLink = false } = {}) => {
  previewModalInstance?.close();
  const actions = {};
  if (!image || !image.url) {
    Message.error('图片信息不完整，无法预览', {
      duration: 2000,
      position: 'top-right'
    });
    return;
  }

  currentPreviewImage = image;

  const previewContent = renderImagePreview(image, {
    multiStorageSync: multiStorageSync.value, buckets: presetBuckets.value,
    imageUrl: getFullUrl(image.url), errorImg, formatFileSize, formatDate,
    editableTags: false, headerTitle: true, showLink,
  });

  // 注册预览相关全局函数
  actions.copyPreviewImageLink = (type) => copyImageLink(currentPreviewImage, type);
  actions.downloadPreviewImage = () => downloadImage(currentPreviewImage);
  actions.deletePreviewImage = () => {
    deleteImage(currentPreviewImage.id);
    actions.closePreviewModal();
  };
  actions.closePreviewModal = () => {
    if (previewModalInstance) {
      previewModalInstance.close();
      cleanupPreview();
    }
  };

  // 创建预览弹窗
  previewModalInstance = new PopupModal({
    title,
    content: previewContent,
    type: 'default',
    buttons: [{
      text: '确定',
      type: 'default',
      callback: (modal) => modal.close()
    }],
    maskClose: true,
    zIndex: 10000,
    maxHeight: '90vh',
    onClose: cleanupPreview
  });

  bindImagePreview(previewModalInstance.content, actions, { image, editableTags: false });
  previewModalInstance.open();


};

const uploadbyurlmodal = () => {
  if (!storageConfigLoaded.value) {
    Message.warning('存储配置正在加载，请稍后重试');
    return;
  }
  const folderList = [{ value: '0', label: '未分类' }, ...folders.value.map(folder => ({ value: String(folder.id), label: folderLabel(folder), disabled: !!folder.deleting }))];
  const uploadFolderId = getUploadFolderId();
  const uploadAccount = folderAccount();
  const storageList = presetBuckets.value.map(storage => ({
    value: storage.id,
    label: storage.name,
  }));
  const modal = new PopupModal({
    title: '从URL上传图片',
    type: 'form',
    formFields: [
      {
        name: 'url',
        label: '图片链接',
        type: 'text',
        required: true,
        placeholder: '请输入图片链接'
      },
      ...(foldersAuthenticated.value ? [{
        name: 'folder_id', label: '上传文件夹', type: 'select', required: true,
        defaultValue: String(uploadFolderId), options: folderList,
      }] : []),
      ...(!multiStorageSync.value ? [{
        name: 'bucket_id',
        label: '存储',
        type: 'select',
        required: true,
        defaultValue: selectedBucket.value || "1",
        options: storageList,
      }] : [])
    ],
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (modal) => {
          modal.close();
        }
      },
      {
        text: '确定',
        type: 'primary',
        callback: (modal) => {
          if (folderAccount() !== uploadAccount) { Message.error('账户已变更，请重新打开上传窗口'); modal.close(); return; }
          const formData = serializeForm(modal);
          formData.folder_id = Number(formData.folder_id ?? uploadFolderId);
          if (formData.folder_id !== 0 && !folders.value.some(folder => folder.id === formData.folder_id && !folder.deleting)) { Message.error('文件夹不可用或正在删除，请重新选择'); return; }
          if(!formData['url']?.trim()) {
            Message.error('请输入图片链接');
            return
          }
          postuploadbyurl(formData);
          modal.close();
        }
      }
    ]
  });
  modal.open();
}

const postuploadbyurl = async (formData) => {
  try {
    const res = await fetch(`/api/images/url`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest'
      },
      body: JSON.stringify(formData)
    });
    const result = await readApiResponse(res, '上传失败');
    if (res.ok && result.code === 200) {
      await loadRecentImages();
      if (isDisposed()) return;
      Message.success(multiStorageSync.value ? '已保存到本机，正在后台同步' : '上传成功');
      onSingleUpload(result.data?.file);
    } else {
      throw new Error(result.message || '上传失败');
    }
  } catch (err) {
    console.error(err);
    Message.error(err.message || '上传失败');
  }
}

/**
 * 序列化表单数据
 * @param {Object} modal - 弹窗实例
 * @returns {Object} 表单数据对象
 */
const serializeForm = (modal) => {
  const form = modal.content?.querySelector('form');
  if (!form) {
    console.warn('未找到表单元素');
    return {};
  }

  return Array.from(form.elements).reduce((acc, element) => {
    const { name, disabled, type, checked, value } = element;

    // 跳过无name、禁用的元素
    if (!name || disabled) return acc;

    // 处理复选框/单选框
    if ((type === 'checkbox' || type === 'radio') && !checked) return acc;

    // 处理文件输入
    if (type === 'file') {
      acc[name] = element.files.length > 0 ? element.files[0].name : '';
      return acc;
    }

    // 处理多值字段
    if (acc[name]) {
      acc[name] = Array.isArray(acc[name]) ? [...acc[name], value] : [acc[name], value];
    } else {
      acc[name] = value;
    }

    return acc;
  }, {});
};

/**
 * 清理预览相关资源
 */
const cleanupPreview = () => { currentPreviewImage = null; previewModalInstance = null; };

return { disposeDialogs, previewImage, uploadbyurlmodal, cleanupPreview: () => { previewModalInstance?.close(); }, closeDeletedPreview: id => { if (currentPreviewImage?.id === id) previewModalInstance?.close(); } };
}
