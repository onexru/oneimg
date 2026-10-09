package controllers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"oneimg/backend/database"
	"oneimg/backend/middlewares"
	"oneimg/backend/utils/authsecurity"
	"oneimg/backend/utils/result"
)

type accountLoginHistoryItem struct {
	CreatedAt time.Time `json:"created_at"`
	Method    string    `json:"method"`
}

// ListAccountLoginHistory exposes the current account's recent successful
// sign-ins. Owner identifiers and session data never enter the response.
func ListAccountLoginHistory(c *gin.Context) {
	var db *gorm.DB
	if current := database.GetDB(); current != nil {
		db = current.DB
	}
	listAccountLoginHistoryDB(c, db)
}

func listAccountLoginHistoryDB(c *gin.Context, db *gorm.DB) {
	user, ok := middlewares.GetCurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, result.Error(http.StatusUnauthorized, "未登录"))
		return
	}
	if user.ID <= 0 || c.GetString("auth_method") == "api_token" {
		c.JSON(http.StatusForbidden, result.Error(http.StatusForbidden, "需要账户登录会话"))
		return
	}
	if db == nil {
		c.JSON(http.StatusServiceUnavailable, result.Error(http.StatusServiceUnavailable, "登录历史服务暂不可用"))
		return
	}
	rows, err := authsecurity.LoginHistory(db.WithContext(c.Request.Context()), user.ID)
	if err != nil {
		internalFailure(c, "获取登录历史失败", err)
		return
	}
	items := make([]accountLoginHistoryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, accountLoginHistoryItem{CreatedAt: row.CreatedAt, Method: row.Method})
	}
	c.JSON(http.StatusOK, result.Success("ok", items))
}

// Deliberately outside saveUserSession: renewal, password-change rotation and
// revoke-others reissue use that helper but must not fabricate a new login.
func recordAccountLogin(c *gin.Context, db *gorm.DB, userID int, method string) {
	if userID <= 0 {
		return
	}
	// A disconnected response must not erase a session already committed. Bound
	// the best-effort history write without relying on request cancellation.
	if db == nil {
		log.Printf("successful login history unavailable for account %d", userID)
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 2*time.Second)
	defer cancel()
	if err := authsecurity.RecordSuccessfulLogin(db.Session(&gorm.Session{Logger: db.Logger.LogMode(logger.Silent)}).WithContext(ctx), userID, method); err != nil {
		// Do not log the error/SQL/request: providers and drivers may include data.
		log.Printf("successful login history unavailable for account %d", userID)
	}
}
