package images

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"

	"oneimg/backend/utils/watermark"
)

// 转码结果缓存。
//
// 为什么需要：一次转码是 io.ReadAll(≤32MB) + image.Decode(≤16MP) + 画字 +
// 重编码，实测一张 4335×2520 的图要 0.57s，而不转码只要 0.03s —— 相差 17 倍。
// 但同一张图 + 同一组水印参数的输出**永远相同**，重复请求没有任何理由重算。
//
// 典型场景就是站点自己的画廊页：同一张图被多个访客反复打开，每次都触发
// 一次全量转码，两个转码槽很快被打满，正常访问开始排队甚至 503。
//
// 缓存键 = 图片标识 + 全部影响输出的参数。参数漏一个就会串味（比如把
// wm_text 漏掉，所有文字共用一个缓存条目，用户看到别人的水印文字），
// 所以这里用结构体的全部字段拼键，而不是「挑几个常用的」。

// transcodedCacheTTL 是单条缓存的有效期。
//
// 设成 10 分钟而不是几小时：管理员改水印参数（颜色/字体/默认位置）后，
// 旧图最多 10 分钟就会重新生成。再长会让「改了没生效」变成需要重启才能解释的现象。
const transcodedCacheTTL = 10 * time.Minute

// transcodedCacheMaxBytes 是缓存总量上限，也是内存安全的边界。
//
// 单个转码结果可能有几 MB（大图重编码后往往比原图更大），无上限缓存
// 等于把攻击者可控的内存分配装进一个长期驻留的 map —— 换个 DoS 而已。
const transcodedCacheMaxBytes = 96 << 20

type transcodedEntry struct {
	data     []byte
	mimeType string
	expires  time.Time
	lastUsed time.Time
}

type transcodedCacheStore struct {
	mu      sync.Mutex
	entries map[string]*transcodedEntry
	bytes   int64
	now     func() time.Time
	// MaxBytes 可在测试里调小，无需真的分配 96MB。
	MaxBytes int64
}

var transcodedCache = &transcodedCacheStore{
	entries:  make(map[string]*transcodedEntry),
	now:      time.Now,
	MaxBytes: transcodedCacheMaxBytes,
}

// transcodedCacheKey 由「原图标识」和「水印参数」共同决定。
//
// objectKey 用存储引用而不是请求 URL：同一张图可能通过原图 URL 和缩略图 URL
// 两条路径访问，它们的内容不同，必须分开缓存（ref 里已经带了这个区分）。
func transcodedCacheKey(objectKey string, wm watermark.WatermarkConfig) string {
	h := sha256.New()
	// 用 NUL 分隔各字段：否则 ("ab","c") 和 ("a","bc") 会拼成同一个键。
	write := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
	}
	write(objectKey)
	write(wm.Text)
	write(wm.Position)
	write(strconv.Itoa(wm.FontSize))
	write(strconv.FormatFloat(wm.FontSizeRatio, 'f', -1, 64))
	write(strconv.Itoa(wm.MinFontSize))
	write(strconv.Itoa(wm.MaxFontSize))
	write(wm.FontColor)
	write(strconv.FormatFloat(wm.Opacity, 'f', -1, 64))
	write(wm.FontPath)
	write(strconv.FormatBool(wm.EnableDynamicSize))
	return hex.EncodeToString(h.Sum(nil))
}

// get 返回命中项。返回的是副本引用，调用方只读不改。
func (s *transcodedCacheStore) get(key string) (*transcodedEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[key]
	if !ok {
		return nil, false
	}
	now := s.now()
	if now.After(entry.expires) {
		s.deleteLocked(key)
		return nil, false
	}
	entry.lastUsed = now
	return entry, true
}

// put 写入一条缓存。超过总量时按最久未使用淘汰。
func (s *transcodedCacheStore) put(key string, data []byte, mimeType string) {
	// 单条就超过总预算的记录直接不缓存：否则会先把别的全清空，
	// 自己又因为超限被立刻丢掉，白白制造一次全表清洗。
	if int64(len(data)) > s.MaxBytes {
		return
	}
	copied := make([]byte, len(data))
	copy(copied, data)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if old, ok := s.entries[key]; ok {
		s.bytes -= int64(len(old.data))
		delete(s.entries, key)
	}
	// 先清过期项，再看要不要按 LRU 淘汰 —— 过期项通常占大头，
	// 先清它们能避免误伤还在有效期内的热数据。
	if s.bytes+int64(len(copied)) > s.MaxBytes {
		s.evictExpiredLocked(now)
	}
	for s.bytes+int64(len(copied)) > s.MaxBytes && len(s.entries) > 0 {
		s.evictOldestLocked()
	}
	if s.bytes+int64(len(copied)) > s.MaxBytes {
		return
	}
	s.entries[key] = &transcodedEntry{data: copied, mimeType: mimeType, expires: now.Add(transcodedCacheTTL), lastUsed: now}
	s.bytes += int64(len(copied))
}

func (s *transcodedCacheStore) deleteLocked(key string) {
	if entry, ok := s.entries[key]; ok {
		s.bytes -= int64(len(entry.data))
		delete(s.entries, key)
	}
}

func (s *transcodedCacheStore) evictExpiredLocked(now time.Time) {
	for k, v := range s.entries {
		if now.After(v.expires) {
			s.bytes -= int64(len(v.data))
			delete(s.entries, k)
		}
	}
}

func (s *transcodedCacheStore) evictOldestLocked() {
	var oldestKey string
	var oldest time.Time
	first := true
	for k, v := range s.entries {
		if first || v.lastUsed.Before(oldest) {
			oldestKey, oldest, first = k, v.lastUsed, false
		}
	}
	if oldestKey != "" {
		s.deleteLocked(oldestKey)
	}
}

// clear 用于测试与「管理员改了全局水印参数」时的主动失效。
func (s *transcodedCacheStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = make(map[string]*transcodedEntry)
	s.bytes = 0
}

// stats 供测试断言容量边界与命中行为。
func (s *transcodedCacheStore) stats() (count int, bytes int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries), s.bytes
}

// InvalidateTranscodedCache 在全局水印参数变化时清空缓存。
//
// 为什么需要：缓存键里已经包含水印参数，改参数后新请求自然算新键、不会读到旧图。
// 但旧键会一直占到过期（10 分钟），既浪费内存，也会让「改回原参数」时
// 拿到十分钟前的旧结果 —— 管理员调试样式时会反复改来改去，这个行为很别扭。
func InvalidateTranscodedCache() {
	transcodedCache.clear()
}

// normalizedCacheObjectKey 把存储引用收敛成一个稳定字符串。
// ref 的 Path 已经是规范化后的对象键，Metadata 只在 Telegram 一类
// 用 file_id 寻址的后端上才有实质差异。
func normalizedCacheObjectKey(path string, metadata map[string]any) string {
	if len(metadata) == 0 {
		return path
	}
	// map 迭代顺序随机，必须排序后拼接，否则同一对象会算出不同的键、永远不命中。
	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	// 手写插入排序：元数据通常只有 1-3 个键，比引入 sort 更直观。
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	var b strings.Builder
	b.WriteString(path)
	for _, k := range keys {
		b.WriteString("\x00")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(toStringValue(metadata[k]))
	}
	return b.String()
}

func toStringValue(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(value)
	default:
		return ""
	}
}
