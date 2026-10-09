package middlewares

import (
 "net/http"
 "strings"
 "github.com/gin-gonic/gin"
)
// Authentication JSON is bounded by the limiter (32 KiB). Other API JSON is
// bounded separately; multipart uploads retain their request-specific limits.
func AuthBodyLimitMiddleware(size int64) gin.HandlerFunc {
 return func(c *gin.Context) {
  if strings.HasPrefix(c.Request.URL.Path,"/api/") && strings.HasPrefix(strings.ToLower(c.ContentType()),"application/json") {
   if c.Request.ContentLength>size { rejectAuth(c,http.StatusRequestEntityTooLarge,"请求内容过大");return }
   c.Request.Body=http.MaxBytesReader(c.Writer,c.Request.Body,size)
  }
  c.Next()
 }
}
