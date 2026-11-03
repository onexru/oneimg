<template>
  <Teleport to="body">
    <div ref="contentEl" class="hidden">
      <slot />
    </div>
    <!-- 按钮由插槽提供。这里用 display:contents 让 wrapper 的盒子消失，
         按钮直接成为 PopupModal footer 的 flex 子项，自动拿到它的 gap-3；
         否则 wrapper 会自成一个盒子，外层 gap 作用不到里面的按钮，
         多个按钮就会紧贴在一起。 -->
    <div ref="footerEl" class="hidden">
      <slot name="footer" />
    </div>
  </Teleport>
</template>

<script setup>
/**
 * Vue 侧的 PopupModal 宿主。
 *
 * 前端原本有两套弹窗：PopupModal（命令式，25 处）与 DialogShell（声明式 slot，6 处）。
 * 两者的遮罩、动画、层级、焦点陷阱、Esc、滚动锁各写了一遍，改一处要改两遍。
 *
 * 这个组件让声明式弹窗也跑在 PopupModal 上：
 *   - 外观与行为 100% 由 PopupModal 决定，只有一份实现
 *   - slot 内容仍然渲染在 Vue 组件树里，v-model / 事件 / 响应式全部照常工作
 *   - 把承载 slot 的真实 DOM 节点搬进 PopupModal 的内容区并去掉 hidden
 *
 * 搬的是节点本身而不是 clone：clone 会丢失 Vue 的事件监听与响应式更新。
 *
 * 注意：内容区的 padding 由插槽内容自己声明。命令式弹窗的内容容器带
 * `px-6 py-5`，但走 contentEl 这条路时容器只有 `min-h-0`（否则外部渲染的
 * 内容会与内边距叠加成双层留白）。因此每个 DialogShell 的根元素都要自带
 * 内边距，漏了就会贴着弹窗边缘，看起来像「padding 没了」。
 */
import { ref, onMounted, onBeforeUnmount, watch } from 'vue';
import PopupModal from '@/utils/popupModal.js';

const props = defineProps({
  title: { type: String, default: '提示' },
  subtitle: { type: String, default: '' },
  icon: { type: String, default: '' },
  width: { type: String, default: 'auto' },
  closeLabel: { type: String, default: '关闭弹窗' },
  showClose: { type: Boolean, default: true },
  maskClose: { type: Boolean, default: true },
  footer: { type: Boolean, default: true },
  contentMaxHeight: { type: String, default: '' },
  initialFocus: { type: String, default: '' },
});

const emit = defineEmits(['close']);

const contentEl = ref(null);
const footerEl = ref(null);
let modal = null;

onMounted(() => {
  // Teleport 出来的节点已经在 body 上；摘掉 hidden 后交给 PopupModal 接管
  if (contentEl.value) contentEl.value.classList.remove('hidden');
  if (footerEl.value) {
    footerEl.value.classList.remove('hidden');
    // display:contents 让按钮直接参与 footer 的 flex 布局，继承它的 12px 间距
    footerEl.value.style.display = 'contents';
  }

  modal = new PopupModal({
    title: props.title,
    subtitle: props.subtitle,
    icon: props.icon,
    width: props.width,
    closeLabel: props.closeLabel,
    showClose: props.showClose,
    maskClose: props.maskClose,
    showFooter: props.footer,
    contentMaxHeight: props.contentMaxHeight,
    contentEl: contentEl.value,
    footerEl: props.footer ? footerEl.value : undefined,
    onOpen: () => {
      // 注意：activateDialog 在 open() 里已按默认规则聚焦首个可聚焦元素，
      // 这里只在明确指定 initialFocus 时覆盖，避免抢走默认焦点。
      if (!props.initialFocus) return;
      const target = modal?.modal?.querySelector(props.initialFocus);
      if (target && typeof target.focus === 'function') target.focus();
    },
    onClose: () => emit('close'),
  });
  modal.open();
});

// 标题、副标题可能随状态变化（例如「新建」和「重命名」共用一个组件）
watch(() => [props.title, props.subtitle], () => modal?.update({ title: props.title, subtitle: props.subtitle }));

/**
 * 让插槽里的按钮也能走完整的关闭流程。
 *
 * 之前「完成」按钮直接写 `@click="open = false"`，父组件立刻 v-if=false，
 * Vue 卸载组件时把内容节点从弹窗里带走，而 300ms 淡出还在跑 ——
 * 用户看到的是空壳在慢慢消失，也就是那个「闪一下」。
 * 走这里才会先播动画、结束再通知父组件销毁。
 */
const requestClose = () => {
  if (!modal) return;
  modal.close();
};

defineExpose({ requestClose });

onBeforeUnmount(() => {
  // destroy 会把内容节点连同遮罩一起移出文档，组件卸载时节点已无引用，无需手动恢复
  modal?.destroy();
  modal = null;
});
</script>
