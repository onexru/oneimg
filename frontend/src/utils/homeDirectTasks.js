import { ref, computed } from 'vue';
import Message from './message.js';
import { boundedPage, RECENT_IMAGE_LIMIT } from './renderBounds.js';
import { canDirectUpload, createDirectUploadApi, fileSha256, isMissingObject, MAX_RECENT_TASKS,
  mergeTasks, mergeImages, newClientId, runDirectUpload, safeMessage } from './directUpload.js';
/** Owns ephemeral Files, transfer controllers, same-task retries and bounded history. */
export function createHomeDirectTasks({ API_BASE_URL, directTasks, recentImages, legacyUploading,
  uploadProgress, uploadingCount, multiStorageSync, storageConfigLoaded, presetBuckets,
  selectedBucket, getUploadFolderId = () => 0, fileInput, validateFiles, loadRecentImages, isDisposed, onSingleUpload = () => {} }) {
let directPollTimer = null;

const lifetimeController = new AbortController();
const directRefreshing = ref(false);
const directTasksError = ref('');

const resumeTask = ref(null);
const sessions = new Map(); // Files and SHA-256 remain in memory only, never localStorage.
const taskControllers = new Map();
const deletedImageIds = new Set();
// Browser UI authenticates exclusively with the server-issued HttpOnly cookie.
const authHeaders = () => ({});
const directApi = createDirectUploadApi({ baseUrl: API_BASE_URL, headers: authHeaders });
const currentUser = () => {
  try { return JSON.parse(localStorage.getItem('userInfo') || '{}'); } catch { return {}; }
};
const realUser = () => !!currentUser().username && !currentUser().isTourist && [1, 3].includes(Number(currentUser().role));
const directEnabled = () => canDirectUpload({ user: currentUser(), multiStorageSync: multiStorageSync.value,
  configLoaded: storageConfigLoaded.value, bucket: presetBuckets.value.find(bucket => bucket.id == selectedBucket.value) });
const uploadSummary = computed(() => {
  if (!legacyUploading.value) return '正在传输文件或确认任务，详细进度见下方上传任务';
  if (uploadProgress.value === 100) return '文件已发送，等待服务器处理主图并返回结果';
  return `正在传输 ${uploadingCount.value} 个文件${uploadProgress.value == null ? '' : `（${Math.round(uploadProgress.value)}%）`}`;
});
const directTaskForImage = id => directTasks.value.find(task => String(task.image_id || task.image?.id) === String(id));
const unlinkedDirectTasks = computed(() => directTasks.value.filter(task =>
  !recentImages.value.some(image => String(image.id) === String(task.image_id || task.image?.id))));
const taskKey = task => task.client_id || task.id;
const patchTask = (key, patch) => {
  if (isDisposed()) return;
  directTasks.value = directTasks.value.map(task => taskKey(task) === key ? { ...task, ...patch } : task);
};
const usableTasks = () => directTasks.value.filter(task => !deletedImageIds.has(String(task.image_id || task.image?.id)));
const acceptDirectTask = value => {
  if (isDisposed()) return;
  const previous = directTasks.value.find(task => task.id === value.id);
  directTasks.value = mergeTasks(directTasks.value, [value]);
  const task = directTasks.value.find(item => item.id === value.id);
  if (task && ['receiving', 'queued', 'processing', 'ready', 'cancelled'].includes(task.status)) {
    patchTask(taskKey(task), { clientError: '', clientRetryable: false });
    const session = sessions.get(taskKey(task));
    if (session && task.status !== 'receiving') { session.file = null; patchTask(taskKey(task), { hasFile: false }); }
  }
  recentImages.value = boundedPage(mergeImages(recentImages.value, usableTasks()), RECENT_IMAGE_LIMIT);
  const retainedKeys = new Set(directTasks.value.map(taskKey));
  for (const key of sessions.keys()) if (!retainedKeys.has(key) && !taskControllers.has(key)) sessions.delete(key);
  // Fetch normal image metadata once on publication (tags, uploader role, storage replicas).
  if (previous && value.status === 'ready' && previous.status !== 'ready') loadRecentImages();
  // Only uploads initiated as a single file in this view may auto-open, once the main image is usable.
  const session = task && sessions.get(taskKey(task));
  if (session?.previewOnReady && task.status === 'ready' && task.image?.id && task.image.url &&
      !deletedImageIds.has(String(task.image.id))) {
    session.previewOnReady = false;
    onSingleUpload({ ...task.image, bucket_id: task.bucket_id,
      folder_id: task.image.folder_id ?? task.folder_id ?? session.folderId, tags: task.image.tags || [] });
  }
};
const refreshDirectTasks = async () => {
  if (isDisposed() || !realUser() || directRefreshing.value) return;
  directRefreshing.value = true;
  try {
    const result = await directApi.list(lifetimeController.signal);
    if (!Array.isArray(result?.tasks)) throw new Error('任务列表响应无效');
    result.tasks.forEach(acceptDirectTask);
    directTasksError.value = '';
  } catch (error) {
    if (!isDisposed() && error.name !== 'AbortError') directTasksError.value = safeMessage(error.message, '任务状态暂时无法刷新；不会自动重复传输文件');
  } finally {
    directRefreshing.value = false;
    if (!isDisposed() && realUser()) {
      clearTimeout(directPollTimer);
      const active = directTasks.value.some(task => task.busy || ['receiving', 'queued', 'processing'].includes(task.status) ||
        (task.status === 'ready' && ['pending', 'processing'].includes(task.thumbnail_status)));
      directPollTimer = setTimeout(refreshDirectTasks, active ? 2500 : 15000);
    }
  }
};
const taskError = (task, error) => patchTask(taskKey(task), {
  clientError: safeMessage(error.message, '请求未确认，请刷新任务状态'),
  clientRetryable: error.retryable === true, busy: false, phase: '',
});
const runSession = async session => {
  const key = session.clientId;
  if (isDisposed() || taskControllers.has(key)) return;
  const controller = new AbortController();
  taskControllers.set(key, controller);
  patchTask(key, { busy: true, clientError: '', clientRetryable: false, hasFile: !!session.file });
  try {
    await runDirectUpload(session, { api: directApi, signal: controller.signal,
      onTask: acceptDirectTask, onState: patch => patchTask(key, patch) });
  } catch (error) {
    if (error.name !== 'AbortError') taskError({ client_id: key }, error);
  } finally {
    if (taskControllers.get(key) === controller) {
      taskControllers.delete(key);
      patchTask(key, { busy: false, phase: '' });
    }
    if (!isDisposed()) refreshDirectTasks();
  }
};
const startDirectFiles = async files => {
  const activeCount = directTasks.value.filter(task => task.busy || ['awaiting_upload', 'receiving', 'queued', 'processing'].includes(task.status)).length;
  if (files.length + activeCount > MAX_RECENT_TASKS) {
    Message.warning(`最多同时保留 ${MAX_RECENT_TASKS} 个未完成任务，请先完成或取消已有任务后再选择文件`);
    return;
  }
  const newSessions = [];
  const folderId = getUploadFolderId();
  try {
    for (const file of files) {
      const clientId = newClientId();
      const session = { clientId, bucketId: selectedBucket.value, folderId, file,
        previewOnReady: files.length === 1 };
      sessions.set(clientId, session);
      directTasks.value = mergeTasks(directTasks.value, [{ client_id: clientId, filename: file.name,
        status: 'awaiting_upload', created_at: new Date().toISOString() }]);
      patchTask(clientId, { hasFile: true, busy: true, phase: 'waiting' });
      newSessions.push(session);
    }
    // Bounded concurrency avoids hashing/retaining additional ArrayBuffers for a large batch.
    const worker = async () => {
      while (newSessions.length && !isDisposed()) {
        const session = newSessions.shift();
        const task = directTasks.value.find(item => taskKey(item) === session.clientId);
        if (task?.status !== 'cancelled') await runSession(session);
      }
    };
    await Promise.all([worker(), worker()]);
  } catch (error) {
    Message.error(safeMessage(error.message));
  }
};
const taskAction = async (task, operation) => {
  const key = taskKey(task);
  if (isDisposed() || taskControllers.has(key)) return;
  const controller = new AbortController();
  taskControllers.set(key, controller);
  patchTask(key, { busy: true, phase: 'confirming', clientError: '', clientRetryable: false });
  try {
    await operation(controller.signal);
    if (!controller.signal.aborted && !isDisposed() && task.id) acceptDirectTask(await directApi.get(task.id, controller.signal));
  } catch (error) {
    if (error.name !== 'AbortError') taskError(task, error);
  } finally {
    if (taskControllers.get(key) === controller) {
      taskControllers.delete(key);
      patchTask(key, { busy: false, phase: '' });
    }
    if (!isDisposed()) refreshDirectTasks();
  }
};
const confirmDirectTask = task => taskAction(task, async signal => {
  try { acceptDirectTask(await directApi.complete(task.id, signal)); }
  catch (error) {
    if (!isMissingObject(error)) throw error;
    patchTask(taskKey(task), { clientError: '存储尚未收到文件，请重试或重新选择原文件', clientRetryable: true });
  }
});
const retryDirectTask = async task => {
  if (task.busy) return;
  if (task.status === 'failed' || (task.status === 'ready' && task.thumbnail_status === 'failed')) {
    return taskAction(task, async signal => { await directApi.retry(task.id, signal); });
  }
  if (task.status === 'ready') return refreshDirectTasks();
  const session = sessions.get(taskKey(task));
  if (session?.file) return runSession(session);
  if (task.status === 'awaiting_upload') return reselectDirectFile(task);
  return confirmDirectTask(task);
};
const cancelDirectTask = async task => {
  const key = taskKey(task);
  const existing = taskControllers.get(key);
  existing?.abort();
  taskControllers.delete(key);
  patchTask(key, { busy: true, phase: 'cancelling' });
  const controller = new AbortController();
  taskControllers.set(key, controller);
  try {
    // A create request can commit after an aborted response. Recover by the same client UUID.
    let id = task.id || sessions.get(key)?.taskId;
    if (!id && existing) {
      const result = await directApi.list(controller.signal);
      const recovered = result.tasks?.find(item => item.client_id === key);
      if (recovered) { id = recovered.id; acceptDirectTask(recovered); }
    }
    if (id) {
      await directApi.cancel(id, controller.signal);
      acceptDirectTask(await directApi.get(id, controller.signal));
    } else {
      patchTask(key, { status: 'cancelled', clientError: existing ? '浏览器已停止；如服务器稍后创建任务，请刷新后取消该任务。' : '' });
    }
    sessions.delete(key);
    patchTask(key, { hasFile: false });
  } catch (error) {
    if (error.name !== 'AbortError') taskError(task, error);
  } finally {
    if (taskControllers.get(key) === controller) {
      taskControllers.delete(key);
      patchTask(key, { busy: false, phase: '' });
    }
    if (!isDisposed()) refreshDirectTasks();
  }
};
const reselectDirectFile = task => {
  resumeTask.value = task;
  fileInput.value?.click();
};
const handleResumeFile = async event => {
  const file = event.target.files?.[0];
  event.target.value = '';
  const task = resumeTask.value;
  resumeTask.value = null;
  if (!file || !task || task.busy || !validateFiles([file]).length) return;
  const key = taskKey(task);
  const old = sessions.get(key);
  if (file.name !== task.filename) {
    Message.warning('请选择原任务中的同名文件，服务器还会核验大小及 SHA-256');
    return;
  }
  const session = { ...old, clientId: task.client_id, taskId: task.id, file,
    bucketId: task.bucket_id || old?.bucketId, folderId: task.folder_id ?? old?.folderId ?? 0 };
  // Reselected files must be hashed again, never reuse a previous file's hash blindly.
  await taskAction(task, async signal => {
    const digest = await fileSha256(file, signal);
    if (old?.sha256 && old.sha256 !== digest) throw new Error('文件内容与原任务不一致，请重新选择原文件');
    session.sha256 = digest;
    sessions.set(key, session);
    patchTask(key, { hasFile: true });
  });
  if (sessions.get(key) === session && !isDisposed()) await runSession(session);
};


const disposeDirectTasks = () => {
  lifetimeController.abort();
  for (const controller of taskControllers.values()) controller.abort();
  taskControllers.clear(); sessions.clear();
  clearTimeout(directPollTimer);
};
return { directRefreshing, directTasksError, resumeTask, deletedImageIds, currentUser, realUser,
  directEnabled, uploadSummary, directTaskForImage, unlinkedDirectTasks, usableTasks,
  refreshDirectTasks, startDirectFiles, retryDirectTask, cancelDirectTask, confirmDirectTask,
  reselectDirectFile, handleResumeFile, disposeDirectTasks };
}
