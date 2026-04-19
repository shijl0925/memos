package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func composeResponse(data interface{}) interface{} {
	type R struct {
		Data interface{} `json:"data"`
	}

	return R{
		Data: data,
	}
}

func writeJSON(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, composeResponse(data))
}

func abortWithError(c *gin.Context, status int, message string, err error) {
	if err != nil {
		_ = c.Error(err)
	}
	c.AbortWithStatusJSON(status, gin.H{
		"message": message,
	})
}

func getCurrentUserID(c *gin.Context) int {
	return c.MustGet(getUserIDContextKey()).(int)
}
