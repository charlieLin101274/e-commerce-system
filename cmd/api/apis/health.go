package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type HealthResponse struct {
	Status string `json:"status" example:"ok"`
}

func RegisterHealthRoute(router *gin.Engine) {
	router.GET("/health", Health)
}

// Health reports API availability.
// @Summary Health check
// @Description Return the current API health status.
// @Tags System
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{Status: "ok"})
}
