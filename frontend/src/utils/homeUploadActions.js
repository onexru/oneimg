import Message from './message.js';
import { fileValidationMessage } from './uploadValidation.js';
import { uploadOutcome } from './batchResults.js';
import { boundedPage, RECENT_IMAGE_LIMIT } from './renderBounds.js';
import { xhrUpload, mergeImages, safeMessage } from './directUpload.js';
export function createHomeUploadActions({ API_BASE_URL, isDragOver, isUploading, fileInput, resumeTask, handleResumeFile, clientUploadLimits, uploadConfig, storageConfigLoaded, directEnabled, startDirectFiles, legacyUploading, uploadingCount, uploadProgress, getUploadFolderId = () => 0, multiStorageSync, selectedBucket, recentImages, loadRecentImages, isDisposed, onSingleUpload = () => {} }) {
let legacyController;
const authHeaders = () => ({});
const handleDragOver = () => {
  isDragOver.value = true;
};

const handleDragEnter = () => {
  isDragOver.value = true;
};

const handleDragLeave = (e) => {
  if (!e.currentTarget.contains(e.relatedTarget)) {
    isDragOver.value = false;
  }
};

const handleDrop = (e) => {
  e.preventDefault();
  isDragOver.value = false;

  const files = Array.from(e.dataTransfer.files);
  const validFiles = validateFiles(files);

  if (validFiles.length > 0) {
    uploadFiles(validFiles);
  } else {
    Message.error('请拖拽有效的 JPG、PNG、GIF 或 WebP 图片；SVG 不允许上传', {
      duration: 3000,
      position: 'top-right'
    });
  }
};

/**
 * 文件选择处理
 */
const triggerFileInput = () => {
  if (!isUploading.value && fileInput.value) {
    resumeTask.value = null;
    fileInput.value.click();
  }
};

const handleFileSelect = (e) => {
  if (resumeTask.value) return handleResumeFile(e);
  const files = Array.from(e.target.files);
  if (files.length > 0) {
    const validFiles = validateFiles(files);
    if (validFiles.length > 0) {
      uploadFiles(validFiles);
    }
  }
  e.target.value = ''; // 清空文件选择
};

/**
 * 剪贴板粘贴处理
 */
const handlePaste = async (e) => {
  const items = e.clipboardData?.items;
  if (!items) return;

  const imageFiles = [];

  for (let item of items) {
    if (item.type.startsWith('image/')) {
      const file = item.getAsFile();
      if (file) {
        const timestamp = new Date().getTime();
        const extension = item.type.split('/')[1] || 'png';
        const newFile = new File([file], `paste-${timestamp}.${extension}`, {
          type: item.type
        });
        imageFiles.push(newFile);
      }
    }
  }

  if (imageFiles.length > 0) {
    e.preventDefault();
    const valid = validateFiles(imageFiles);
    if (!valid.length) return;
    uploadFiles(valid);
    Message.success(`从剪贴板粘贴了 ${valid.length} 个图片`, {
      duration: 2000,
      position: 'top-right'
    });
  }
};

/**
 * 验证文件有效性
 */
const validateFiles = (files) => {
  if (files.length > clientUploadLimits.value.max_upload_files) {
    Message.warning(`每批最多上传 ${clientUploadLimits.value.max_upload_files} 个文件，请分批选择`);
    return [];
  }
  return files.filter(file => {
    const error = fileValidationMessage(file, uploadConfig.value);
    if (error) Message.warning(error, { duration: 4000, position: 'top-right' });
    return !error;
  });
};

const uploadFiles = async (files) => {
  if (isUploading.value) return;
  if (!storageConfigLoaded.value) {
    Message.warning('存储配置正在加载，请稍后重试');
    return;
  }

  files = validateFiles(files);
  if (!files.length) return;
  if (directEnabled()) { await startDirectFiles(files); return; }
  legacyUploading.value = true;
  legacyController = new AbortController();
  uploadingCount.value = files.length;
  uploadProgress.value = 0;

  try {
    const formData = new FormData();
    files.forEach(file => {
      formData.append('images[]', file);
    });

    // Snapshot before any transfer; never send removed upload-tag state.
    formData.append('folder_id', String(getUploadFolderId()));
    if (!multiStorageSync.value) {
      formData.append('bucket_id', selectedBucket.value || '1');
    }
    const { result } = await xhrUpload({ url: `${API_BASE_URL}/api/upload/images`, body: formData, returnEnvelope: true,
      headers: authHeaders(), signal: legacyController.signal,
      onProgress: ({ percent }) => { uploadProgress.value = percent; } });
    if (isDisposed()) return;
    const outcome = uploadOutcome(result, files.length);
    recentImages.value = boundedPage(mergeImages(recentImages.value, outcome.images), RECENT_IMAGE_LIMIT);
    await loadRecentImages();
    if (outcome.failed) {
      const detail = outcome.errors.map(item => `${item.filename || item.file || '文件'}：${item.message || item.error || '上传失败'}`).join('；');
      Message.warning(`成功 ${outcome.images.length} 个，失败 ${outcome.failed} 个。成功文件已保留，请勿整批重传。${detail || result.message || ''}`, { duration: 7000, showClose: true });
    } else {
      Message.success(multiStorageSync.value ? '已保存到本机，正在后台同步' : `上传成功 ${outcome.images.length} 个`, { duration: 2000, position: 'top-right' });
    }
    if (!isDisposed() && files.length === 1 && !outcome.failed && outcome.images.length === 1) {
      onSingleUpload(outcome.images[0]);
    }
  } catch (error) {
    if (!isDisposed()) {
      if (error.name === 'AbortError') Message.info('已停止上传请求；已接收的文件仍可能完成，请检查最近上传');
      else Message.error(`上传失败: ${safeMessage(error.message)}`, { duration: 3000, position: 'top-right', showClose: true });
      await loadRecentImages();
    }
  } finally {
    legacyUploading.value = false;
    legacyController = null;
    uploadingCount.value = 0;
    uploadProgress.value = 0;
  }
};

/**
 * 标签相关处理
 */
return { handleDragOver, handleDragEnter, handleDragLeave, handleDrop, triggerFileInput, handleFileSelect, handlePaste, validateFiles, uploadFiles, cancelLegacyUpload: () => legacyController?.abort() };
}
