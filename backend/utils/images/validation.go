package images

import (
 "mime"
 "path"
 "strings"
 "unicode"
)

// Limits apply even when an administrator configures an unbounded upload size.
const MaxUploadBytes int64 = 32 << 20
var processingSlots = make(chan struct{}, 2)

// AcquireProcessing bounds simultaneous raster allocations across legacy and
// direct uploads. Release after encoding, not merely after DecodeConfig.
func AcquireProcessing() func() {
 processingSlots <- struct{}{}
 return func() { <-processingSlots }
}
func UploadByteLimit(configured int64) int64 {
 if configured > 0 && configured < MaxUploadBytes { return configured }
 return MaxUploadBytes
}
func declaredImageMIME(value string) string {
 parsed, _, err := mime.ParseMediaType(value)
 if err != nil { return "" }
 parsed = strings.ToLower(parsed)
 // An absent/generic MIME is not evidence of an image type.
 if parsed == "application/octet-stream" { return "" }
 return parsed
}
func AllowedMIME(actual string, allowed []string) bool {
 if len(allowed)==0 || (len(allowed)==1 && strings.TrimSpace(allowed[0])=="") { return actual=="image/png" || actual=="image/jpeg" || actual=="image/gif" || actual=="image/webp" }
 for _, v := range allowed {
  if declaredImageMIME(strings.TrimSpace(v)) == actual { return true }
 }
 return false
}
func formatMIME(format string) string {
 return map[string]string{"png":"image/png", "jpeg":"image/jpeg", "gif":"image/gif", "webp":"image/webp"}[format]
}

// windowsReservedStems 是 Windows 设备名。"CON.png" 这类文件名在 Windows 宿主上
// 创建即失败，需要加前缀规避。
var windowsReservedStems = map[string]bool{
 "CON": true, "PRN": true, "AUX": true, "NUL": true,
 "COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
 "LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

const (
 // stemMaxRunes 限制单段长度：留出随机后缀与扩展名的空间，且不触发各存储的 key 上限。
 stemMaxRunes = 80
 // stemFallback 是文件名被清洗后为空时的兜底名，与历史行为保持一致。
 stemFallback = "image"
)

// keepStemRune 判断字符是否原样保留：字母、数字（含中文等非 ASCII 文字）、
// 以及对象 key 与 URL 中无需转义的 "-"、"_"、"."。
//
// 其余字符（空格、括号、引号、路径分隔符、URL 保留字符）统一收敛成 "-"，
// 既保证 key 与 URL 安全（不会出现 # ? % : 造成截断或转义歧义），也避免
// Markdown/HTML 里带空格的链接被解析截断。
func keepStemRune(r rune) bool {
 if r < 128 {
  return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.'
 }
 // 剥离 Unicode Cf（格式字符）类：包含 RTL 覆盖符 U+202E 等 bidi 欺骗字符，
 // 它们对路径安全无意义但可造成视觉欺骗；控制字符、行分隔符同样丢弃。
 if unicode.Is(unicode.Cf, r) || unicode.IsControl(r) || r == '\uFFFD' {
  return false
 }
 // 非 ASCII 空白（全角空格、不换行空格等）与 ASCII 空格一致处理。
 if unicode.IsSpace(r) {
  return false
 }
 return true
}

// safeFilenameStem 把客户端可控的文件名收敛成可安全落盘的 stem，同时尽量
// 保留用户可识别的原始名称。
//
// 历史实现只放行 [A-Za-z0-9-_] 与非 ASCII，等于把 "my photo 2026" 变成
// "myphoto2026"、"截图 2026-11-02 (1)" 变成 "截图2026-11-02-1"，开启
// “保存源文件名”后拿到的名字与源文件名已经对不上。现在改为：可读字符原样
// 保留，不可读字符折叠成单个 "-"，因此 "my photo 2026" → "my-photo-2026"。
func safeFilenameStem(stem string) string {
 // 客户端可能塞入完整路径或反斜杠路径，只取最后一段，避免 key 逃逸。
 if i := strings.LastIndexAny(stem, `/\`); i >= 0 {
  stem = stem[i+1:]
 }
 var out strings.Builder
 separator := false
 for _, r := range stem {
  if keepStemRune(r) {
   if separator && out.Len() > 0 {
    out.WriteByte('-')
   }
   separator = false
   out.WriteRune(r)
   continue
  }
  separator = true
 }
 name := strings.Trim(out.String(), "-._ ")
 // 按 rune 截断，避免切坏 UTF-8；再清一次尾部 "."
 if runes := []rune(name); len(runes) > stemMaxRunes {
  name = strings.Trim(string(runes[:stemMaxRunes]), "-._ ")
 }
 if name == "" {
  return stemFallback
 }
 head := name
 if i := strings.IndexByte(head, '.'); i >= 0 {
  head = head[:i]
 }
 if windowsReservedStems[strings.ToUpper(strings.TrimSpace(head))] {
  return "_" + name
 }
 return name
}

// OriginalStem 返回上传文件名的可读 stem（去除目录与扩展名、已消毒）。
// “保存源文件名”与 {filename} 魔法变量共用同一口径，避免两处命名规则漂移。
func OriginalStem(fileName string) string {
	base := strings.ReplaceAll(strings.TrimSpace(fileName), "\\", "/")
	base = path.Base(base)
	if base == "." || base == "/" || base == ".." {
		base = ""
	}
	ext := path.Ext(base)
	// ".hidden" 的 path.Ext 返回整个名字，它不是扩展名而是无扩展名的隐藏文件。
	if ext == base {
		ext = ""
	}
	return safeFilenameStem(strings.TrimSuffix(base, ext))
}
