package shared

import (
	"log"
	"net/http"
	"oneimg/backend/utils/result"

	"github.com/gin-gonic/gin"
)

// InternalFailure preserves operator diagnostics without exposing SQL/schema,
// storage credentials, or upstream response internals to an authenticated user.
//
// 导出：auth / users / images / storage 各包都需要它作为「500 但不泄露细节」
// 的统一出口，避免各自再写一份 fmt.Sprintf("%v", err)。
func InternalFailure(c *gin.Context, message string, err error) {
	if err != nil {
		log.Printf("%s: %v", message, err)
	}
	c.JSON(http.StatusInternalServerError, result.Error(http.StatusInternalServerError, message))
}
