import { ref, computed, watch, onBeforeUnmount } from 'vue';
import { nextRenderCount } from '@/utils/chunkedRendering.js';
export function useChunkedImages(images) {
  const count = ref(24);
  const sentinel = ref(null);
  let observer;
  const visibleImages = computed(() => images.value.slice(0, count.value));
  const hasHiddenImages = computed(() => count.value < images.value.length);
  const revealImages = () => { count.value = nextRenderCount(count.value, images.value.length); };
  watch(images, (next, previous) => {
    // Background status refresh must not collapse a user's already-rendered selection.
    if (next.map(item => item.id).join(',') !== previous.map(item => item.id).join(',')) count.value = 24;
  });
  watch(sentinel, element => {
    observer?.disconnect();
    if (!element || !globalThis.IntersectionObserver) return;
    observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) revealImages();
    }, { rootMargin: '400px' });
    observer.observe(element);
  }, { flush: 'post' });
  onBeforeUnmount(() => observer?.disconnect());
  return { sentinel, visibleImages, hasHiddenImages, revealImages };
}
