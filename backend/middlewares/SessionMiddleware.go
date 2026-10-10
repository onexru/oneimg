package middlewares

import (
 "encoding/json"
 "errors"
 "net/http"
 "strings"
 "sync"
 "time"
 "oneimg/backend/config"
 "oneimg/backend/database"
 "oneimg/backend/models"
 "oneimg/backend/utils/authsecurity"
 gs "github.com/gorilla/sessions"
 "github.com/gorilla/securecookie"
 "github.com/gin-contrib/sessions"
 "github.com/gin-gonic/gin"
 "gorm.io/gorm"
 "gorm.io/gorm/clause"
)

const SessionCookie = "oneimg-session"
var ErrSessionRevoked = errors.New("session revoked; sign in again")

type DBSessionStore struct {
 db *gorm.DB
 codecs []securecookie.Codec
 mu sync.RWMutex
 options gs.Options
}

type sessionPayload struct {
 UserID int `json:"user_id"`
 Role int `json:"role"`
 Username string `json:"username"`
 Version uint64 `json:"version"`
}

func NewDBSessionStore(db *gorm.DB, secret string, secure bool) *DBSessionStore {
 return &DBSessionStore{db:db,codecs:securecookie.CodecsFromPairs([]byte(secret)),
 options:gs.Options{Path:"/",MaxAge:86400,HttpOnly:true,Secure:secure,SameSite:http.SameSiteLaxMode}}
}
func (s *DBSessionStore) Options(o sessions.Options) {
 s.mu.Lock(); defer s.mu.Unlock(); s.options=*o.ToGorillaOptions()
}
func (s *DBSessionStore) Get(r *http.Request,name string) (*gs.Session,error) {
 return gs.GetRegistry(r).Get(s,name)
}
func (s *DBSessionStore) New(r *http.Request,name string) (*gs.Session,error) {
 sess:=gs.NewSession(s,name)
 s.mu.RLock(); opts:=s.options; s.mu.RUnlock(); sess.Options=&opts; sess.IsNew=true
 var state models.AuthState
 if err:=s.db.First(&state,1).Error; err!=nil { return sess,err }
 sess.Values["_epoch"]=state.Epoch
 cookie,err:=r.Cookie(name); if err!=nil { return sess,nil }
 var raw string
 if securecookie.DecodeMulti(name,cookie.Value,&raw,s.codecs...)!=nil || len(raw)!=43 { return sess,nil }
 var row models.AuthSession
 err=s.db.Where("id = ? AND expires_at > ? AND epoch = ?",authsecurity.Digest(raw),time.Now(),state.Epoch).First(&row).Error
 if errors.Is(err,gorm.ErrRecordNotFound) { return sess,nil }; if err!=nil { return sess,err }
 var p sessionPayload
 if json.Unmarshal(row.Values,&p)!=nil || p.UserID!=row.UserID || p.UserID==0 { return sess,nil }
 if p.UserID>0 {
  var u models.User
  if err=s.db.Select("id","auth_version").First(&u,p.UserID).Error; errors.Is(err,gorm.ErrRecordNotFound) { return sess,nil }
  if err!=nil { return sess,err }; if u.AuthVersion!=row.UserVersion { return sess,nil }
 } else {
  var g models.GuestIdentity
  if err=s.db.Where("id = ? AND expires_at > ?",-p.UserID,time.Now()).First(&g).Error; errors.Is(err,gorm.ErrRecordNotFound) { return sess,nil }
  if err!=nil { return sess,err }
  if g.OwnerKey!=p.Username || p.Role!=models.RoleGuest { return sess,nil }
 }
 sess.ID=raw; sess.IsNew=false
 sess.Values["user_id"]=p.UserID; sess.Values["user_role"]=p.Role
 sess.Values["username"]=p.Username; sess.Values["logged_in"]=true
 sess.Values["auth_version"]=p.Version
 return sess,nil
}
func (s *DBSessionStore) Save(r *http.Request,w http.ResponseWriter,sess *gs.Session) error {
 if sess.Options.MaxAge<0 || sess.Values["logged_in"]!=true {
  if sess.ID!="" { if err:=s.db.Where("id = ?",authsecurity.Digest(sess.ID)).Delete(&models.AuthSession{}).Error; err!=nil { return err } }
  opts:=*sess.Options; opts.MaxAge=-1
  http.SetCookie(w,gs.NewCookie(sess.Name(),"",&opts)); return nil
 }
 id,ok:=sess.Values["user_id"].(int); if !ok || id==0 { return ErrSessionRevoked }
 role,_:=sess.Values["user_role"].(int); username,_:=sess.Values["username"].(string)
 version,_:=sess.Values["auth_version"].(uint64); epoch,_:=sess.Values["_epoch"].(uint64)
 raw:=sess.ID
 rotate:=sess.Values["_rotate"]==true || raw==""
 if rotate { var err error; raw,err=authsecurity.RandomSecret(32); if err!=nil { return err } }
 payload,err:=json.Marshal(sessionPayload{UserID:id,Role:role,Username:username,Version:version}); if err!=nil { return err }
 encoded,err:=securecookie.EncodeMulti(sess.Name(),raw,s.codecs...); if err!=nil { return err }
 err=s.db.Transaction(func(tx *gorm.DB) error {
  var state models.AuthState
  if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).First(&state,1).Error; err!=nil { return err }
  if state.Epoch!=epoch { return ErrSessionRevoked }
  if id>0 {
   var user models.User
   if err:=tx.Clauses(clause.Locking{Strength:"UPDATE"}).Select("id","auth_version").First(&user,id).Error; err!=nil { return err }
   if user.AuthVersion!=version { return ErrSessionRevoked }
  }
  row:=models.AuthSession{ID:authsecurity.Digest(raw),UserID:id,UserVersion:version,Epoch:epoch,Values:payload,ExpiresAt:time.Now().Add(time.Duration(sess.Options.MaxAge)*time.Second)}
  if rotate {
   if sess.ID!="" {
    removed:=tx.Where("id = ?",authsecurity.Digest(sess.ID)).Delete(&models.AuthSession{})
    if removed.Error!=nil { return removed.Error }
    if !sess.IsNew && removed.RowsAffected!=1 && sess.Values["_reissue"]!=true { return ErrSessionRevoked }
   }
   return tx.Create(&row).Error
  }
  result:=tx.Model(&models.AuthSession{}).Where("id = ? AND epoch = ? AND expires_at > ?",row.ID,epoch,time.Now()).Updates(map[string]any{"values":payload,"expires_at":row.ExpiresAt})
  if result.Error!=nil { return result.Error }; if result.RowsAffected!=1 { return ErrSessionRevoked }; return nil
 })
 if err!=nil { return err }
 sess.ID=raw; sess.IsNew=false; delete(sess.Values,"_rotate"); delete(sess.Values,"_reissue")
 http.SetCookie(w,gs.NewCookie(sess.Name(),encoded,sess.Options)); return nil
}

// Construct once during router setup, after database migrations, without a
// process-global mutable store. Independent routers cannot overwrite each other.
func SessionMiddleware(cfg *config.Config) gin.HandlerFunc {
 db:=database.GetDB()
 if db==nil || db.DB==nil { panic("session middleware: database not initialized") }
 if len(cfg.SessionSecret)<32 { panic("SESSION_SECRET must contain at least 32 characters") }
 if err:=authsecurity.EnsureState(db.DB); err!=nil { panic(err) }
 store:=NewDBSessionStore(db.DB,cfg.SessionSecret,isHTTPS(cfg.AppURL))
 middleware:=sessions.Sessions(SessionCookie,store)
 return func(c *gin.Context) {
  if _,err:=store.Get(c.Request,SessionCookie); err!=nil {
   c.AbortWithStatusJSON(503,AuthResponse{Code:503,Message:"会话服务暂不可用"}); return
  }
  middleware(c)
 }
}
func isHTTPS(rawURL string) bool { return strings.HasPrefix(strings.ToLower(strings.TrimSpace(rawURL)),"https://") }
