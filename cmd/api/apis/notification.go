package apis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
	notificationservice "github.com/linenxing/e-commerce-system/services/notification"
)

type NotificationAPI struct{ service notificationservice.Service }

type NotificationResponse struct {
	ID        uuid.UUID  `json:"id"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	DeepLink  string     `json:"deep_link,omitempty"`
	OpenedAt  *time.Time `json:"opened_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func NewNotificationAPI(service notificationservice.Service) *NotificationAPI {
	return &NotificationAPI{service: service}
}

func (a *NotificationAPI) RegisterRoutes(router *gin.Engine, auth, admin gin.HandlerFunc) {
	user := router.Group("/notifications", auth)
	user.GET("", a.List)
	user.POST("/:id/open", a.Open)
	preferences := router.Group("/me/notification-preferences", auth)
	preferences.GET("", a.GetPreferences)
	preferences.PUT("", a.UpdatePreferences)
	group := router.Group("/admin/notification-tasks", auth, admin)
	group.GET("", a.ListAdmin)
	group.GET("/:id", a.GetAdmin)
	group.POST("/:id/retry", a.Retry)
}

// ListNotifications godoc
// @Summary List delivered in-app notifications
// @Tags Notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {array} NotificationResponse
// @Router /notifications [get]
func (a *NotificationAPI) List(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	values, err := a.service.ListForUser(c.Request.Context(), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	responses := make([]NotificationResponse, 0, len(values))
	for _, value := range values {
		var payload models.NotificationPayload
		if err = json.Unmarshal(value.Payload, &payload); err != nil {
			writeError(c, fmt.Errorf("decode notification payload: %w", err))
			return
		}
		responses = append(responses, NotificationResponse{ID: value.ID, Title: payload.Title, Body: payload.Body, DeepLink: payload.DeepLink, OpenedAt: value.OpenedAt, CreatedAt: value.CreatedAt})
	}
	c.JSON(http.StatusOK, responses)
}

// OpenNotification godoc
// @Summary Mark an in-app notification opened
// @Tags Notifications
// @Security BearerAuth
// @Param id path string true "Notification ID"
// @Success 204
// @Router /notifications/{id}/open [post]
func (a *NotificationAPI) Open(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := a.service.Open(c.Request.Context(), id, userID); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListAdminNotificationTasks godoc
// @Summary List notification tasks
// @Tags Admin Notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.NotificationTask
// @Router /admin/notification-tasks [get]
func (a *NotificationAPI) ListAdmin(c *gin.Context) {
	values, err := a.service.ListAdmin(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, values)
}

// GetAdminNotificationTask godoc
// @Summary Get notification task
// @Tags Admin Notifications
// @Produce json
// @Security BearerAuth
// @Param id path string true "Notification Task ID"
// @Success 200 {object} models.NotificationTask
// @Router /admin/notification-tasks/{id} [get]
func (a *NotificationAPI) GetAdmin(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	value, err := a.service.GetAdmin(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}

// RetryAdminNotificationTask godoc
// @Summary Retry failed notification task
// @Tags Admin Notifications
// @Security BearerAuth
// @Param id path string true "Notification Task ID"
// @Success 204
// @Router /admin/notification-tasks/{id}/retry [post]
func (a *NotificationAPI) Retry(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := a.service.Retry(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GetNotificationPreferences godoc
// @Summary Get notification preferences
// @Tags Notification Preferences
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.NotificationPreferences
// @Router /me/notification-preferences [get]
func (a *NotificationAPI) GetPreferences(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	value, err := a.service.GetPreferences(c.Request.Context(), userID)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}

// UpdateNotificationPreferences godoc
// @Summary Update notification preferences
// @Tags Notification Preferences
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.NotificationPreferences true "Notification preferences"
// @Success 200 {object} models.NotificationPreferences
// @Router /me/notification-preferences [put]
func (a *NotificationAPI) UpdatePreferences(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	var request models.NotificationPreferences
	if c.ShouldBindJSON(&request) != nil {
		writeError(c, errorsInvalid())
		return
	}
	value, err := a.service.UpdatePreferences(c.Request.Context(), userID, request)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}
