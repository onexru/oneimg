/**
 * 验证组件共用的脚本加载器。登录与注册共用，避免两处各写一份超时/重试逻辑。
 *
 * 注意：在线 POW（外部 pow-widget）已下线，这里只保留 Turnstile 与 cap-pow
 * 需要的通用加载能力。服务端不再返回 pow_* 配置，因此前端也不再有
 * 「按管理员配置切换外部验证提供方」的路径。
 */

// 人机验证方式只有三种，与后端 models.VerifyMethod* 保持一致
export const VERIFICATION_METHODS = Object.freeze(['none', 'turnstile', 'cappow']);

/**
 * 归一化服务端返回的验证方式。
 * 旧库可能残留 'pow'（在线 POW 已移除）或空值，统一回落到 none ——
 * 否则前端会去找一个早已不存在的 pow-widget，界面卡在「验证中」。
 */
export function effectiveVerificationMethod(config = {}) {
  const method = typeof config.verify_method === 'string' ? config.verify_method : '';
  return VERIFICATION_METHODS.includes(method) ? method : 'none';
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
