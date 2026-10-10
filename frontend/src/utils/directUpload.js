// Upload instructions are bearer credentials: keep them inside the runner, never in UI/history.
export const MAX_RECENT_TASKS = 12;
const API_PATH = '/api/uploads/direct';
const TRANSFER_STATES = new Set(['awaiting_upload']);
const BACKGROUND_STATES = new Set(['receiving', 'queued', 'processing', 'ready']);
const HARD_CODE = /permission|forbidden|unauthori[sz]ed|quota|too_large|size_(?:mismatch|limit)|hash|checksum|digest|disabled|expired_task|task_expired|invalid_(?:file|image|bucket)/i;

export class UploadError extends Error {
  constructor(message, { kind = 'control', code = '', status = 0, retryable = false } = {}) {
    super(message);
    this.name = 'UploadError';
    Object.assign(this, { kind, code, status, retryable: retryable && !HARD_CODE.test(code) &&
      (!([401, 403, 413, 422].includes(status)) || (kind === 'transport' && code === 'signature_expired')) });
  }
}

export const abortError = () => new DOMException('上传已停止', 'AbortError');
const checkAbort = signal => { if (signal?.aborted) throw abortError(); };
const transientStatus = status => status === 408 || status === 429 || status >= 500;
export const isMissingObject = error => error instanceof UploadError &&
  /^(object_missing|object_not_found|upload_missing|upload_not_found|upload_incomplete|staging_missing|source_missing)$/i.test(error.code);

export function safeMessage(message, fallback = '请求失败，请稍后重试') {
  // Also protect against accidentally unsanitized upstream errors.
  if (typeof message !== 'string' || /https?:\/\/|X-Amz-|Signature=/i.test(message)) return fallback;
  return message.slice(0, 300) || fallback;
}

export function safeTask(task) {
  if (!task || typeof task !== 'object') throw new UploadError('任务响应无效');
  const fields = ['id', 'client_id', 'bucket_id', 'folder_id', 'filename', 'status', 'transport', 'thumbnail_status',
    'error_code', 'retryable', 'attempts', 'thumbnail_attempts', 'image_id', 'created_at', 'updated_at', 'next_retry_at'];
  const clean = Object.fromEntries(fields.filter(key => task[key] !== undefined).map(key => [key, task[key]]));
  clean.message = safeMessage(task.message, '');
  // The image contains permanent OneImg links only; never copy the upload object.
  if (task.image) {
    clean.image = Object.fromEntries(['id', 'url', 'thumbnail_url', 'thumbnail', 'filename', 'file_size',
      'width', 'height', 'created_at', 'tags', 'storage', 'uploader_role', 'folder_id'].filter(key => task.image[key] !== undefined)
      .map(key => [key, task.image[key]]));
    for (const key of ['url', 'thumbnail_url', 'thumbnail']) {
      if (/X-Amz-|[?&]Signature=/i.test(clean.image[key] || '')) delete clean.image[key];
    }
  } else if (task.image === null) clean.image = null;
  return clean;
}

export function mergeTasks(current, incoming, limit = MAX_RECENT_TASKS) {
  const merged = current.map(task => ({ ...task }));
  for (const value of incoming) {
    const task = safeTask(value);
    const index = merged.findIndex(old => (task.id && old.id === task.id) ||
      (task.client_id && old.client_id === task.client_id));
    if (index < 0) merged.push(task);
    else {
      const old = merged[index];
      if (!old.updated_at || !task.updated_at || Date.parse(task.updated_at) >= Date.parse(old.updated_at)) {
        merged[index] = { ...old, ...task };
      }
    }
  }
  return merged.sort((a, b) => {
    const active = task => task.busy || ['awaiting_upload', 'receiving', 'queued', 'processing'].includes(task.status);
    return Number(active(b)) - Number(active(a)) ||
      Date.parse(b.created_at || 0) - Date.parse(a.created_at || 0);
  }).slice(0, limit);
}

export function mergeImages(images, tasks, limit = 12) {
  const byId = new Map(images.map(image => [String(image.id), { ...image }]));
  for (const task of tasks) {
    if (!task.image?.id || task.status !== 'ready') continue;
    const key = String(task.image.id);
    const old = byId.get(key);
    byId.set(key, { ...old, ...task.image, created_at: task.image.created_at || old?.created_at || task.created_at,
      thumbnail: task.image.thumbnail_url || task.image.thumbnail || old?.thumbnail || '' });
  }
  return [...byId.values()].sort((a, b) => Number(b.id) - Number(a.id)).slice(0, limit);
}

export function canDirectUpload({ user, bucket, multiStorageSync, configLoaded = true }) {
  return configLoaded && !!user?.username && user.isTourist !== true &&
    [1, 3].includes(Number(user.role)) && !multiStorageSync && bucket?.type === 'r2';
}

export function delay(ms, signal) {
  return new Promise((resolve, reject) => {
    checkAbort(signal);
    const abort = () => { clearTimeout(timer); reject(abortError()); };
    const timer = setTimeout(() => { signal?.removeEventListener('abort', abort); resolve(); }, ms);
    signal?.addEventListener('abort', abort, { once: true });
  });
}

export function newClientId(cryptoImpl = globalThis.crypto) {
  if (cryptoImpl?.randomUUID) return cryptoImpl.randomUUID();
  if (!cryptoImpl?.getRandomValues) throw new UploadError('浏览器不支持安全上传，请使用 HTTPS 和新版浏览器', { kind: 'browser' });
  const bytes = cryptoImpl.getRandomValues(new Uint8Array(16));
  bytes[6] = (bytes[6] & 15) | 64;
  bytes[8] = (bytes[8] & 63) | 128;
  const hex = Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

export async function fileSha256(file, signal, cryptoImpl = globalThis.crypto) {
  checkAbort(signal);
  if (!cryptoImpl?.subtle) throw new UploadError('无法计算 SHA-256，请使用 HTTPS 和支持 Web Crypto 的浏览器', { kind: 'browser' });
  const bytes = await file.arrayBuffer();
  checkAbort(signal);
  const digest = await cryptoImpl.subtle.digest('SHA-256', bytes);
  checkAbort(signal);
  return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, '0')).join('');
}

// Shared by direct PUT, proxy fallback, and the existing multipart uploader.
export function xhrUpload({ url, method = 'POST', body, headers = {}, signal, onProgress,
  direct = false, returnEnvelope = false, timeout = 180000, xhrFactory = () => new XMLHttpRequest() }) {
  return new Promise((resolve, reject) => {
    checkAbort(signal);
    const xhr = xhrFactory();
    let settled = false;
    const finish = (callback, value) => {
      if (settled) return;
      settled = true;
      signal?.removeEventListener('abort', abort);
      callback(value);
    };
    const abort = () => { xhr.abort(); finish(reject, abortError()); };
    const networkError = () => finish(reject, new UploadError(direct ? 'R2 连接中断或跨域响应不可读' : '上传连接中断，结果待确认', {
      kind: direct ? 'transport' : 'control', retryable: true,
    }));
    xhr.open(method, url, true);
    // Cookies and app authorization must NEVER accompany the signed R2 request.
    xhr.withCredentials = !direct;
    xhr.timeout = timeout;
    for (const [name, value] of Object.entries(headers)) {
      if (direct && /^(authorization|cookie|proxy-authorization)$/i.test(name)) continue;
      xhr.setRequestHeader(name, value);
    }
    xhr.upload.onprogress = event => {
      if (settled || signal?.aborted) return;
      onProgress?.({ loaded: event.loaded, total: event.lengthComputable ? event.total : null,
        percent: event.lengthComputable && event.total > 0 ? Math.min(100, event.loaded / event.total * 100) : null });
    };
    xhr.onload = () => {
      if (direct) {
        if (xhr.status >= 200 && xhr.status < 300) return finish(resolve, null);
        const expired = /<Code>(ExpiredToken|RequestExpired)<\/Code>/.test(xhr.responseText || '');
        return finish(reject, new UploadError(expired ? '上传签名已过期，正在重新获取' : `R2 上传返回 HTTP ${xhr.status}`, {
          kind: 'transport', status: xhr.status, code: expired ? 'signature_expired' : '',
          retryable: expired || transientStatus(xhr.status) || xhr.status === 0,
        }));
      }
      let result;
      try { result = JSON.parse(xhr.responseText); } catch {
        return finish(reject, new UploadError('服务器响应无法确认，请刷新任务状态', {
          kind: 'control', status: xhr.status, retryable: transientStatus(xhr.status) || xhr.status === 0,
        }));
      }
      // Legacy batch callers need the entire per-file result even on HTTP partial failure.
      if (returnEnvelope) return finish(resolve, { status: xhr.status, result });
      if (xhr.status >= 200 && xhr.status < 300 && result.code === 200) finish(resolve, result.data);
      else finish(reject, new UploadError(safeMessage(result.message, '上传失败'), {
        kind: 'control', code: result.error_code, status: xhr.status,
        retryable: result.retryable === true || (result.retryable === undefined && transientStatus(xhr.status)),
      }));
    };
    xhr.onerror = networkError;
    xhr.ontimeout = networkError;
    xhr.onabort = () => finish(reject, abortError());
    signal?.addEventListener('abort', abort, { once: true });
    try { checkAbort(signal); xhr.send(body); } catch (error) {
      finish(reject, error.name === 'AbortError' ? error : new UploadError('无法发送上传请求', {
        kind: direct ? 'transport' : 'control', retryable: true,
      }));
    }
  });
}

export function createDirectUploadApi({ baseUrl = '', fetchImpl = globalThis.fetch,
  headers = () => ({}), xhr = xhrUpload, timeout = 35000 } = {}) {
  const request = async (path = '', { method = 'GET', body, signal } = {}) => {
    checkAbort(signal);
    const controller = new AbortController();
    let timedOut = false;
    const abort = () => controller.abort();
    signal?.addEventListener('abort', abort, { once: true });
    const timer = setTimeout(() => { timedOut = true; controller.abort(); }, timeout);
    try {
      const response = await fetchImpl(`${baseUrl}${API_PATH}${path}`, {
        method, credentials: 'include', cache: 'no-store', signal: controller.signal,
        headers: { ...headers(), ...(body ? { 'Content-Type': 'application/json' } : {}) },
        ...(body ? { body: JSON.stringify(body) } : {}),
      });
      let result;
      try { result = await response.json(); } catch (error) {
        if (signal?.aborted || timedOut) throw error;
        throw new UploadError('任务服务响应无法确认，请刷新状态', { status: response.status, retryable: true });
      }
      if (!response.ok || result.code !== 200) throw new UploadError(safeMessage(result.message), {
        status: response.status, code: result.error_code,
        retryable: result.retryable === true || (result.retryable === undefined && transientStatus(response.status)),
      });
      return result.data;
    } catch (error) {
      if (signal?.aborted) throw abortError();
      if (timedOut) throw new UploadError('任务服务确认超时，请重试查询状态', { retryable: true });
      if (error instanceof UploadError) throw error;
      throw new UploadError('任务服务暂时无法连接，请重试确认状态', { retryable: true });
    } finally {
      clearTimeout(timer);
      signal?.removeEventListener('abort', abort);
    }
  };
  const path = id => `/${encodeURIComponent(id)}`;
  return {
    list: signal => request('', { signal }),
    get: (id, signal) => request(path(id), { signal }),
    create: (body, signal) => request('', { method: 'POST', body, signal }),
    sign: (id, signal) => request(`${path(id)}/sign`, { method: 'POST', signal }),
    complete: (id, signal) => request(`${path(id)}/complete`, { method: 'POST', signal }),
    retry: (id, signal) => request(`${path(id)}/retry`, { method: 'POST', signal }),
    cancel: (id, signal) => request(path(id), { method: 'DELETE', signal }),
    fallback: (id, file, signal, onProgress, folderId = 0) => {
      const body = new FormData();
      body.append('file', file);
      body.append('folder_id', String(folderId));
      return xhr({ url: `${baseUrl}${API_PATH}${path(id)}/fallback`, body,
        headers: headers(), signal, onProgress, direct: false });
    },
  };
}

// session lives only in memory and is reused for independent retries (same UUID/task/hash).
export async function runDirectUpload(session, { api, signal, onTask = () => {}, onState = () => {},
  transfer = xhrUpload, sleep = delay, hash = fileSha256 } = {}) {
  const state = patch => { checkAbort(signal); onState(patch); };
  const accept = task => {
    checkAbort(signal);
    if (!task?.id || !task.status) throw new UploadError('任务响应不完整');
    session.taskId = task.id;
    onTask(safeTask(task));
    return task;
  };
  // Control-plane retries never spend PUT/fallback attempts.
  const control = async operation => {
    for (let attempt = 0; ; attempt++) {
      checkAbort(signal);
      try { return await operation(); } catch (error) {
        if (error.name === 'AbortError' || !error.retryable || isMissingObject(error) || attempt >= 2) throw error;
        await sleep(400 * (attempt + 1), signal);
      }
    }
  };
  const probe = async () => {
    state({ phase: 'confirming' });
    try {
      const task = accept(await control(() => api.complete(session.taskId, signal)));
      if (BACKGROUND_STATES.has(task.status) || task.status === 'cancelled' || task.status === 'failed') return task;
      // A successful but still awaiting response is not proof that bytes are missing.
      throw new UploadError('上传结果尚未确认，请稍后检查任务', { retryable: true });
    } catch (error) {
      if (isMissingObject(error)) return null;
      throw error;
    }
  };
  let task;
  if (session.taskId) {
    task = accept(await control(() => api.get(session.taskId, signal)));
    if (!TRANSFER_STATES.has(task.status)) return task;
    const confirmed = await probe();
    if (confirmed) return confirmed;
  } else {
    state({ phase: 'hashing', percent: null });
    session.sha256 ||= await hash(session.file, signal);
    state({ phase: 'creating' });
    task = accept(await control(() => api.create({ client_id: session.clientId,
      bucket_id: Number(session.bucketId), filename: session.file.name, size: session.file.size,
      content_type: session.file.type, sha256: session.sha256, folder_id: Number(session.folderId ?? 0) }, signal)));
    if (!TRANSFER_STATES.has(task.status)) return task;
  }
  if (!session.file) throw new UploadError('浏览器未保留原文件，请重新选择同一文件', { kind: 'file' });
  const progress = details => state(details);
  for (let attempt = 1; attempt <= 3; attempt++) {
    if (attempt > 1) {
      const confirmed = await probe();
      if (confirmed) return confirmed;
    }
    if (attempt > 1 || !task.upload) {
      state({ phase: 'signing' });
      task = accept(await control(() => api.sign(session.taskId, signal)));
      if (!TRANSFER_STATES.has(task.status)) return task;
    }
    if (!task.upload?.url || task.upload.method !== 'PUT') throw new UploadError('服务器未返回有效的直传指令');
    state({ phase: 'uploading_direct', directAttempt: attempt, percent: 0, loaded: 0, total: session.file.size });
    let transferError = null;
    try {
      await transfer({ url: task.upload.url, method: 'PUT', headers: task.upload.headers || {},
        body: session.file, direct: true, signal, onProgress: progress });
    } catch (error) {
      if (error.name === 'AbortError') throw error;
      transferError = error;
    }
    // Success, failure, timeout, CORS and lost responses all go through the same HEAD confirmation.
    const confirmed = await probe();
    if (confirmed) return confirmed;
    if (transferError && !transferError.retryable) throw transferError;
    if (!transferError) {
      // A 2xx PUT followed by an explicit missing object is retryable, unlike a control-plane outage.
      transferError = new UploadError('R2 尚未确认文件，正在重试', { kind: 'transport', retryable: true });
    }
    if (attempt < 3) {
      state({ phase: 'waiting_retry' });
      await sleep(500 * attempt, signal);
    }
  }
  for (let attempt = 1; attempt <= 2; attempt++) {
    // Do not fall back when task/status service is unavailable or a worker already owns the bytes.
    state({ phase: 'confirming' });
    task = accept(await control(() => api.get(session.taskId, signal)));
    if (!TRANSFER_STATES.has(task.status)) return task;
    const confirmed = await probe();
    if (confirmed) return confirmed;
    state({ phase: 'uploading_proxy', proxyAttempt: attempt, percent: 0, loaded: 0, total: null });
    let error;
    try {
      task = accept(await api.fallback(session.taskId, session.file, signal, progress, Number(session.folderId ?? task.folder_id ?? 0)));
      if (!TRANSFER_STATES.has(task.status)) return task;
    } catch (caught) {
      if (caught.name === 'AbortError') throw caught;
      error = caught;
    }
    // Proxy requests can succeed on the server after the browser times out.
    task = accept(await control(() => api.get(session.taskId, signal)));
    if (!TRANSFER_STATES.has(task.status)) return task;
    const recovered = await probe();
    if (recovered) return recovered;
    if (error && !error.retryable) throw error;
    if (attempt < 2) {
      state({ phase: 'waiting_retry' });
      await sleep(700, signal);
    } else throw error || new UploadError('中转上传未确认，请检查状态后独立重试', { retryable: true });
  }
}

export function taskPresentation(task) {
  const progress = task.percent == null ? '正在传输' : `${Math.round(task.percent)}%`;
  const localLabels = {
    waiting: '等待浏览器上传', hashing: '校验文件 SHA-256', creating: '创建上传任务', signing: '获取上传签名',
    uploading_direct: `直传 R2 · 第 ${task.directAttempt || 1}/3 次 · ${progress}`,
    uploading_proxy: `服务器中转 · 第 ${task.proxyAttempt || 1}/2 次 · ${progress}`,
    confirming: '确认文件已到达存储', waiting_retry: '连接不稳定，准备重试', cancelling: '正在确认取消',
  };
  const statusLabels = { awaiting_upload: '等待原文件上传', receiving: '服务器正在接收文件', queued: '等待后台处理主图',
    processing: '后台正在处理主图', ready: '主图已可用', failed: '后台处理失败', cancelled: '已取消' };
  const thumbnailLabels = { disabled: '缩略图未开启', pending: '缩略图等待处理', processing: '缩略图处理中',
    ready: '缩略图已就绪', failed: '缩略图失败，主图仍可用' };
  return {
    label: task.status === 'ready' ? statusLabels.ready :
      (task.busy && localLabels[task.phase]) || (task.clientError ? '上传需要处理' : statusLabels[task.status]) || '等待上传',
    thumbnail: task.status === 'ready' ? thumbnailLabels[task.thumbnail_status] || '' : '',
    showProgress: task.busy && ['uploading_direct', 'uploading_proxy'].includes(task.phase),
    needsFile: !task.busy && !task.hasFile && task.status === 'awaiting_upload',
    canRetry: !task.busy && task.status !== 'cancelled' &&
      (!!task.clientRetryable || (task.status === 'failed' && task.retryable) ||
      (task.status === 'ready' && task.thumbnail_status === 'failed' && task.retryable)),
    canConfirm: !!task.id && !task.busy && ['awaiting_upload', 'receiving'].includes(task.status),
    canCancel: task.phase !== 'cancelling' && !['ready', 'cancelled'].includes(task.status),
  };
}
