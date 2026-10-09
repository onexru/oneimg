package controllers
import (
 "errors"
 "net/url"
 "strings"
 "oneimg/backend/config"
 "github.com/gin-gonic/gin"
)
// 辅助函数，校验来源
func checkReferer(referer, whiteList, selfDomain string) bool {
 return checkRefererPolicy(referer, whiteList, selfDomain, true)
}
func checkRefererPolicy(referer, whiteList, selfDomain string, allowEmpty bool) bool {
 if strings.TrimSpace(referer) == "" { return allowEmpty }
 domain, err := extractDomainFromReferer(referer)
 if err != nil { return false }
 allowed := append(strings.Split(whiteList, ","), selfDomain)
 for _, raw := range allowed {
  host := strings.ToLower(strings.TrimSpace(raw))
  if host == "" || strings.ContainsAny(host, "/:@?# 	\r\n") { continue }
  if domain == host || strings.HasSuffix(domain, "."+host) { return true }
 }
 return false
}
func extractDomainFromReferer(referer string) (string, error) {
 u, err := url.Parse(referer)
 if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.Hostname() == "" { return "", errors.New("invalid referer") }
 return strings.ToLower(u.Hostname()), nil
}
// Never derive a trust boundary from client-supplied Host/Forwarded headers.
func GetSelfDomain(c *gin.Context) string {
 if config.App == nil { return "" }
 host, err := extractDomainFromReferer(strings.TrimSpace(config.App.AppURL))
 if err != nil { return "" }
 return host
}
