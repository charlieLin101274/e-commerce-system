package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterHealthRoute(router *gin.Engine) {
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
