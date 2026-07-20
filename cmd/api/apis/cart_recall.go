package apis

import (
	"net/http"

	"github.com/gin-gonic/gin"
	cartrecallservice "github.com/linenxing/e-commerce-system/services/cartrecall"
)

type CartRecallAPI struct{ service cartrecallservice.Service }

func NewCartRecallAPI(service cartrecallservice.Service) *CartRecallAPI {
	return &CartRecallAPI{service: service}
}
func (a *CartRecallAPI) RegisterRoutes(router *gin.Engine, auth, admin gin.HandlerFunc) {
	group := router.Group("/admin/cart-recall-journeys", auth, admin)
	group.GET("", a.List)
	group.GET("/:id", a.Get)
	group.POST("/:id/cancel", a.Cancel)
}

// ListCartRecallJourneys godoc
// @Summary List cart recall journeys
// @Tags Admin Cart Recall
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.CartRecallJourney
// @Router /admin/cart-recall-journeys [get]
func (a *CartRecallAPI) List(c *gin.Context) {
	values, err := a.service.List(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, values)
}

// GetCartRecallJourney godoc
// @Summary Get cart recall journey
// @Tags Admin Cart Recall
// @Produce json
// @Security BearerAuth
// @Param id path string true "Journey ID"
// @Success 200 {object} models.CartRecallJourney
// @Router /admin/cart-recall-journeys/{id} [get]
func (a *CartRecallAPI) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	value, err := a.service.Get(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}

// CancelCartRecallJourney godoc
// @Summary Cancel cart recall journey
// @Tags Admin Cart Recall
// @Produce json
// @Security BearerAuth
// @Param id path string true "Journey ID"
// @Success 204
// @Router /admin/cart-recall-journeys/{id}/cancel [post]
func (a *CartRecallAPI) Cancel(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := a.service.Cancel(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
