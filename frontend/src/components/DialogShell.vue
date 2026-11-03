<template>
  <PopupModalHost
    ref="host"
    :title="title"
    :subtitle="subtitle"
    :icon="icon"
    :width="width"
    :close-label="closeLabel"
    :mask-close="maskClose"
    :footer="hasFooter"
    :content-max-height="contentMaxHeight"
    :initial-focus="initialFocus || ''"
    @close="onHostClosed"
  >
    <slot />
    <template v-if="hasFooter" #footer>
      <slot name="footer" />
    </template>
  </PopupModalHost>
</template>

<script setup>
/**
 * 声明式弹窗入口，内部完全跑在 PopupModal 上。
 *
 * 全站弹窗只有 PopupModal 一份实现：遮罩、动画、层级、焦点陷阱、Esc、滚动锁
 * 都由它负责。这个组件把「props + slot」翻译成 PopupModal 的调用方式，
 * 让习惯写模板的调用方不必手搓 DOM。
 *
 * 保留原有 props 接口（title / subtitle / icon / presentation / initialFocus …），
 * 因此所有调用点无需修改。presentation 仅作兼容保留，外观已由 PopupModal 统一。
 *
 * 关闭语义（很容易踩错）：
 *   - `close` 事件只在淡出动画**结束后**抛给父组件，父组件这时才能 v-if=false。
 *     提前销毁会让 Vue 把内容节点从弹窗里带走，而动画还在跑 —— 就是那个「闪一下」。
 *   - 内容里想主动关闭，用 `requestClose()`（通过 ref 拿）或 emit('request-close')，
 *     它们只负责「开始关闭」，不会提前通知父组件。
 *   - 组件内部若需要一个 `close()` 处理函数，绝不能再 emit('close') 给外面，
 *     那等于越过了动画。应该用 requestClose。
 */
import { computed, ref, useSlots } from 'vue';
import PopupModalHost from '@/components/PopupModalHost.vue';

defineProps({
  title: { type: String, default: '安全验证' },
  closeLabel: { type: String, default: '关闭验证弹窗' },
  subtitle: String,
  icon: String,
  // 旧接口遗留：早期用它切换 folder 版外观，现已由 PopupModal 统一，仅为不破坏调用点保留
  presentation: String,
  initialFocus: String,
  width: { type: String, default: 'md' },
  maskClose: { type: Boolean, default: true },
  contentMaxHeight: { type: String, default: '' },
});
const emit = defineEmits(['close']);

const slots = useSlots();
const hasFooter = computed(() => !!slots.footer);

const host = ref(null);

// 开始关闭：只播动画，不通知父组件
const requestClose = () => host.value?.requestClose();

// 动画播完后宿主才回调这里，这时再让父组件销毁才是安全的
const onHostClosed = () => emit('close');

defineExpose({ requestClose });
</script>
