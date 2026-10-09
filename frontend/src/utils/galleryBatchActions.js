import { readApiResponse } from './apiFeedback.js';
import { createDialogScope } from './dialogScope.js';
import { copyToClipboard } from './clipboard.js';
import Message from './message.js';
import { escapeHtml } from './html.js';

export function createGalleryBatchActions({ selectedImages, images, presetTags, loadImages, getFullUrl, API_BASE_URL, showFormModal, serializeForm }) {
const { Dialog: PopupModal, dispose: disposeDialogs } = createDialogScope();
const handleBatchSetTag = () => {
  if (selectedImages.value.length === 0) {
    Message.warning('请选择要编辑的图片');
    return;
  }
  const imageId = selectedImages.value;
  const tagList = [
    { value: "0", label: "请选择Tag", disabled: true }
  ];
  presetTags.value.forEach(tag => {
    tagList.push({ value: tag.id, label: tag.name });
  });
  const modal = new showFormModal({
    title: '批量编辑Tag',
    formFields: [
      {
        name: 'tag',
        label: 'Tag标签',
        type: 'select',
        required: true,
        defaultValue: "0",
        options: tagList,
        tip: "已选择的图片：\n" + images.value.filter(item => imageId.includes(item.id)).map(item => item.filename).join("\n")
      },
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
        text: '删除Tag',
        type: 'danger',
        callback: (modal) => {
          const formData = serializeForm(modal);
          batchDeleteTag(formData);
          modal.close();
        }
      },
      {
        text: '添加Tag',
        type: 'primary',
        callback: (modal) => {
          const formData = serializeForm(modal);
          batchAddTag(formData);
          modal.close();
        }
      }
    ]
  });
  modal.open();
}

const batchDeleteTag = async (formData) => {
  if (formData.tag === "0") {
    Message.warning("请选择Tag");
    return;
  }
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/tags`, {
      method: 'DELETE',
      headers: {
        'X-Requested-With': 'XMLHttpRequest'
      },
      body: JSON.stringify({
        image_ids: selectedImages.value,
        tag_id: formData.tag
      })
    });
    const result = await readApiResponse(response, '请求失败');
    if (response.ok && result.code === 200) {
      Message.success('删除Tag成功');
      await loadImages();
    } else {
      throw new Error(result.message || '删除Tag失败');
    }
  } catch (error) {
    console.error('删除Tag失败:', error);
    Message.error(error.message || '删除Tag失败');
  }
}

const batchAddTag = async (formData) => {
  if (formData.tag === "0") {
    Message.warning("请选择Tag");
    return;
  }
  try {
    const response = await fetch(`${API_BASE_URL}/api/images/tags`, {
      method: 'POST',
      headers: {
        'X-Requested-With': 'XMLHttpRequest'
      },
      body: JSON.stringify({
        image_ids: selectedImages.value,
        tag_id: formData.tag
      })
    });
    const result = await readApiResponse(response, '请求失败');
    if (response.ok && result.code === 200) {
      Message.success('添加Tag成功');
      await loadImages();
    } else {
      throw new Error(result.message || '添加Tag失败');
    }
  } catch (error) {
    console.error('添加Tag失败:', error);
    Message.error(error.message || '添加Tag失败');
  }
}

const handleBatchCopy = () => {
  if (selectedImages.value.length === 0) {
    Message.warning('请选择要复制的图片');
    return;
  }
  const selectedImageList = images.value.filter(img => selectedImages.value.includes(img.id));
  if (selectedImageList.length === 0) {
    Message.warning('未找到选中的图片');
    return;
  }

  const generateText = (format) => {
    return selectedImageList.map(img => {
      const url = getFullUrl(img.url);
      switch (format) {
        case 'url': return url;
        case 'markdown': return `![${escapeHtml(img.filename)}](${url})`;
        case 'html': return `<img src="${url}" alt="${escapeHtml(img.filename)}">`;
        case 'bbcode': return `[img]${url}[/img]`;
        default: return url;
      }
    }).join('\n');
  };

  const formatLabels = {
    url: 'URL 链接',
    markdown: 'Markdown',
    html: 'HTML',
    bbcode: 'BBCode'
  };



  const modal = new PopupModal({
    title: `批量复制（${selectedImageList.length} 张图片）`,
    content: `
      <div class="space-y-3">
        <p class="text-sm text-secondary">选择要复制的格式：</p>
        <div class="space-y-2" id="batchCopyFormatList">
          <label class="flex items-center gap-3 p-3 rounded-lg border border-primary bg-primary/5 dark:bg-primary/10 cursor-pointer transition-colors batch-copy-option" data-format="url">
            <input type="radio" name="batchCopyFormat" value="url" checked class="h-4 w-4 text-primary shrink-0">
            <div class="min-w-0">
              <div class="text-sm font-medium">URL 链接</div>
              <div class="text-xs text-secondary mt-0.5">每行一个图片直链地址</div>
            </div>
          </label>
          <label class="flex items-center gap-3 p-3 rounded-lg border border-slate-200 dark:border-white/10 cursor-pointer hover:bg-slate-50 dark:hover:bg-white/5 transition-colors batch-copy-option" data-format="markdown">
            <input type="radio" name="batchCopyFormat" value="markdown" class="h-4 w-4 text-primary shrink-0">
            <div class="min-w-0">
              <div class="text-sm font-medium">Markdown</div>
              <div class="text-xs text-secondary mt-0.5">![filename](url) 格式，适用于 Markdown 编辑器</div>
            </div>
          </label>
          <label class="flex items-center gap-3 p-3 rounded-lg border border-slate-200 dark:border-white/10 cursor-pointer hover:bg-slate-50 dark:hover:bg-white/5 transition-colors batch-copy-option" data-format="html">
            <input type="radio" name="batchCopyFormat" value="html" class="h-4 w-4 text-primary shrink-0">
            <div class="min-w-0">
              <div class="text-sm font-medium">HTML</div>
              <div class="text-xs text-secondary mt-0.5">&lt;img src="url" alt="filename"&gt; 格式</div>
            </div>
          </label>
          <label class="flex items-center gap-3 p-3 rounded-lg border border-slate-200 dark:border-white/10 cursor-pointer hover:bg-slate-50 dark:hover:bg-white/5 transition-colors batch-copy-option" data-format="bbcode">
            <input type="radio" name="batchCopyFormat" value="bbcode" class="h-4 w-4 text-primary shrink-0">
            <div class="min-w-0">
              <div class="text-sm font-medium">BBCode</div>
              <div class="text-xs text-secondary mt-0.5">[img]url[/img] 格式，适用于论坛</div>
            </div>
          </label>
        </div>
        <div class="mt-3 p-3 rounded-lg bg-slate-50 dark:bg-white/5 border border-slate-200/50 dark:border-white/5">
          <label class="flex items-center gap-2 cursor-pointer mb-2">
            <input type="checkbox" id="batchCopyPreview" class="h-4 w-4 rounded border-gray-300 text-primary focus:ring-primary">
            <span class="text-sm font-medium">预览内容</span>
          </label>
          <pre id="batchCopyPreviewContent" class="hidden text-xs text-secondary overflow-auto max-h-40 whitespace-pre-wrap break-all bg-white dark:bg-slate-900 rounded-lg p-3 border border-slate-200 dark:border-white/10 font-mono"></pre>
        </div>
      </div>
    `,
    buttons: [
      {
        text: '取消',
        type: 'default',
        callback: (m) => {
          m.close();

        }
      },
      {
        text: '复制到剪贴板',
        type: 'primary',
        callback: (m) => {
          const format = m.content?.querySelector('input[name="batchCopyFormat"]:checked')?.value || 'url';
          const genFn = generateText;
          if (typeof genFn !== 'function') {
            Message.error('复制功能异常，请重试');
            return;
          }
          const text = genFn(format);
          copyToClipboard(text).then(ok => {
            if (ok) {
              Message.success(`已复制 ${selectedImageList.length} 张图片的${formatLabels[format]}格式`);
              m.close();

            } else {
              Message.error('复制失败，请手动复制');
            }
          });
        }
      }
    ],
    maskClose: true
  });
  modal.open();

  requestAnimationFrame(() => {
    const container = modal.content;
    if (!container) return;

    container.querySelectorAll('input[name="batchCopyFormat"]').forEach(radio => {
      radio.addEventListener('change', () => {
        const selected = container.querySelector('input[name="batchCopyFormat"]:checked')?.value || 'url';
        container.querySelectorAll('.batch-copy-option').forEach(el => {
          if (el.dataset.format === selected) {
            el.classList.add('border-primary', 'bg-primary/5', 'dark:bg-primary/10');
            el.classList.remove('border-slate-200', 'dark:border-white/10');
          } else {
            el.classList.remove('border-primary', 'bg-primary/5', 'dark:bg-primary/10');
            el.classList.add('border-slate-200', 'dark:border-white/10');
          }
        });
        const checkbox = container.querySelector('#batchCopyPreview');
        const preview = container.querySelector('#batchCopyPreviewContent');
        if (checkbox?.checked && preview && typeof generateText === 'function') {
          preview.textContent = generateText(selected);
        }
      });
    });

    const checkbox = container.querySelector('#batchCopyPreview');
    const preview = container.querySelector('#batchCopyPreviewContent');
    checkbox?.addEventListener('change', () => {
      if (checkbox.checked) {
        const format = container.querySelector('input[name="batchCopyFormat"]:checked')?.value || 'url';
        if (typeof generateText === 'function') {
          preview.textContent = generateText(format);
        }
        preview.classList.remove('hidden');
      } else {
        preview.classList.add('hidden');
      }
    });
  });
};


return { disposeDialogs, handleBatchSetTag, handleBatchCopy };
}
