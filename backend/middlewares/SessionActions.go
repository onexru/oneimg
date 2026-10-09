package middlewares

import (
 "net/http"
 "oneimg/backend/config"
 "github.com/gin-contrib/sessions"
 "github.com/gin-gonic/gin"
)

func DeleteCurrentSession(c *gin.Context) error {
 session:=sessions.Default(c)
 session.Clear()
 session.Options(sessions.Options{Path:"/",MaxAge:-1,HttpOnly:true,SameSite:http.SameSiteLaxMode,
 Secure:config.App!=nil && isHTTPS(config.App.AppURL)})
 // gin-contrib Clear marks an existing session dirty. An empty session still
 // needs saving so a stale/invalid browser cookie is explicitly expired.
 session.Set("_delete",true)
 return session.Save()
}
