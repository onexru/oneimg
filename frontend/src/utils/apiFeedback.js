/** Respect the HTTP status and the OneImg {code,message,data} envelope together. */
export function apiMessage(status, result, fallback = '请求失败，请重试') {
  const message = typeof result?.message === 'string' ? result.message.trim() : '';
  if (message) return message;
  return ({ 401: '登录已过期，请重新登录', 403: '没有权限执行此操作，请联系管理员',
    413: '请求或文件超过服务器限制', 415: '不支持此图片格式，SVG 不允许上传',
    429: '请求过于频繁，请稍后重试', 503: '服务暂时不可用，请稍后重试' })[status] || fallback;
}
export async function readApiResponse(response, fallback) {
  let result;
  try { result = await response.json(); } catch { result = {}; }
  if (!response.ok || result?.code !== 200) {
    const error = new Error(apiMessage(response.status, result, response.ok && !result?.code ? '服务器响应格式无效，请重试' : fallback));
    error.status = response.status;
    error.code = result?.error_code || result?.code;
    error.error_code = result?.error_code;
    error.data = result?.data;
    error.retryAfter = response.headers?.get('Retry-After') || '';
    throw error;
  }
  return result;
}
