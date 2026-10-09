package images

import (
 "mime"
 "strings"
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
func safeFilenameStem(stem string) string {
 var out strings.Builder
 for _, r := range stem {
  if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r > 127 { out.WriteRune(r) }
  if out.Len() >= 120 { break }
 }
 if out.Len() == 0 { return "image" }
 return out.String()
}
