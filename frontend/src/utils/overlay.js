/** Shared overlay layers, scroll locking, focus and keyboard lifecycle. */
export const OVERLAY_LAYERS = Object.freeze({ dialog: 10000, loading: 20000, toast: 200000 });
const locks = new Set();
const dialogs = [];
let releaseScrollLock = null;

const SCROLL_KEYS = new Set(['PageUp', 'PageDown', 'Home', 'End', ' ', 'Spacebar']);

/**
 * 锁住背景滚动，同时保证「打开/关闭弹窗时页面不位移」。
 *
 * 不用 overflow:hidden，因为它有两个无法回避的副作用：
 *   1. 滚动条消失 → 视口宽 8px，header 从 1432 变 1440，右侧 fixed 元素整体右移；
 *      body 补 padding-right 也救不了，fixed 元素是按视口定位的。
 *   2. 滚动位置被清零 → 页面弹回顶部，关闭后也回不去。
 *
 * 改为保留滚动条、只拦截滚动输入：布局完全不变，位置自然保持。
 * 弹窗自己的内容区仍在 document 内，所以判定「是否在可滚动区域内」，
 * 在里面滚动时放行，滚到头也不把滚动链传给背景。
 */
export function lockScroll(owner) {
  if (locks.has(owner)) return;
  locks.add(owner);
  if (locks.size > 1) return;      // 已经有更外层的锁，不重复挂监听

  const root = document.documentElement;

  // 1) 滚轮 / 触摸板：只在「不在可滚动容器内」时拦掉
  const onWheel = (event) => {
    let node = event.target instanceof Element ? event.target : null;
    while (node && node !== document.body && node !== root) {
      const style = getComputedStyle(node);
      if (/(auto|scroll)/.test(style.overflowY) && node.scrollHeight > node.clientHeight) {
        const atTop = node.scrollTop <= 0;
        const atBottom = node.scrollTop + node.clientHeight >= node.scrollHeight - 1;
        const goingUp = event.deltaY < 0;
        // 容器还能滚就让它滚；已经到头则吞掉，避免穿透到背景
        if (!(atTop && goingUp) && !(atBottom && !goingUp)) return;
      }
      node = node.parentElement;
    }
    event.preventDefault();
  };

  // 2) 触摸：手机上滚动是 touchmove，方向靠记录起点判断
  let touchY = 0;
  const onTouchStart = (event) => { touchY = event.touches[0]?.clientY ?? 0; };
  const onTouchMove = (event) => {
    const y = event.touches[0]?.clientY ?? 0;
    const goingUp = y > touchY;
    touchY = y;
    let node = event.target instanceof Element ? event.target : null;
    while (node && node !== document.body && node !== root) {
      const style = getComputedStyle(node);
      if (/(auto|scroll)/.test(style.overflowY) && node.scrollHeight > node.clientHeight) {
        const atTop = node.scrollTop <= 0;
        const atBottom = node.scrollTop + node.clientHeight >= node.scrollHeight - 1;
        if (!(atTop && goingUp) && !(atBottom && !goingUp)) return;
      }
      node = node.parentElement;
    }
    event.preventDefault();
  };

  // 3) 键盘：空格/翻页/Home/End 会滚动页面，焦点在弹窗里时也要挡住。
  //    但有两类元素必须放行，否则会破坏可访问性：
  //      - 按钮/链接：空格是「按下」语义
  //      - 滑块：PageUp/PageDown/Home/End 是它的标准调值快捷键
  const onKeyDown = (event) => {
    if (!SCROLL_KEYS.has(event.key)) return;
    const target = event.target;
    if (!(target instanceof Element)) return event.preventDefault();
    if (target.closest('input, textarea, select, button, a[href], [contenteditable="true"], [role="button"]')) return;
    event.preventDefault();
  };

  // passive:false 是必须的，否则 preventDefault 无效
  document.addEventListener('wheel', onWheel, { passive: false });
  document.addEventListener('touchstart', onTouchStart, { passive: true });
  document.addEventListener('touchmove', onTouchMove, { passive: false });
  document.addEventListener('keydown', onKeyDown, { passive: false });

  releaseScrollLock = () => {
    document.removeEventListener('wheel', onWheel);
    document.removeEventListener('touchstart', onTouchStart);
    document.removeEventListener('touchmove', onTouchMove);
    document.removeEventListener('keydown', onKeyDown);
    releaseScrollLock = null;
  };
}

export function unlockScroll(owner) {
  if (!locks.delete(owner) || locks.size) return;
  releaseScrollLock?.();
}
export function nextDialogLayer() {
  return Math.max(OVERLAY_LAYERS.dialog - 2, ...dialogs.map(({ layer }) => layer)) + 2;
}
export function activateDialog(element, { close, labelId, initialFocus, layer = Number(element.style.zIndex) || nextDialogLayer() } = {}) {
  const previous = document.activeElement;
  const entry = { element, layer };
  dialogs.push(entry);
  lockScroll(entry);
  element.setAttribute('role', 'dialog');
  element.setAttribute('aria-modal', 'true');
  if (labelId) element.setAttribute('aria-labelledby', labelId);
  element.tabIndex = -1;
  const focusables = () => [...element.querySelectorAll('button:not([disabled]), a[href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex="0"]')]
    .filter(node => node.getClientRects().length && !node.closest('[inert]'));
  const keydown = event => {
    if (dialogs.at(-1) !== entry) return;
    if (event.key === 'Escape' && close) { event.preventDefault(); close(); }
    if (event.key === 'Tab') {
      const nodes = focusables();
      const first = nodes[0] || element;
      const last = nodes.at(-1) || element;
      if (!element.contains(document.activeElement) || (!event.shiftKey && document.activeElement === last) || (event.shiftKey && document.activeElement === first) || !nodes.length) {
        event.preventDefault(); (event.shiftKey ? last : first).focus();
      }
    }
  };
  document.addEventListener('keydown', keydown);
  const timer = setTimeout(() => {
    if (dialogs.at(-1) === entry) {
      const preferred = initialFocus ? element.querySelector(initialFocus) : null;
      (preferred && focusables().includes(preferred) ? preferred : focusables()[0] || element).focus();
    }
  }, 20);
  return () => {
    clearTimeout(timer);
    const top = dialogs.at(-1) === entry;
    const index = dialogs.indexOf(entry);
    if (index < 0) return;
    dialogs.splice(index, 1);
    document.removeEventListener('keydown', keydown);
    unlockScroll(entry);
    if (top) {
      const remaining = dialogs.at(-1)?.element;
      if (remaining && (!previous?.isConnected || !remaining.contains(previous))) remaining.focus();
      else if (previous?.isConnected) previous.focus();
    }
  };
}
