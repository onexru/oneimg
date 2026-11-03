package images

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"oneimg/backend/models"
	"oneimg/backend/utils/result"
	"oneimg/backend/utils/securestorage"
	"oneimg/backend/utils/watermark"

	"github.com/gin-gonic/gin"
)

const maxProcessedObjectBytes int64 = 32 << 20

func unsafeImageExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".svg" || ext == ".svgz"
}

var encryptedReadSlots = make(chan struct{}, 2)

func isHistoricalSVG(image models.Image, objectPath string) bool {
	return strings.EqualFold(strings.SplitN(image.MimeType, ";", 2)[0], "image/svg+xml") || unsafeImageExtension(objectPath)
}

func imageResponseHeaders(c *gin.Context, mimeType, storage string, unsafe bool) {
	c.Header("Content-Type", mimeType)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "public, max-age=31536000")
	c.Header("Access-Control-Allow-Origin", "*")
	// 不再回 X-Storage-Type：它把「这张图存在哪个后端」告诉任何访客，
	// 对排障没多大用，却方便攻击者识别部署形态（本地/S3/R2/Telegram）。
	if unsafe {
		c.Header("Content-Security-Policy", "sandbox; default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filepath.Base(c.Request.URL.Path)}))
		c.Header("Cache-Control", "private, no-store")
	}
}

// applyTranscodedCacheHeaders 覆盖 imageResponseHeaders 设过的缓存策略。
//
// 为什么必须覆盖：转码结果的字节**取决于 query 参数**（wm_text/wm_size/wm_pos…），
// 但原实现给它打的是 `Cache-Control: public, max-age=31536000` 且没有 Vary。
// 于是攻击者请求一次
//
//	/uploads/x.webp?watermark=true&wm_text=广告内容
//
// 前置 CDN / 反代会把这**一张带攻击者文字的图**缓存到该 URL 上，长达一年；
// 之后所有正常访客（哪怕不带任何参数）看到的都是被篡改的图。
// 这比 DoS 更隐蔽 —— 站点看起来像是被劫持了。
//
// 处理：转码结果一律 private。
//
// 不能只靠 Vary + 短 max-age：不少 CDN/反代默认「忽略 query，只按路径做缓存键」
// （常见于只缓存静态后缀的配置），那样无论 Vary 写什么都会串味。
// private 让中间层根本不缓存，从根上消除污染；代价只是转码图失去 CDN 加速，
// 而转码本来就该是低频操作（也已被限流到每 IP 6 次/10 秒）。
func applyTranscodedCacheHeaders(c *gin.Context) {
	c.Header("Vary", "Accept, Accept-Encoding")
	c.Header("Cache-Control", "private, max-age=600, no-transform")
}
func imagePayloadType(prefix []byte, declared string, path string) (string, bool) {
	actual := http.DetectContentType(prefix)
	// Safe raster magic only. Everything else, including historical SVG whose
	// metadata lies, is sandboxed and downloaded rather than rendered inline.
	raster := actual == "image/png" || actual == "image/jpeg" || actual == "image/gif" || actual == "image/webp"
	historicalSVG := strings.EqualFold(strings.SplitN(declared, ";", 2)[0], "image/svg+xml") || unsafeImageExtension(path)
	if raster && !historicalSVG {
		return actual, false
	}
	if historicalSVG {
		return "image/svg+xml", true
	}
	return "application/octet-stream", true
}

// Plain seekable objects are served without a whole-file allocation. Streaming
// drivers only spool to bounded temporary disk when Range needs random access.
// GCM objects authenticate in bounded memory before returning any plaintext.
//
// cacheKey 非空时启用转码结果缓存：转码成功就把结果存进去，命中的请求
// 连 driver.Get 都不需要（省掉一次回源）。空字符串表示这次请求不缓存。
func serveStoredImage(c *gin.Context, stored io.Reader, declared, storage string, wm watermark.WatermarkConfig, cacheKey string) error {
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
		c.Header("Allow", "GET, HEAD")
		c.AbortWithStatus(http.StatusMethodNotAllowed)
		return nil
	}
	reader := bufio.NewReaderSize(stored, 4096)
	prefix, err := reader.Peek(512)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	var payload io.Reader = reader
	var seeker io.ReadSeeker
	if securestorage.IsEncrypted(prefix) {
		select {
		case encryptedReadSlots <- struct{}{}:
		case <-c.Request.Context().Done():
			return c.Request.Context().Err()
		}
		defer func() { <-encryptedReadSlots }()
		content, _, err := securestorage.ReadAllLimited(reader, maxProcessedObjectBytes+64)
		if err != nil {
			return err
		}
		payload = bytes.NewReader(content)
		seeker = payload.(io.ReadSeeker)
		prefix = content
		if len(prefix) > 512 {
			prefix = prefix[:512]
		}
	}
	actual, unsafe := imagePayloadType(prefix, declared, c.Request.URL.Path)
	imageResponseHeaders(c, actual, storage, unsafe)
	needRandomAccess := c.Request.Header.Get("Range") != "" || c.Request.Method == http.MethodHead
	// HEAD 直接跳过转码。
	//
	// 转码是纯 CPU 的重活，而 HEAD 的语义只需要「这个尺寸/类型是多少」——
	// 响应头在校验出 actual 之后就已经齐了，正文本来也不会发出去。
	// 原实现放行 HEAD 并照常转码，等于给攻击者一个「零回包带宽、满额计算」的
	// 杠杆：每个 HEAD 都能触发一次 32MB 读 + 16MP 解码 + 重编码，却不需要接收结果。
	// 这也是 M1 攻击手册里判定的「最优攻击形态」。
	// transcoded 标记 payload 是否已是「转码后的内存缓冲」。
	// 用它而不是 wm.Enable 来决定能否走 ReadSeeker 快捷路径：
	// HEAD 在 wm.Enable 为真时也不会转码（见上面），此时 payload 仍是原始流，
	// 若继续用 wm.Enable 判断就会白白落一次临时盘。
	transcoded := false
	if wm.Enable && !unsafe && c.Request.Method != http.MethodHead {
		transformed, err := watermark.ProcessImageWithWatermarkContext(c.Request.Context(), payload, actual, wm)
		if err == nil {
			data, err := io.ReadAll(io.LimitReader(transformed, maxProcessedObjectBytes+1))
			if err != nil {
				return err
			}
			if int64(len(data)) > maxProcessedObjectBytes {
				return securestorage.ErrObjectTooLarge
			}
			payload = bytes.NewReader(data)
			seeker = payload.(io.ReadSeeker)
			transcoded = true
			// 转码成功才写缓存。失败不缓存：那可能只是槽位超时或参数越界，
			// 把失败也记住会让一次偶发错误在 TTL 内持续复现。
			if cacheKey != "" {
				transcodedCache.put(cacheKey, data, http.DetectContentType(data))
			}
			imageResponseHeaders(c, http.DetectContentType(data), storage, false)
			// 转码结果与参数强相关，必须换成不会被污染的缓存策略。
			applyTranscodedCacheHeaders(c)
		} else {
			// 槽位满/排队超时：明确回 503 + Retry-After，而不是让它继续排队。
			// 排队自增的请求在客户端断开后仍会被执行，正是放大攻击的燃料。
			if errors.Is(err, watermark.ErrProcessingBusy) {
				c.Header("Retry-After", "2")
				c.JSON(http.StatusServiceUnavailable, result.Error(http.StatusServiceUnavailable, "图片处理繁忙，请稍后重试"))
				return nil
			}
			// No silently corrupted/empty fallback after consuming a stream.
			return err
		}
	}
	if seeker != nil {
		http.ServeContent(c.Writer, c.Request, filepath.Base(c.Request.URL.Path), time.Time{}, seeker)
		c.Writer.WriteHeaderNow()
		return nil
	}
	if needRandomAccess {
		if rs, ok := stored.(io.ReadSeeker); ok && !transcoded {
			// Re-read from the start: reader has already consumed the peeked prefix.
			if _, err := rs.Seek(0, io.SeekStart); err != nil {
				return err
			}
			http.ServeContent(c.Writer, c.Request, filepath.Base(c.Request.URL.Path), time.Time{}, rs)
			c.Writer.WriteHeaderNow()
			return nil
		}
		f, err := os.CreateTemp("", "oneimg-range-*")
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		defer f.Close()
		n, err := io.Copy(f, io.LimitReader(payload, maxProcessedObjectBytes+1))
		if err != nil {
			return err
		}
		if n > maxProcessedObjectBytes {
			return securestorage.ErrObjectTooLarge
		}
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			return err
		}
		http.ServeContent(c.Writer, c.Request, filepath.Base(c.Request.URL.Path), time.Time{}, f)
		c.Writer.WriteHeaderNow()
		return nil
	}
	c.Status(http.StatusOK)
	_, err = io.Copy(c.Writer, payload)
	return err
}
