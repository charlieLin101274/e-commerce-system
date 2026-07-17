package apis

import (
	"github.com/gin-gonic/gin"
	cartservice "github.com/linenxing/e-commerce-system/services/cart"
	"net/http"
)

type CartAPI struct{ service cartservice.Service }

func NewCartAPI(s cartservice.Service) *CartAPI { return &CartAPI{service: s} }

type AddCartItemRequest struct {
	ProductID string `json:"product_id" binding:"required,uuid"`
	Quantity  int64  `json:"quantity" binding:"required,gt=0"`
}
type UpdateCartItemRequest struct {
	Quantity int64 `json:"quantity" binding:"required,gt=0"`
}

func (a *CartAPI) RegisterRoutes(r *gin.Engine, auth gin.HandlerFunc) {
	g := r.Group("/cart")
	g.Use(auth)
	g.GET("", a.Get)
	g.POST("/items", a.Add)
	g.PUT("/items/:id", a.Update)
	g.DELETE("/items/:id", a.Remove)
}

// GetCart godoc
// @Summary Get cart
// @Tags Cart
// @Security BearerAuth
// @Produce json
// @Success 200 {object} models.CartResp
// @Router /cart [get]
func (a *CartAPI) Get(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	v, err := a.service.Get(c.Request.Context(), uid)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

// AddCartItem godoc
// @Summary Add cart item
// @Tags Cart
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body AddCartItemRequest true "Cart item"
// @Success 200 {object} models.CartResp
// @Router /cart/items [post]
func (a *CartAPI) Add(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	var req AddCartItemRequest
	if c.ShouldBindJSON(&req) != nil {
		writeError(c, errorsInvalid())
		return
	}
	pid, err := parseUUID(req.ProductID)
	if err != nil {
		writeError(c, errorsInvalid())
		return
	}
	v, err := a.service.AddItem(c.Request.Context(), cartservice.AddItemParam{UserID: uid, ProductID: pid, Quantity: req.Quantity})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

// UpdateCartItem godoc
// @Summary Update cart item
// @Tags Cart
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param id path string true "Cart item ID"
// @Param request body UpdateCartItemRequest true "Quantity"
// @Success 200 {object} models.CartResp
// @Router /cart/items/{id} [put]
func (a *CartAPI) Update(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req UpdateCartItemRequest
	if c.ShouldBindJSON(&req) != nil {
		writeError(c, errorsInvalid())
		return
	}
	v, err := a.service.UpdateItem(c.Request.Context(), cartservice.UpdateItemParam{UserID: uid, ItemID: id, Quantity: req.Quantity})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

// RemoveCartItem godoc
// @Summary Remove cart item
// @Tags Cart
// @Security BearerAuth
// @Param id path string true "Cart item ID"
// @Success 204
// @Router /cart/items/{id} [delete]
func (a *CartAPI) Remove(c *gin.Context) {
	uid, ok := currentUserID(c)
	if !ok {
		return
	}
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := a.service.RemoveItem(c.Request.Context(), cartservice.RemoveItemParam{UserID: uid, ItemID: id}); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
