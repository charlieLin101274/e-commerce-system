package apis

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/base/logger"
	"github.com/linenxing/e-commerce-system/base/response"
	"github.com/linenxing/e-commerce-system/middlewares"
	"github.com/linenxing/e-commerce-system/models"
	authservice "github.com/linenxing/e-commerce-system/services/auth"
)

type AuthAPI struct {
	service authservice.Service
}

func NewAuthAPI(service authservice.Service) *AuthAPI {
	return &AuthAPI{service: service}
}

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email,max=320"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Name     string `json:"name" binding:"required,max=100"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email,max=320"`
	Password string `json:"password" binding:"required,max=72"`
}

type UserResponse = models.UserResp

func (a *AuthAPI) RegisterRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc) {
	authRoutes := router.Group("/auth")
	authRoutes.POST("/register", a.Register)
	authRoutes.POST("/login", a.Login)

	userRoutes := router.Group("/users")
	userRoutes.Use(authMiddleware)
	userRoutes.GET("/me", a.GetCurrentUser)
}

// Register creates a customer account.
// @Summary Register customer
// @Description Create a customer account and return a JWT access token.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Registration request"
// @Success 201 {object} authservice.AuthOutput
// @Failure 400 {object} response.ErrorBody
// @Failure 409 {object} response.ErrorBody
// @Failure 500 {object} response.ErrorBody
// @Router /auth/register [post]
func (a *AuthAPI) Register(c *gin.Context) {
	var request RegisterRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid registration request")
		return
	}

	output, err := a.service.Register(c.Request.Context(), authservice.RegisterParam{
		Email:    request.Email,
		Password: request.Password,
		Name:     request.Name,
	})
	if err != nil {
		a.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, output)
}

// Login authenticates a customer.
// @Summary Login
// @Description Authenticate with email and password and return a JWT access token.
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login request"
// @Success 200 {object} authservice.AuthOutput
// @Failure 400 {object} response.ErrorBody
// @Failure 401 {object} response.ErrorBody
// @Failure 500 {object} response.ErrorBody
// @Router /auth/login [post]
func (a *AuthAPI) Login(c *gin.Context) {
	var request LoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.Error(c, http.StatusBadRequest, "invalid_request", "invalid login request")
		return
	}

	output, err := a.service.Login(c.Request.Context(), authservice.LoginParam{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		a.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

// GetCurrentUser returns the authenticated customer.
// @Summary Get current user
// @Description Return the user represented by the bearer token.
// @Tags Users
// @Produce json
// @Security BearerAuth
// @Success 200 {object} UserResponse
// @Failure 401 {object} response.ErrorBody
// @Failure 404 {object} response.ErrorBody
// @Failure 500 {object} response.ErrorBody
// @Router /users/me [get]
func (a *AuthAPI) GetCurrentUser(c *gin.Context) {
	userID, exists := middlewares.UserIDFromContext(c)
	if !exists {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return
	}

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "unauthorized", "authentication is required")
		return
	}

	output, err := a.service.GetCurrentUser(c.Request.Context(), parsedUserID)
	if err != nil {
		a.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, output)
}

func (a *AuthAPI) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, apperror.ErrInvalidInput):
		response.Error(c, http.StatusBadRequest, "invalid_request", "request data is invalid")
	case errors.Is(err, apperror.ErrConflict):
		response.Error(c, http.StatusConflict, "email_already_exists", "email is already registered")
	case errors.Is(err, apperror.ErrUnauthorized):
		response.Error(c, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
	case errors.Is(err, apperror.ErrNotFound):
		response.Error(c, http.StatusNotFound, "user_not_found", "user was not found")
	default:
		logger.FromContext(c.Request.Context()).Error().Err(err).Msg("auth request failed")
		response.Error(c, http.StatusInternalServerError, "internal_error", "an internal error occurred")
	}
}
