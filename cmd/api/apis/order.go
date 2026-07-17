package apis

import (
	"github.com/gin-gonic/gin"
	orderservice "github.com/linenxing/e-commerce-system/services/order"
	"net/http"
)

type OrderAPI struct{ service orderservice.Service }

func NewOrderAPI(s orderservice.Service) *OrderAPI { return &OrderAPI{service: s} }
func (a *OrderAPI) RegisterRoutes(r *gin.Engine, auth gin.HandlerFunc) {
	g := r.Group("/orders")
	g.Use(auth)
	g.POST("", a.Create)
	g.GET("", a.List)
	g.GET("/:id", a.Get)
}

// CreateOrder godoc
// @Summary Create order from cart
// @Tags Orders
// @Security BearerAuth
// @Produce json
// @Success 201 {object} models.OrderResp
// @Router /orders [post]
func (a *OrderAPI) Create(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	v, err := a.service.Create(c.Request.Context(), uid)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

// ListOrders godoc
// @Summary List my orders
// @Tags Orders
// @Security BearerAuth
// @Produce json
// @Success 200 {array} models.OrderResp
// @Router /orders [get]
func (a *OrderAPI) List(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	v, err := a.service.List(c.Request.Context(), uid)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

// GetOrder godoc
// @Summary Get my order
// @Tags Orders
// @Security BearerAuth
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} models.OrderResp
// @Router /orders/{id} [get]
func (a *OrderAPI) Get(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	v, err := a.service.Get(c.Request.Context(), uid, id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}
