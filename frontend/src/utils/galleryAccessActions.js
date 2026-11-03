import { readApiResponse } from './apiFeedback.js';
import { createDialogScope } from './dialogScope.js';
import Message from './message.js';
import { getSelectedAccessBucketId, getAccessSourceOptionLabel } from './imageAccessSources.js';

export function createGalleryAccessActions({ API_BASE_URL, images, selectedImages, currentPreviewImage, getAccessSourceOptions, setAccessSourceUpdating, serializeForm }) {
const { Dialog: PopupModal, dispose: disposeDialogs } = createDialogScope();
const showFormModal = function(options) { return new PopupModal({ type: 'form', ...options }); };

// 单张与批量共用同一条写入路径，避免两份请求体/错误处理各写一遍
const putImageAccessSource = async (imageIds, bucketId) => {
  const single = imageIds.length === 1;
  const response = await fetch(`${API_BASE_URL}${single ? `/api/images/${Number(imageIds[0])}/access-source` : '/api/images/access-source'}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' },
    body: JSON.stringify(single ? { bucket_id: Number(bucketId) } : { image_ids: imageIds, bucket_id: Number(bucketId) }),
  });
  const result = await readApiResponse(response, '设置访问源失败');
  if (!response.ok || result.code !== 200) throw new Error(result.message || '设置访问源失败');
  return result;
};

/**
 * 单张图片切换访问源。
 *
 * 之前这里会再弹一个「访问链接读取源」的小弹窗，现在详情本身已经是弹窗了，
 * 再叠一层属于弹窗套弹窗；改为由详情弹窗里的 select 直接调用本函数。
 * @returns {Promise<boolean>} 是否成功（失败时已回滚界面状态并提示）
 */
const applyImageAccessSource = async (image, bucketId) => {
  const previousBucketId = getSelectedAccessBucketId(image);
  const target = Number(bucketId);
  if (!target || target === previousBucketId) return true;
  // 目标源必须在可选列表里，且不是已停用/不可用项
  const source = getAccessSourceOptions(image).find(item => Number(item.bucket_id) === target);
  if (!source || source.bucket_disabled || source.access_unavailable) {
    Message.warning('该存储源当前不可用');
    return false;
  }
  setAccessSourceUpdating(image.id, true);
  try {
    const result = await putImageAccessSource([image.id], target);
    image.access_bucket_id = target;
    if (currentPreviewImage.value?.id === image.id) currentPreviewImage.value.access_bucket_id = target;
    Message.success(result.message || '图片访问源已更新');
    return true;
  } catch (error) {
    console.error('设置图片访问源失败:', error);
    Message.error(error.message || '设置图片访问源失败');
    return false;
  } finally {
    setAccessSourceUpdating(image.id, false);
  }
};

const getBatchAccessSourceOptions = () => {
  const selected = images.value.filter(image => selectedImages.value.includes(image.id));
  if (selected.length === 0) return [];
  const candidates = getAccessSourceOptions(selected[0]).filter(
    source => !source.bucket_disabled && !source.access_unavailable
  );
  return candidates.filter(candidate => selected.every(image =>
    getAccessSourceOptions(image).some(source =>
      Number(source.bucket_id) === Number(candidate.bucket_id) &&
      !source.bucket_disabled &&
      !source.access_unavailable
    )
  ));
};

const updateBatchAccessSource = async (bucketId) => {
  try {
    const result = await putImageAccessSource(selectedImages.value, bucketId);
    images.value.forEach(image => {
      if (selectedImages.value.includes(image.id)) image.access_bucket_id = Number(bucketId);
    });
    Message.success(result.message || '批量访问源已更新');
    return true;
  } catch (error) {
    console.error('批量设置访问源失败:', error);
    Message.error(error.message || '批量设置访问源失败');
    return false;
  }
};

const handleBatchSetAccessSource = () => {
  if (selectedImages.value.length === 0) {
    Message.warning('请选择要编辑的图片');
    return;
  }
  const sources = getBatchAccessSourceOptions();
  if (sources.length === 0) {
    Message.warning('所选图片没有共同的、已同步成功的可用存储源');
    return;
  }
  const localSource = sources.find(source => source.bucket_type === 'default');
  const defaultBucketId = localSource?.bucket_id || sources[0].bucket_id;
  const modal = new showFormModal({
    title: '批量设置访问源',
    formFields: [
      {
        name: 'bucket_id',
        label: '访问链接读取源',
        type: 'select',
        required: true,
        defaultValue: String(defaultBucketId),
        options: sources.map(source => ({
          value: String(source.bucket_id),
          label: getAccessSourceOptionLabel(source),
        })),
        tip: `仅显示这 ${selectedImages.value.length} 张图片都已同步成功的存储源`,
      },
    ],
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: modalInstance => modalInstance.close(),
      },
      {
        text: '确认设置',
        type: 'primary',
        callback: async modalInstance => {
          const formData = serializeForm(modalInstance);
          if (await updateBatchAccessSource(formData.bucket_id)) modalInstance.close();
        },
      },
    ],
  });
  modal.open();
};

return { disposeDialogs, applyImageAccessSource, handleBatchSetAccessSource };
}
