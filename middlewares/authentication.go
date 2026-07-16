package middlewares

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	baseauth "github.com/linenxing/e-commerce-system/base/auth"
	"github.com/linenxing/e-commerce-system/base/response"
)

const (
	userIDContextKey = "user_id"
	roleContextKey   = "user_role"
)

func Authentication(tokenManager baseauth.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.Fields(header)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
			return
		}

		claims, err := tokenManager.Verify(c.Request.Context(), parts[1])
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "unauthorized", "valid bearer token is required")
			return
		}

		c.Set(userIDContextKey, claims.UserID.String())
		c.Set(roleContextKey, claims.Role)
		c.Next()
	}
}

func UserIDFromContext(c *gin.Context) (string, bool) {
	value, exists := c.Get(userIDContextKey)
	if !exists {
		return "", false
	}
	userID, ok := value.(string)
	return userID, ok
}
