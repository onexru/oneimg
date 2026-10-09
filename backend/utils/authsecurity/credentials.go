package authsecurity

import (
 "crypto/rand"
 "crypto/sha256"
 "encoding/base64"
 "encoding/hex"
 "fmt"
 "time"
 "oneimg/backend/models"
 "gorm.io/gorm"
 "gorm.io/gorm/clause"
)

const GuestCookie = "oneimg-guest"
const GuestLifetime = 365 * 24 * time.Hour
const TokenLifetime = 30 * 24 * time.Hour
const DefaultTokenScopes = "image:read,upload:write"

func RandomSecret(size int) (string, error) {
 b := make([]byte, size)
 if _, err := rand.Read(b); err != nil { return "", err }
 return base64.RawURLEncoding.EncodeToString(b), nil
}
func Digest(raw string) string {
 s := sha256.Sum256([]byte(raw)); return hex.EncodeToString(s[:])
}
func NewOwnerKey() (string, error) {
 b := make([]byte,16)
 if _,err := rand.Read(b); err != nil { return "",err }
 b[6]=(b[6]&0x0f)|0x40; b[8]=(b[8]&0x3f)|0x80
 return fmt.Sprintf("%x-%x-%x-%x-%x",b[:4],b[4:6],b[6:8],b[8:10],b[10:]),nil
}

// RecoverGuest never consults a fingerprint or a supplied public owner key.
func RecoverGuest(db *gorm.DB, credential string) (models.GuestIdentity, string, bool, error) {
 var g models.GuestIdentity
 if len(credential)==43 {
  err:=db.Where("credential_hash = ? AND expires_at > ?",Digest(credential),time.Now()).First(&g).Error
  if err==nil {
   expiry:=time.Now().Add(GuestLifetime)
   err=db.Model(&g).Update("expires_at",expiry).Error
   return g,credential,true,err
  }
  if err!=gorm.ErrRecordNotFound { return g,"",false,err }
 }
 raw,err:=RandomSecret(32); if err!=nil { return g,"",false,err }
 owner,err:=NewOwnerKey(); if err!=nil { return g,"",false,err }
 g=models.GuestIdentity{OwnerKey:owner,CredentialHash:Digest(raw),ExpiresAt:time.Now().Add(GuestLifetime)}
 err=db.Create(&g).Error
 return g,raw,false,err
}

func EnsureState(db *gorm.DB) error {
 return db.Clauses(clause.OnConflict{DoNothing:true}).Create(&models.AuthState{ID:1}).Error
}
func RevokeUserSessions(tx *gorm.DB,id int) error {
 // Increment in the same transaction as credential changes. Old logins cannot
 // create a new valid session with an earlier password after this commits.
 if err:=tx.Model(&models.User{}).Where("id = ?",id).UpdateColumn("auth_version",gorm.Expr("auth_version + 1")).Error; err!=nil { return err }
 return tx.Where("user_id = ?",id).Delete(&models.AuthSession{}).Error
}
func RevokeAllSessions(tx *gorm.DB) error {
 if err:=tx.Model(&models.AuthState{}).Where("id = ?",1).UpdateColumn("epoch",gorm.Expr("epoch + 1")).Error; err!=nil { return err }
 return tx.Where("1 = 1").Delete(&models.AuthSession{}).Error
}
