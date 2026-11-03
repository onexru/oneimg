// 上传前的客户端压缩。
// 后端没有图像编码能力（只做存储），所以质量档位只能在浏览器端生效：
// 用 canvas 重编码为 WebP/JPEG，再交给原有上传流程。
// 原则：宁可放弃压缩，也不能让文件损坏或明显变大。

// 这些格式没有可靠的浏览器解码路径，直接原样上传
const SKIP_TYPES = new Set([
  'image/gif',      // 动图重编码会丢帧
  'image/svg+xml',  // 矢量图重编码会栅格化
  'image/avif',
  'image/heic',
  'image/heif',
])

// 小于这个体积的图压缩收益很低，反而可能因为重编码变大
const MIN_COMPRESS_BYTES = 80 * 1024

// 质量 100 视为「原图」：不重编码，避免无谓的画质与元数据损失
export const QUALITY_MAX = 100

export function loadImageElement(file) {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file)
    const img = new Image()
    img.onload = () => { URL.revokeObjectURL(url); resolve(img) }
    img.onerror = () => { URL.revokeObjectURL(url); reject(new Error('无法读取图片')) }
    img.src = url
  })
}

function canvasToBlob(canvas, type, quality) {
  return new Promise((resolve) => canvas.toBlob(resolve, type, quality))
}

/**
 * 按质量档位压缩单张图片。
 * @param {File} file 原始文件
 * @param {number} quality 1-100，100 直接返回原文件
 * @returns {Promise<File>} 压缩后的文件；任何异常都回退为原文件
 */
export async function compressImage(file, quality) {
  if (!file || quality >= QUALITY_MAX) return file
  if (!file.type.startsWith('image/')) return file
  if (SKIP_TYPES.has(file.type)) return file
  if (file.size < MIN_COMPRESS_BYTES) return file

  try {
    const img = await loadImageElement(file)
    const canvas = document.createElement('canvas')
    canvas.width = img.naturalWidth
    canvas.height = img.naturalHeight
    if (!canvas.width || !canvas.height) return file

    const ctx = canvas.getContext('2d')
    if (!ctx) return file
    // JPEG 没有透明通道，铺白底避免透明区域变黑
    ctx.fillStyle = '#ffffff'
    ctx.fillRect(0, 0, canvas.width, canvas.height)
    ctx.drawImage(img, 0, 0)

    // 有透明通道的格式尽量保留 WebP，否则用 JPEG
    const hasAlpha = /png|webp/i.test(file.type)
    const outType = hasAlpha ? 'image/webp' : 'image/jpeg'
    const q = Math.min(1, Math.max(0.01, quality / 100))

    let blob = await canvasToBlob(canvas, outType, q)
    // 高质量档位下 WebP 可能比原图还大，换 JPEG 再试一次；
    // 仍然更大才放弃压缩，避免出现「调了质量却没变化」的困惑。
    if ((!blob || blob.size >= file.size) && outType === 'image/webp') {
      const alt = await canvasToBlob(canvas, 'image/jpeg', q)
      if (alt && (!blob || alt.size < blob.size)) blob = alt
    }
    if (!blob || blob.size >= file.size) return file

    const ext = blob.type === 'image/webp' ? '.webp' : '.jpg'
    const baseName = (file.name || 'image').replace(/\.[^.]+$/, '')
    return new File([blob], `${baseName}${ext}`, { type: blob.type || outType, lastModified: file.lastModified })
  } catch {
    return file
  }
}

/**
 * 批量压缩，保持顺序。逐个处理而不是并发，
 * 避免同时解码多个大图把手机端内存打爆。
 */
export async function compressImages(files, quality) {
  const out = []
  for (const file of files) {
    out.push(await compressImage(file, quality))
  }
  return out
}
