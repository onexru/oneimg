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
 row:=models.APICredential{ID:LegacyTokenID(setting),OwnerID:models.SuperAdminID,Scopes:DefaultTokenScopes,ExpiresAt:time.Now().Add(TokenLifetime),Legacy:true}
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
 if token.Revoked || !time.Now().Before(token.ExpiresAt) || token.OwnerID<=0 { return token,ErrToken }
 now:=time.Now()
 result:=db.Model(&models.APICredential{}).Where("id = ? AND revoked = ? AND expires_at > ?",token.ID,false,now).Update("last_used_at",now)
 if result.Error!=nil { return token,result.Error }; if result.RowsAffected!=1 { return token,ErrToken }
 token.LastUsedAt=&now
 return token,nil
}
func HasScope(token models.APICredential, scope string) bool {
 for _,s:=range strings.Split(token.Scopes,",") { if s==scope { return true } }; return false
}
func NewAPICredential(db *gorm.DB,owner int) (string,models.APICredential,error) {
 raw,err:=RandomSecret(32); if err!=nil { return "",models.APICredential{},err }; raw="oi_"+raw
 row:=models.APICredential{ID:Digest(raw),OwnerID:owner,Scopes:DefaultTokenScopes,ExpiresAt:time.Now().Add(TokenLifetime)}
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
