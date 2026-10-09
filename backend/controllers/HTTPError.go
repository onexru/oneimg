package controllers

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"oneimg/backend/utils/result"
)

// internalFailure preserves operator diagnostics without exposing SQL/schema,
// storage credentials, or upstream response internals to an authenticated user.
func internalFailure(c *gin.Context, message string, err error) {
	if err != nil {
		log.Printf("%s: %v", message, err)
	}
	c.JSON(http.StatusInternalServerError, result.Error(http.StatusInternalServerError, message))
}
