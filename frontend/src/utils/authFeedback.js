export function authErrorMessage(response, result = {}) {
  if (response.status === 429 || result.code === 429) {
    const raw = response.headers?.get('Retry-After');
    const seconds = /^\d+$/.test(raw || '') ? Number(raw) : raw ? Math.ceil((Date.parse(raw) - Date.now()) / 1000) : 0;
    return `操作过于频繁，请${seconds > 0 ? ` ${seconds} 秒后` : '稍后'}重试`;
  }
  if (response.status === 401 || result.code === 401) return result.message || '用户名或密码错误';
  return result.message || '请求失败，请稍后重试';
}
