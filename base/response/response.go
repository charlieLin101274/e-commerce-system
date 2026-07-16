package response

import "github.com/gin-gonic/gin"

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorBody{
		Code:    code,
		Message: message,
	})
}
