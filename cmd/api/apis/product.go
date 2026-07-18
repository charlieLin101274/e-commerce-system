package apis

import (
	"github.com/gin-gonic/gin"
	"github.com/linenxing/e-commerce-system/models"
	productservice "github.com/linenxing/e-commerce-system/services/product"
	"net/http"
)

type ProductAPI struct{ service productservice.Service }

func NewProductAPI(s productservice.Service) *ProductAPI { return &ProductAPI{service: s} }

type ProductRequest struct {
	Name        string               `json:"name" binding:"required,max=200"`
	Description string               `json:"description" binding:"max=5000"`
	Category    string               `json:"category" binding:"max=100"`
	Price       int64                `json:"price" binding:"gte=0"`
	Stock       int64                `json:"stock" binding:"gte=0"`
	Status      models.ProductStatus `json:"status" enums:"active,inactive"`
}

func (a *ProductAPI) RegisterRoutes(r *gin.Engine, auth, admin gin.HandlerFunc) {
	r.GET("/products", a.List)
	r.GET("/products/:id", a.Get)
	g := r.Group("/admin/products")
	g.Use(auth, admin)
	g.GET("", a.ListAdmin)
	g.POST("", a.Create)
	g.PUT("/:id", a.Update)
	g.DELETE("/:id", a.Disable)
}

func (a *ProductAPI) ListAdmin(c *gin.Context) {
	v, err := a.service.List(c.Request.Context(), true)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

// ListProducts godoc
// @Summary List products
// @Tags Products
// @Produce json
// @Success 200 {array} models.ProductResp
// @Router /products [get]
func (a *ProductAPI) List(c *gin.Context) {
	v, err := a.service.List(c.Request.Context(), false)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

// GetProduct godoc
// @Summary Get product
// @Tags Products
// @Produce json
// @Param id path string true "Product ID"
// @Success 200 {object} models.ProductResp
// @Router /products/{id} [get]
func (a *ProductAPI) Get(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	v, err := a.service.Get(c.Request.Context(), id, false)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

// CreateProduct godoc
// @Summary Create product
// @Tags Admin Products
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ProductRequest true "Product"
// @Success 201 {object} models.ProductResp
// @Router /admin/products [post]
func (a *ProductAPI) Create(c *gin.Context) {
	var req ProductRequest
	if c.ShouldBindJSON(&req) != nil {
		writeError(c, errorsInvalid())
		return
	}
	v, err := a.service.Create(c.Request.Context(), productservice.CreateParam{Name: req.Name, Description: req.Description, Category: req.Category, Price: req.Price, Stock: req.Stock})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

// UpdateProduct godoc
// @Summary Update product
// @Tags Admin Products
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Product ID"
// @Param request body ProductRequest true "Product"
// @Success 200 {object} models.ProductResp
// @Router /admin/products/{id} [put]
func (a *ProductAPI) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var req ProductRequest
	if c.ShouldBindJSON(&req) != nil {
		writeError(c, errorsInvalid())
		return
	}
	v, err := a.service.Update(c.Request.Context(), productservice.UpdateParam{ID: id, Name: req.Name, Description: req.Description, Category: req.Category, Price: req.Price, Stock: req.Stock, Status: req.Status})
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

// DisableProduct godoc
// @Summary Disable product
// @Tags Admin Products
// @Security BearerAuth
// @Param id path string true "Product ID"
// @Success 204
// @Router /admin/products/{id} [delete]
func (a *ProductAPI) Disable(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	if err := a.service.Disable(c.Request.Context(), id); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
