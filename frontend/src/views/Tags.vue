<template>
  <div class="page-shell">
    <!-- 全站规范：页面标题清零，page-header 只放右侧统计/动作 -->
    <section class="page-header">
      <div class="page-actions">
        <span class="stat-inline">标签总数 <strong>{{ totalTagCount }}</strong></span>
      </div>
    </section>

    <!-- 新建标签：输入框 + 按钮同一行，窄屏自动堆叠 -->
    <div class="surface flex flex-col gap-3 sm:flex-row sm:items-center">
      <div class="relative min-w-0 flex-1">
        <i class="ri-price-tag-3-line pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 text-sm text-slate-400"></i>
        <input
          id="tag-name-input"
          v-model="tagInput"
          type="text"
          :maxlength="TAG_MAX_LENGTH"
          class="input-modern pl-9"
          placeholder="输入标签名称，回车即可添加"
          @keyup.enter="handleAddTag"
        />
      </div>
      <div class="flex shrink-0 items-center gap-2">
        <!-- 字数提示：接近上限时才变色，平时保持安静 -->
        <span
          class="text-xs tabular-nums"
          :class="tagInput.length >= TAG_MAX_LENGTH ? 'text-amber-600 dark:text-amber-400' : 'text-slate-400 dark:text-slate-500'"
        >{{ tagInput.length }}/{{ TAG_MAX_LENGTH }}</span>
        <button
          id="tag-add-button"
          type="button"
          class="primary-button shrink-0"
          :disabled="isAdding || !tagInput.trim()"
          @click="handleAddTag"
        >
          <i :class="isAdding ? 'ri-loader-4-line animate-spin' : 'ri-add-line'" aria-hidden="true"></i>
          <span>{{ isAdding ? '提交中' : '添加' }}</span>
        </button>
      </div>
    </div>

    <!-- 校验错误提示：与网络错误分开，本地校验不弹 Toast（太吵） -->
    <p
      v-if="errorMsg"
      class="flex items-start gap-2 rounded-[var(--radius-sm)] border border-red-200 bg-red-50 px-3 py-2.5 text-sm text-red-700 dark:border-red-400/20 dark:bg-red-400/10 dark:text-red-300"
      role="alert"
    >
      <i class="ri-error-warning-line mt-0.5 shrink-0" aria-hidden="true"></i>
      <span>{{ errorMsg }}</span>
    </p>

    <!-- 标签列表 -->
    <div class="surface">
      <div class="mb-4 flex items-center justify-between gap-2">
        <h2 class="surface-title">
          <i class="ri-bookmark-3-line text-primary" aria-hidden="true"></i>
          已创建标签
          <span class="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-normal text-slate-500 dark:bg-slate-800 dark:text-slate-400">{{ totalTagCount }}</span>
        </h2>
        <p v-if="tagList.length" class="hidden text-xs text-slate-400 dark:text-slate-500 sm:block">点击标签可重命名</p>
      </div>

      <!-- 「默认」标签是系统内置的，任何情况下都存在：
           它不依赖 tagList，所以不能和自定义标签一起被空态分支挡掉。 -->
      <div class="flex flex-wrap gap-2">
        <span
          class="inline-flex items-center gap-1.5 rounded-full border border-slate-200 bg-slate-50 py-1.5 pl-3 pr-3 text-sm text-slate-500 dark:border-white/10 dark:bg-slate-950 dark:text-slate-400"
          title="默认标签不可修改或删除"
        >
          <i class="ri-lock-line text-xs" aria-hidden="true"></i>
          默认
        </span>

        <button
          v-for="tag in tagList"
          :key="tag.id"
          type="button"
          class="tag-chip"
          :title="`点击重命名「${tag.name}」`"
          @click="handleUpdateTag(tag)"
        >
          <span class="min-w-0 truncate">{{ tag.name }}</span>
          <span
            role="button"
            tabindex="0"
            class="tag-chip-remove"
            :aria-label="`删除标签 ${tag.name}`"
            @click.stop="handleDeleteTag(tag)"
            @keydown.enter.stop.prevent="handleDeleteTag(tag)"
            @keydown.space.stop.prevent="handleDeleteTag(tag)"
          >
            <i class="ri-close-line" aria-hidden="true"></i>
          </span>
        </button>
      </div>

      <!-- 空态：没有自定义标签时给出引导，而不是留一片空白 -->
      <p v-if="!tagList.length" class="mt-3 rounded-[var(--radius-sm)] border border-dashed border-slate-200 px-4 py-6 text-center text-sm text-slate-500 dark:border-white/10 dark:text-slate-400">
        还没有自定义标签，在上方输入名称即可创建。
      </p>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue';
import Message from '@/utils/message.js';
import { createDialogScope } from '@/utils/dialogScope.js';

const { Dialog: PopupModal, dispose: disposeDialogs } = createDialogScope();
const showFormModal = options => new PopupModal({ type: 'form', ...options });

// 标签长度上限与后端 setting 的 tag_max_length 默认值一致
const TAG_MAX_LENGTH = 10;
// 「默认」是模板里硬编码的虚拟标签，不在 tagList 里，
// 所以总数要单独加上它，避免列表里看得见、数字里数不到。
const VIRTUAL_TAG_COUNT = 1;

const tagInput = ref('');
const tagList = ref([]);
const totalTagCount = computed(() => tagList.value.length + VIRTUAL_TAG_COUNT);
const errorMsg = ref('');
const isAdding = ref(false);
const isDeleting = ref(false);

const readApiResponse = async (response, fallback) => {
  try {
    return await response.json();
  } catch {
    return { code: response.status, message: fallback };
  }
};

const fetchTagList = async () => {
  try {
    const response = await fetch('/api/tags', {
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
    });
    const result = await readApiResponse(response, '获取标签列表失败');
    if (!response.ok || result.code !== 200) {
      throw new Error(result.message || '获取标签列表失败');
    }
    tagList.value = result.data?.list || [];
  } catch (error) {
    console.error('获取标签失败:', error);
    Message.error(error.message || '获取标签列表失败');
  }
};

const handleAddTag = async () => {
  errorMsg.value = '';
  const tagName = tagInput.value.trim();

  if (!tagName) {
    errorMsg.value = '标签名称不能为空';
    return;
  }
  if (tagName.length > TAG_MAX_LENGTH) {
    errorMsg.value = `标签名称不能超过 ${TAG_MAX_LENGTH} 个字符`;
    return;
  }
  // 本地查重只为提前反馈；后端仍有唯一约束做最终兜底
  if (tagList.value.some(tag => tag.name === tagName)) {
    errorMsg.value = '该标签已存在，请勿重复添加';
    return;
  }

  isAdding.value = true;
  try {
    const response = await fetch('/api/tags', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest',
      },
      body: JSON.stringify({ name: tagName }),
    });
    const result = await readApiResponse(response, '添加标签失败');
    if (!response.ok || result.code !== 200) {
      throw new Error(result.message || '添加标签失败');
    }
    // 后端返回的才是权威记录（含真实 id），不能自己拼一个对象塞进列表
    if (result.data?.id != null) tagList.value.push(result.data);
    else await fetchTagList();
    tagInput.value = '';
    Message.success('标签添加成功');
  } catch (error) {
    console.error('添加标签失败:', error);
    errorMsg.value = error.message || '添加标签失败';
  } finally {
    isAdding.value = false;
  }
};

const handleDeleteTag = (tag) => {
  // 删除确认没有表单字段，必须用普通模态：PopupModal 在 type==='form' 时
  // 会走 renderFormContent() 并忽略 content，写了也渲染不出来。
  const modal = new PopupModal({
    title: '删除标签',
    content: `
      <div class="flex gap-3">
          <i class="fa fa-exclamation-triangle text-warning text-xl mt-1"></i>
          <div>
          <p>确定要删除这个标签吗？</p>
          <p class="mt-1 text-secondary text-sm">删除后无法恢复，请谨慎操作</p>
          </div>
      </div>
      `,
    buttons: [
      { text: '取消', type: 'default', callback: instance => instance.close() },
      {
        text: '删除',
        type: 'danger',
        callback: async instance => {
          const ok = await deleteTag(tag);
          // 失败时保留弹窗，让用户能直接重试
          if (ok) instance.close();
        },
      },
    ],
  });
  modal.open();
};

const deleteTag = async (tag) => {
  if (isDeleting.value) return false;
  isDeleting.value = true;
  try {
    const response = await fetch(`/api/tags/${tag.id}`, {
      method: 'DELETE',
      headers: { 'X-Requested-With': 'XMLHttpRequest' },
    });
    const result = await readApiResponse(response, '删除标签失败');
    if (!response.ok || result.code !== 200) {
      throw new Error(result.message || '删除标签失败');
    }
    // 按 id 过滤而不是按位置：列表在请求期间可能已经变了
    tagList.value = tagList.value.filter(item => item.id !== tag.id);
    Message.success('标签删除成功');
    return true;
  } catch (error) {
    console.error('删除标签失败:', error);
    Message.error(error.message || '删除标签失败');
    return false;
  } finally {
    isDeleting.value = false;
  }
};

const handleUpdateTag = (tag) => {
  const modal = showFormModal({
    title: '重命名标签',
    subtitle: `修改「${tag.name}」的名称。`,
    formFields: [{
      name: 'name',
      label: '标签名称',
      type: 'text',
      required: true,
      maxlength: TAG_MAX_LENGTH,
      defaultValue: tag.name,
    }],
    buttons: [
      { text: '取消', type: 'default', callback: instance => instance.close() },
      {
        text: '保存',
        type: 'primary',
        callback: instance => {
          instance.content.querySelector('form')?.requestSubmit();
        },
      },
    ],
    formSubmit: async (instance, formData) => {
      const name = String(formData.name ?? '').trim();
      if (!name) {
        Message.warning('标签名称不能为空');
        return;
      }
      if (name.length > TAG_MAX_LENGTH) {
        Message.warning(`标签名称不能超过 ${TAG_MAX_LENGTH} 个字符`);
        return;
      }
      if (name === tag.name) {
        instance.close();
        return;
      }
      if (tagList.value.some(item => item.id !== tag.id && item.name === name)) {
        Message.warning('该标签已存在');
        return;
      }
      const ok = await updateTag(tag, name);
      if (ok) instance.close();
    },
  });
  modal.open();
};

const updateTag = async (tag, name) => {
  try {
    const response = await fetch(`/api/tags/${tag.id}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'X-Requested-With': 'XMLHttpRequest',
      },
      body: JSON.stringify({ name }),
    });
    const result = await readApiResponse(response, '修改标签失败');
    if (!response.ok || result.code !== 200) {
      throw new Error(result.message || '修改标签失败');
    }
    tag.name = name;
    Message.success('标签修改成功');
    return true;
  } catch (error) {
    console.error('修改标签失败:', error);
    Message.error(error.message || '修改标签失败');
    return false;
  }
};

onMounted(() => { fetchTagList(); });
// 弹窗由本页创建，离开时必须销毁，否则会残留遮罩与滚动锁
onBeforeUnmount(() => { disposeDialogs(); });
</script>

<style scoped>
/* 标签胶囊：可点重命名，右侧内嵌删除按钮。
   删除按钮用 span[role=button] 而不是嵌套 <button>——HTML 不允许 button 套 button，
   浏览器会把结构拆坏，导致点击区域错乱。 */
.tag-chip {
  @apply inline-flex max-w-[12rem] items-center gap-1 rounded-full border border-primary/20 bg-primary/10 py-1.5 pl-3 pr-1.5 text-sm font-medium text-primary transition-colors;
}

.tag-chip:hover {
  @apply border-primary/40 bg-primary/15;
}

.tag-chip-remove {
  @apply inline-flex h-5 w-5 shrink-0 cursor-pointer items-center justify-center rounded-full text-[13px] text-primary/60 transition-colors;
}

.tag-chip-remove:hover {
  @apply bg-red-500/10 text-red-600;
}

.dark .tag-chip-remove:hover {
  @apply text-red-400;
}
</style>
