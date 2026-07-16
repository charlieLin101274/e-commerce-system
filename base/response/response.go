package response

import "github.com/gin-gonic/gin"

type ErrorBody struct {
	Code    string `json:"code" example:"invalid_request"`
	Message string `json:"message" example:"request data is invalid"`
}

func Error(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, ErrorBody{
		Code:    code,
		Message: message,
	})
}
