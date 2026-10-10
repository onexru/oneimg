import { ref, computed, onMounted, onUnmounted } from 'vue';
import { createFolderApi, folderAccount, FOLDER_LIMIT } from '@/utils/folders.js';

// View-scoped only: never cache folder IDs or names in cross-account storage.
export function useFolders(baseUrl = '') {
  const account = ref(folderAccount());
  const folders = ref([]), unfiledCount = ref(0), loading = ref(false), loaded = ref(false), error = ref('');
  const api = createFolderApi(baseUrl);
  let disposed = false, generation = 0;
  const authenticated = computed(() => !!account.value);
  const atLimit = computed(() => folders.value.length >= FOLDER_LIMIT);
  const totalImages = computed(() => unfiledCount.value + folders.value.reduce((sum, folder) => sum + Number(folder.image_count || 0), 0));
  async function reload() {
    const owner = folderAccount(), request = ++generation;
    if (owner !== account.value) {
      account.value = owner; folders.value = []; unfiledCount.value = 0; loaded.value = false;
    }
    error.value = '';
    if (!owner) { loading.value = false; return; }
    loading.value = true;
    try {
      const data = await api.list();
      if (disposed || request !== generation || owner !== folderAccount()) return;
      folders.value = Array.isArray(data?.folders) ? data.folders : [];
      unfiledCount.value = Number(data?.unfiled_count || 0);
      loaded.value = true;
    } catch (caught) {
      if (!disposed && request === generation && owner === folderAccount()) error.value = caught.message || '获取文件夹失败';
    } finally {
      if (!disposed && request === generation) loading.value = false;
    }
  }
  function checkAccount() { if (folderAccount() !== account.value) reload(); }
  onMounted(() => { reload(); window.addEventListener('storage', checkAccount); window.addEventListener('focus', checkAccount); });
  onUnmounted(() => { disposed = true; generation++; window.removeEventListener('storage', checkAccount); window.removeEventListener('focus', checkAccount); });
  return { account, authenticated, folders, unfiledCount, loading, loaded, error, atLimit, totalImages, reload };
}
