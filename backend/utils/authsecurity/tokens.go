package authsecurity

import (
 "errors"
 "strings"
 "time"
 "oneimg/backend/models"
 "oneimg/backend/utils/secureconfig"
 "gorm.io/gorm"
 "gorm.io/gorm/clause"
)

var ErrToken = errors.New("API credential invalid or expired")
func LegacyTokenID(setting models.Settings) string {
 // Stable even before the legacy plaintext-to-bcrypt migration: only callers
 // seeing the active stored hash enroll it. No raw legacy secret is persisted.
 return Digest("legacy:"+setting.APITokenHash+":"+setting.APIToken)
}
func EnrollLegacyToken(db *gorm.DB, setting models.Settings) error {
 if setting.APITokenHash=="" && setting.APIToken=="" { return nil }
 row:=models.APICredential{ID:LegacyTokenID(setting),OwnerID:models.SuperAdminID,Scopes:DefaultTokenScopes,ExpiresAt:time.Now().Add(TokenLifetime),ExpirySpec:FormatTokenExpiry(TokenLifetime),Legacy:true}
 return db.Clauses(clause.OnConflict{DoNothing:true}).Create(&row).Error
}
func ValidateAPICredential(db *gorm.DB, setting models.Settings, raw string) (models.APICredential,error) {
 var token models.APICredential
 if !setting.StartAPI || raw=="" || len(raw)>256 { return token,ErrToken }
 err:=db.Where("id = ? AND legacy = ?",Digest(raw),false).First(&token).Error
 if errors.Is(err,gorm.ErrRecordNotFound) {
  if !secureconfig.CompareSecretHash(setting.APITokenHash,raw) && !(setting.APIToken!="" && secureconfig.ConstantTimeEqual(setting.APIToken,raw)) { return token,ErrToken }
  // Enrolled at startup (after secrets migration), never by GET or by
  // presenting an old credential. Expired tokens cannot extend themselves.
  err=db.First(&token,"id = ?",LegacyTokenID(setting)).Error
 }
 if err!=nil { return token,err }
 now:=time.Now()
 if !token.Usable(now) { return token,ErrToken }
 result:=db.Model(&models.APICredential{}).Where("id = ? AND revoked = ? AND (never_expires = ? OR expires_at > ?)",token.ID,false,true,now).Update("last_used_at",now)
 if result.Error!=nil { return token,result.Error }; if result.RowsAffected!=1 { return token,ErrToken }
 token.LastUsedAt=&now
 return token,nil
}
func HasScope(token models.APICredential, scope string) bool {
 for _,s:=range strings.Split(token.Scopes,",") { if s==scope { return true } }; return false
}

// TokenScopes 是 API Token 支持的 scope 全集，命名与用户权限码一致。
// 只有列在这里的值会被当成有效权限：即使数据库里出现脏字符串（例如 "*"），
// 也不会进入控制器的权限判断，避免 token 意外拿到超管级的 "*" 判定。
var TokenScopes = []string{"image:read", "upload:write", "image:delete"}

// PermissionCodes 把 token 的 scopes 还原成权限码列表。
//
// 中间件按 scope 判断路由是否可达，控制器按权限码判权（图片删除就是
// image:delete）。两者必须同源，否则会出现「路由放行了、控制器却仍 403」
// 或者反过来「控制器允许、路由先拦住」的两头不一致。
func PermissionCodes(scopes string) []string {
 allowed := make(map[string]struct{}, len(TokenScopes))
 for _, scope := range TokenScopes { allowed[scope] = struct{}{} }
 codes := make([]string, 0, len(TokenScopes))
 seen := make(map[string]struct{}, len(TokenScopes))
 for _, raw := range strings.Split(scopes, ",") {
  scope := strings.TrimSpace(raw)
  if _, ok := allowed[scope]; !ok { continue }
  if _, dup := seen[scope]; dup { continue }
  seen[scope] = struct{}{}
  codes = append(codes, scope)
 }
 return codes
}
func NewAPICredential(db *gorm.DB,owner int,expiry TokenExpiry) (string,models.APICredential,error) {
 raw,err:=RandomSecret(32); if err!=nil { return "",models.APICredential{},err }; raw="oi_"+raw
 now:=time.Now()
 row:=models.APICredential{ID:Digest(raw),OwnerID:owner,Scopes:DefaultTokenScopes,ExpiresAt:expiry.ExpiresAt(now),NeverExpires:expiry.Permanent,ExpirySpec:expiry.Spec}
 err=db.Transaction(func(tx *gorm.DB) error {
  // Serialise regenerations. Concurrent requests must not leave two active
  // credentials. This also interlocks with global session revocation.
  var state models.AuthState
  if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&state,1).Error; err!=nil { return err }
  if err:=tx.Model(&models.APICredential{}).Where("revoked = ?",false).Update("revoked",true).Error; err!=nil { return err }
  if err:=tx.Model(&models.Settings{}).Where("id = ?",1).Updates(map[string]any{"api_token":"","api_token_hash":""}).Error; err!=nil { return err }
  return tx.Create(&row).Error
 })
 return raw,row,err
}

// ErrNoActiveCredential 表示目标用户当前没有可修改有效期的凭据。
var ErrNoActiveCredential = errors.New("没有可更新的 API 凭据")

// ErrCredentialExpired 表示凭据已经过期：此时不允许延长有效期，否则一个
// 本该失效的凭据会因为一次「改配置」重新生效，等于绕过撤销语义。
var ErrCredentialExpired = errors.New("API 凭据已过期，请重新生成")

// UpdateAPICredentialExpiry 修改用户当前生效凭据的有效期配置。
//
// 「现有 API KEY」在数据模型里就是该用户唯一未撤销的那一条，因此这里按
// owner 取最近创建的一条，而不是让前端传 digest —— digest 从不对外暴露。
// 更新只动有效期三件套（expires_at / never_expires / expiry_spec），
// 凭据原文、scopes、last_used_at 与创建时间保持不变，调用方无需更换 Token。
func UpdateAPICredentialExpiry(db *gorm.DB,owner int,expiry TokenExpiry) (models.APICredential,error) {
 var token models.APICredential
 err:=db.Transaction(func(tx *gorm.DB) error {
  // 与重新生成共用同一把锁：避免「一边改有效期、一边重新生成」时把新凭据
  // 的配置覆盖成旧凭据的选择。
  var state models.AuthState
  if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&state,1).Error; err!=nil { return err }
  err:=tx.Where("owner_id = ? AND revoked = ?",owner,false).Order("created_at desc").First(&token).Error
  if errors.Is(err,gorm.ErrRecordNotFound) { return ErrNoActiveCredential }
  if err!=nil { return err }
  now:=time.Now()
  if !token.Usable(now) { return ErrCredentialExpired }
  updates:=map[string]any{"expires_at":expiry.ExpiresAt(now),"never_expires":expiry.Permanent,"expiry_spec":expiry.Spec}
  if err:=tx.Model(&models.APICredential{}).Where("id = ?",token.ID).Updates(updates).Error; err!=nil { return err }
  token.ExpiresAt=expiry.ExpiresAt(now); token.NeverExpires=expiry.Permanent; token.ExpirySpec=expiry.Spec
  return nil
 })
 if err!=nil { return models.APICredential{},err }
 return token,nil
}
