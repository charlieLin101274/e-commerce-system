package apis

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/base/logger"
	"github.com/linenxing/e-commerce-system/base/response"
	"github.com/linenxing/e-commerce-system/middlewares"
	"net/http"
)

func currentUserID(c *gin.Context) (uuid.UUID, bool) {
	raw, ok := middlewares.UserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return uuid.Nil, false
	}
	return id, true
}
func pathID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid resource ID")
		return uuid.Nil, false
	}
	return id, true
}
func errorsInvalid() error                  { return apperror.ErrInvalidInput }
func parseUUID(v string) (uuid.UUID, error) { return uuid.Parse(v) }
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperror.ErrInvalidInput):
		response.Error(c, http.StatusBadRequest, "invalid_request", "request data is invalid")
	case errors.Is(err, apperror.ErrInsufficientStock):
		response.Error(c, http.StatusConflict, "insufficient_stock", "product stock is insufficient")
	case errors.Is(err, apperror.ErrConflict):
		response.Error(c, http.StatusConflict, "conflict", "resource conflict")
	case errors.Is(err, apperror.ErrNotFound):
		response.Error(c, http.StatusNotFound, "not_found", "resource was not found")
	default:
		logger.FromContext(c.Request.Context()).Error().Err(err).Msg("API request failed")
		response.Error(c, http.StatusInternalServerError, "internal_error", "an internal error occurred")
	}
}
