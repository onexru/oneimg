package controllers

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
 "oneimg/backend/utils/securestorage"
 "oneimg/backend/utils/watermark"
 "github.com/gin-gonic/gin"
)

const maxProcessedObjectBytes int64 = 32 << 20
func unsafeImageExtension(path string)bool {ext:=strings.ToLower(filepath.Ext(path));return ext==".svg" || ext==".svgz"}
var encryptedReadSlots = make(chan struct{}, 2)

func isHistoricalSVG(image models.Image, objectPath string) bool {
 return strings.EqualFold(strings.SplitN(image.MimeType, ";", 2)[0], "image/svg+xml") || unsafeImageExtension(objectPath)
}
func imageResponseHeaders(c *gin.Context, mimeType, storage string, unsafe bool) {
 c.Header("Content-Type", mimeType)
 c.Header("X-Content-Type-Options", "nosniff")
 c.Header("Cache-Control", "public, max-age=31536000")
 c.Header("X-Storage-Type", storage)
 c.Header("Access-Control-Allow-Origin", "*")
 if unsafe {
  c.Header("Content-Security-Policy", "sandbox; default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
  c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename":filepath.Base(c.Request.URL.Path)}))
  c.Header("Cache-Control", "private, no-store")
 }
}
func imagePayloadType(prefix []byte, declared string, path string) (string, bool) {
 actual := http.DetectContentType(prefix)
 // Safe raster magic only. Everything else, including historical SVG whose
 // metadata lies, is sandboxed and downloaded rather than rendered inline.
 raster := actual == "image/png" || actual == "image/jpeg" || actual == "image/gif" || actual == "image/webp"
 historicalSVG := strings.EqualFold(strings.SplitN(declared, ";", 2)[0], "image/svg+xml") || unsafeImageExtension(path)
 if raster && !historicalSVG { return actual, false }
 if historicalSVG { return "image/svg+xml", true }
 return "application/octet-stream", true
}

// Plain seekable objects are served without a whole-file allocation. Streaming
// drivers only spool to bounded temporary disk when Range needs random access.
// GCM objects authenticate in bounded memory before returning any plaintext.
func serveStoredImage(c *gin.Context, stored io.Reader, declared, storage string, wm watermark.WatermarkConfig) error {
 if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
  c.Header("Allow", "GET, HEAD"); c.AbortWithStatus(http.StatusMethodNotAllowed); return nil
 }
 reader := bufio.NewReaderSize(stored, 4096)
 prefix, err := reader.Peek(512)
 if err != nil && !errors.Is(err, io.EOF) { return err }
 var payload io.Reader = reader
 var seeker io.ReadSeeker
 if securestorage.IsEncrypted(prefix) {
  select { case encryptedReadSlots <- struct{}{}: case <-c.Request.Context().Done(): return c.Request.Context().Err() }
  defer func(){ <-encryptedReadSlots }()
  content, _, err := securestorage.ReadAllLimited(reader, maxProcessedObjectBytes+64)
  if err != nil { return err }
  payload = bytes.NewReader(content)
  seeker = payload.(io.ReadSeeker)
  prefix = content
  if len(prefix)>512 { prefix=prefix[:512] }
 }
 actual, unsafe := imagePayloadType(prefix, declared, c.Request.URL.Path)
 imageResponseHeaders(c, actual, storage, unsafe)
 needRandomAccess := c.Request.Header.Get("Range") != "" || c.Request.Method == http.MethodHead
 if wm.Enable && !unsafe {
  transformed, err := watermark.ProcessImageWithWatermark(payload, actual, wm)
  if err == nil {
   data, err := io.ReadAll(io.LimitReader(transformed, maxProcessedObjectBytes+1))
   if err != nil { return err }; if int64(len(data)) > maxProcessedObjectBytes { return securestorage.ErrObjectTooLarge }
   payload = bytes.NewReader(data); seeker=payload.(io.ReadSeeker)
   imageResponseHeaders(c, http.DetectContentType(data), storage, false)
  } else {
   // No silently corrupted/empty fallback after consuming a stream.
   return err
  }
 }
 if seeker != nil {
  http.ServeContent(c.Writer,c.Request,filepath.Base(c.Request.URL.Path),time.Time{},seeker)
  c.Writer.WriteHeaderNow()
  return nil
 }
 if needRandomAccess {
  if rs, ok := stored.(io.ReadSeeker); ok && !wm.Enable {
   // Re-read from the start: reader has already consumed the peeked prefix.
   if _, err := rs.Seek(0, io.SeekStart); err != nil { return err }
   http.ServeContent(c.Writer,c.Request,filepath.Base(c.Request.URL.Path),time.Time{},rs)
   c.Writer.WriteHeaderNow()
   return nil
  }
  f, err := os.CreateTemp("", "oneimg-range-*")
  if err != nil { return err }; defer os.Remove(f.Name()); defer f.Close()
  n,err:=io.Copy(f,io.LimitReader(payload,maxProcessedObjectBytes+1))
  if err!=nil { return err }; if n>maxProcessedObjectBytes { return securestorage.ErrObjectTooLarge }
  if _,err=f.Seek(0,io.SeekStart);err!=nil{return err}
  http.ServeContent(c.Writer,c.Request,filepath.Base(c.Request.URL.Path),time.Time{},f)
  c.Writer.WriteHeaderNow()
  return nil
 }
 c.Status(http.StatusOK)
 _,err=io.Copy(c.Writer,payload)
 return err
}
