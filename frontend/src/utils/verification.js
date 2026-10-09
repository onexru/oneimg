/** Compatibility defaults are shared by login and registration, never derived from the server verifier URL. */
export const LEGACY_POW_SCRIPT_URL = 'https://cha.eta.im/static/js/pow.min.js';
export const LEGACY_POW_WIDGET_URL = 'https://cha.eta.im/';

function providerURL(value, fallback) {
  if (value === undefined || value === null || value === '') return fallback;
  if (typeof value !== 'string') throw new Error('在线 POW 提供方配置无效，请联系管理员');
  const raw = value.trim();
  if (!raw) return fallback;
  if (raw.length > 2048 || !raw.startsWith('https://') || /[\s\\]/.test(raw)) {
    throw new Error('在线 POW 提供方地址无效，请联系管理员');
  }
  let url;
  try { url = new URL(raw); } catch { throw new Error('在线 POW 提供方地址无效，请联系管理员'); }
  if (url.protocol !== 'https:' || !url.hostname || url.username || url.password || url.search || raw.includes('?') || raw.includes('#') || (url.port && url.port !== '443')) {
    throw new Error('在线 POW 提供方必须使用无凭据、查询参数或片段的 HTTPS 地址，请联系管理员');
  }
  return url.href;
}

export function effectiveVerificationMethod(config = {}) {
  const method = config.verify_method || (config.pow_verify ? 'pow' : 'none');
  return method === 'pow' && config.pow_local_fallback === true ? 'cappow' : method;
}

export function legacyPowProvider(config = {}) {
  return {
    scriptURL: providerURL(config.pow_script_url, LEGACY_POW_SCRIPT_URL),
    widgetURL: providerURL(config.pow_widget_url, LEGACY_POW_WIDGET_URL),
  };
}

/** Bounded, URL-aware loading. Custom elements cannot be replaced safely in a live document. */
const pending = new Map();
const loadedSources = new Map();
export function loadVerificationScript(id, src, ready, timeout = 12000) {
  const source = new URL(src, document.baseURI).href;
  const previous = loadedSources.get(id);
  const inFlight = pending.get(id);
  if ((previous && previous !== source) || (inFlight && inFlight.source !== source)) {
    return Promise.reject(new Error('验证提供方已更改，请刷新页面后重试'));
  }
  if (inFlight) return inFlight.promise;
  if (ready()) {
    // Do not silently reuse another provider's already-registered pow-widget.
    if (id === 'online-pow-script' && !previous && document.getElementById(id)?.src !== source) {
      return Promise.reject(new Error('验证提供方已更改，请刷新页面后重试'));
    }
    loadedSources.set(id, source);
    return Promise.resolve();
  }
  let script;
  const promise = new Promise((resolve, reject) => {
    document.getElementById(id)?.remove();
    script = document.createElement('script');
    script.id = id; script.src = source; script.async = true;
    let settled = false;
    const finish = error => {
      if (settled) return;
      settled = true;
      clearTimeout(timer); script.onload = script.onerror = null;
      if (error) { script.remove(); reject(error); }
      else { loadedSources.set(id, source); resolve(); }
    };
    const timer = setTimeout(() => finish(new Error('验证组件加载超时，请重试或联系管理员')), timeout);
    script.onload = () => finish(ready() ? null : new Error('验证组件不可用，请重试'));
    script.onerror = () => finish(new Error('验证组件加载失败，请检查网络后重试'));
    document.head.appendChild(script);
  });
  const tracked = promise.finally(() => pending.delete(id));
  pending.set(id, { source, promise: tracked });
  return tracked;
}

/** Mount only for the current dialog attempt; stale loads/events cannot submit authentication. */
export async function mountLegacyPowWidget(container, config, { isCurrent, onReady, onSolve, onError }) {
  const { scriptURL, widgetURL } = legacyPowProvider(config);
  await loadVerificationScript('online-pow-script', scriptURL, () => !!customElements.get('pow-widget'));
  if (!isCurrent() || !container.isConnected) return null;
  const widget = document.createElement('pow-widget');
  widget.id = 'pow';
  widget.setAttribute('data-pow-api-endpoint', widgetURL);
  const current = callback => event => {
    if (isCurrent() && widget.isConnected) callback(event);
  };
  widget.addEventListener('load', current(onReady));
  widget.addEventListener('ready', current(onReady));
  widget.addEventListener('solve', current(onSolve));
  widget.addEventListener('error', current(() => onError(new Error('验证服务不可用，请重试'))));
  container.appendChild(widget);
  return widget;
}
