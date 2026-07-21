package apis

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/middlewares"
	"github.com/linenxing/e-commerce-system/models"
	campaignservice "github.com/linenxing/e-commerce-system/services/campaign"
)

type CampaignAPI struct{ service campaignservice.Service }

type AdminCampaignResponse struct {
	models.Campaign
	RuleVersion     int                          `json:"rule_version"`
	RuleContextType models.EvaluationContextType `json:"rule_context_type,omitempty"`
	EligibilityRule *models.RuleGroup            `json:"eligibility_rule,omitempty"`
}

func NewCampaignAPI(service campaignservice.Service) *CampaignAPI {
	return &CampaignAPI{service: service}
}

type CampaignRequest struct {
	Name                  string                       `json:"name" binding:"required,max=200"`
	Description           string                       `json:"description" binding:"max=5000"`
	Priority              int                          `json:"priority"`
	StartsAt              time.Time                    `json:"starts_at" binding:"required"`
	EndsAt                time.Time                    `json:"ends_at" binding:"required"`
	PromotionTitle        string                       `json:"promotion_title" binding:"required,max=200"`
	PromotionDescription  string                       `json:"promotion_description" binding:"max=5000"`
	BenefitType           models.BenefitType           `json:"benefit_type" binding:"required,oneof=fixed_amount percentage"`
	BenefitValue          int64                        `json:"benefit_value" binding:"required,gt=0"`
	MaximumDiscountAmount *int64                       `json:"maximum_discount_amount"`
	ProductIDs            []uuid.UUID                  `json:"product_ids"`
	Categories            []string                     `json:"categories"`
	ContextType           models.EvaluationContextType `json:"context_type" binding:"omitempty,oneof=campaign_discovery cart_recall"`
	EligibilityRule       *models.RuleGroup            `json:"eligibility_rule"`
}

func (a *CampaignAPI) RegisterRoutes(router *gin.Engine, optionalAuth, auth, admin gin.HandlerFunc) {
	public := router.Group("/campaigns", optionalAuth)
	public.GET("", a.ListPublic)
	public.GET("/:id", a.GetPublic)
	public.POST("/:id/evaluate", a.EvaluatePublic)
	group := router.Group("/admin/campaigns")
	group.Use(auth, admin)
	group.POST("", a.Create)
	group.GET("", a.ListAdmin)
	group.GET("/:id", a.GetAdmin)
	group.PUT("/:id", a.Update)
	group.POST("/:id/publish", a.Publish)
	group.POST("/:id/pause", a.Pause)
	group.POST("/:id/resume", a.Resume)
	group.POST("/:id/archive", a.Archive)
	group.POST("/:id/rules/validate", a.ValidateRules)
	group.POST("/:id/rules/evaluate", a.EvaluateRules)
}

// CreateCampaign godoc
// @Summary Create draft campaign
// @Tags Admin Campaigns
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CampaignRequest true "Campaign"
// @Success 201 {object} models.Campaign
// @Router /admin/campaigns [post]
func (a *CampaignAPI) Create(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}
	param, ok := bindCampaign(c)
	if !ok {
		return
	}
	value, err := a.service.Create(c.Request.Context(), userID, param)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, adminCampaign(value))
}

// UpdateCampaign godoc
// @Summary Update draft campaign
// @Tags Admin Campaigns
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Campaign ID"
// @Param request body CampaignRequest true "Campaign"
// @Success 200 {object} models.Campaign
// @Router /admin/campaigns/{id} [put]
func (a *CampaignAPI) Update(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	param, ok := bindCampaign(c)
	if !ok {
		return
	}
	value, err := a.service.Update(c.Request.Context(), id, param)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, adminCampaign(value))
}

// ListAdminCampaigns godoc
// @Summary List campaigns for admin
// @Tags Admin Campaigns
// @Produce json
// @Security BearerAuth
// @Success 200 {array} models.Campaign
// @Router /admin/campaigns [get]
func (a *CampaignAPI) ListAdmin(c *gin.Context) {
	values, err := a.service.ListAdmin(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	responses := make([]AdminCampaignResponse, 0, len(values))
	for _, value := range values {
		responses = append(responses, adminCampaign(value))
	}
	c.JSON(http.StatusOK, responses)
}

// GetAdminCampaign godoc
// @Summary Get campaign for admin
// @Tags Admin Campaigns
// @Produce json
// @Security BearerAuth
// @Param id path string true "Campaign ID"
// @Success 200 {object} models.Campaign
// @Router /admin/campaigns/{id} [get]
func (a *CampaignAPI) GetAdmin(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	value, err := a.service.GetAdmin(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, adminCampaign(value))
}

// ListCampaigns godoc
// @Summary List currently active campaigns
// @Tags Campaigns
// @Produce json
// @Param product_id query string false "Product ID used for scope matching"
// @Param limit query int false "Candidate page size (default 20, maximum 20)"
// @Param offset query int false "Candidate offset (default 0)"
// @Success 200 {array} models.Campaign
// @Router /campaigns [get]
func (a *CampaignAPI) ListPublic(c *gin.Context) {
	productID, ok := optionalProductID(c)
	if !ok {
		return
	}
	userID := optionalCurrentUserID(c)
	page, ok := campaignPage(c)
	if !ok {
		return
	}
	values, err := a.service.ListPublic(c.Request.Context(), productID, userID, page)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, values)
}

func campaignPage(c *gin.Context) (campaignservice.PageParam, bool) {
	page := campaignservice.PageParam{}
	var err error
	if raw := c.Query("limit"); raw != "" {
		page.Limit, err = strconv.Atoi(raw)
		if err != nil || page.Limit <= 0 || page.Limit > 20 {
			writeError(c, errorsInvalid())
			return campaignservice.PageParam{}, false
		}
	}
	if raw := c.Query("offset"); raw != "" {
		page.Offset, err = strconv.Atoi(raw)
		if err != nil || page.Offset < 0 {
			writeError(c, errorsInvalid())
			return campaignservice.PageParam{}, false
		}
	}
	return page, true
}

// GetCampaign godoc
// @Summary Get currently active campaign
// @Tags Campaigns
// @Produce json
// @Param id path string true "Campaign ID"
// @Param product_id query string false "Product ID used for scope matching"
// @Success 200 {object} models.Campaign
// @Router /campaigns/{id} [get]
func (a *CampaignAPI) GetPublic(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	productID, ok := optionalProductID(c)
	if !ok {
		return
	}
	value, err := a.service.GetPublic(c.Request.Context(), id, productID, optionalCurrentUserID(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}

// EvaluatePublicCampaign godoc
// @Summary Evaluate campaign eligibility
// @Tags Campaigns
// @Produce json
// @Param id path string true "Campaign ID"
// @Param product_id query string false "Product ID used for server-side facts"
// @Success 200 {object} models.EvaluationResult
// @Router /campaigns/{id}/evaluate [post]
func (a *CampaignAPI) EvaluatePublic(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	productID, ok := optionalProductID(c)
	if !ok {
		return
	}
	value, err := a.service.EvaluatePublic(c.Request.Context(), id, productID, optionalCurrentUserID(c))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}

// ValidateCampaignRules godoc
// @Summary Validate active campaign rules
// @Tags Admin Campaigns
// @Produce json
// @Security BearerAuth
// @Param id path string true "Campaign ID"
// @Success 200 {object} map[string]interface{}
// @Router /admin/campaigns/{id}/rules/validate [post]
func (a *CampaignAPI) ValidateRules(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	errors, err := a.service.ValidateRules(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"valid": len(errors) == 0, "validation_errors": errors})
}

type RuleEvaluateRequest struct {
	ContextType models.EvaluationContextType `json:"context_type" binding:"required,oneof=campaign_discovery cart_recall"`
	Facts       models.EvaluationFacts       `json:"facts"`
}

// EvaluateCampaignRules godoc
// @Summary Dry-run active campaign rules
// @Tags Admin Campaigns
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Campaign ID"
// @Param request body RuleEvaluateRequest true "Evaluation context"
// @Success 200 {object} models.EvaluationResult
// @Router /admin/campaigns/{id}/rules/evaluate [post]
func (a *CampaignAPI) EvaluateRules(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	var request RuleEvaluateRequest
	if c.ShouldBindJSON(&request) != nil {
		writeError(c, errorsInvalid())
		return
	}
	value, err := a.service.EvaluateRules(c.Request.Context(), id, request.ContextType, request.Facts)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}

// PublishCampaign godoc
// @Summary Publish draft campaign
// @Tags Admin Campaigns
// @Produce json
// @Security BearerAuth
// @Param id path string true "Campaign ID"
// @Success 200 {object} models.Campaign
// @Router /admin/campaigns/{id}/publish [post]
func (a *CampaignAPI) Publish(c *gin.Context) { a.transition(c, a.service.Publish) }

// PauseCampaign godoc
// @Summary Pause campaign
// @Tags Admin Campaigns
// @Produce json
// @Security BearerAuth
// @Param id path string true "Campaign ID"
// @Success 200 {object} models.Campaign
// @Router /admin/campaigns/{id}/pause [post]
func (a *CampaignAPI) Pause(c *gin.Context) { a.transition(c, a.service.Pause) }

// ResumeCampaign godoc
// @Summary Resume paused campaign
// @Tags Admin Campaigns
// @Produce json
// @Security BearerAuth
// @Param id path string true "Campaign ID"
// @Success 200 {object} models.Campaign
// @Router /admin/campaigns/{id}/resume [post]
func (a *CampaignAPI) Resume(c *gin.Context) { a.transition(c, a.service.Resume) }

// ArchiveCampaign godoc
// @Summary Archive campaign
// @Tags Admin Campaigns
// @Produce json
// @Security BearerAuth
// @Param id path string true "Campaign ID"
// @Success 200 {object} models.Campaign
// @Router /admin/campaigns/{id}/archive [post]
func (a *CampaignAPI) Archive(c *gin.Context) { a.transition(c, a.service.Archive) }

func (a *CampaignAPI) transition(c *gin.Context, action func(context.Context, uuid.UUID) (models.Campaign, error)) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	value, err := action(c.Request.Context(), id)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, adminCampaign(value))
}

func adminCampaign(value models.Campaign) AdminCampaignResponse {
	return AdminCampaignResponse{Campaign: value, RuleVersion: value.RuleVersion, RuleContextType: value.RuleContextType, EligibilityRule: value.EligibilityRule}
}

func bindCampaign(c *gin.Context) (campaignservice.WriteParam, bool) {
	var request CampaignRequest
	if c.ShouldBindJSON(&request) != nil {
		writeError(c, errorsInvalid())
		return campaignservice.WriteParam{}, false
	}
	return campaignservice.WriteParam{Name: request.Name, Description: request.Description, Priority: request.Priority, StartsAt: request.StartsAt, EndsAt: request.EndsAt, PromotionTitle: request.PromotionTitle, PromotionDescription: request.PromotionDescription, BenefitType: request.BenefitType, BenefitValue: request.BenefitValue, MaximumDiscountAmount: request.MaximumDiscountAmount, ProductIDs: request.ProductIDs, Categories: request.Categories, ContextType: request.ContextType, EligibilityRule: request.EligibilityRule}, true
}

func optionalCurrentUserID(c *gin.Context) *uuid.UUID {
	raw, ok := middlewares.UserIDFromContext(c)
	if !ok {
		return nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}
	return &id
}

func optionalProductID(c *gin.Context) (*uuid.UUID, bool) {
	raw := c.Query("product_id")
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		writeError(c, errorsInvalid())
		return nil, false
	}
	return &id, true
}
