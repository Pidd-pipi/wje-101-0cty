package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/constants"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/dto"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/middleware"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/service"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/util"
)

// BlindTastingHandler exposes blind cupping endpoints.
type BlindTastingHandler struct {
	svc    *service.BlindTastingService
	logger *slog.Logger
}

// NewBlindTastingHandler creates the handler.
func NewBlindTastingHandler(svc *service.BlindTastingService, logger *slog.Logger) *BlindTastingHandler {
	return &BlindTastingHandler{svc: svc, logger: logger}
}

// Create handles POST /blind-tastings.
func (h *BlindTastingHandler) Create(c *gin.Context) {
	var req dto.BlindCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam+": "+err.Error()))
		return
	}
	session, err := h.svc.Create(middleware.GetUserID(c), req.CoffeeBeanID, req.ParticipantIDs)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.OK(session))
}

// List handles GET /blind-tastings.
func (h *BlindTastingHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	involved := c.Query("scope") == "mine"
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}
	items, total, err := h.svc.List(middleware.GetUserID(c), involved, page, pageSize)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(dto.PageData{List: items, Total: total, Page: page, Size: pageSize}))
}

// Get handles GET /blind-tastings/:id.
func (h *BlindTastingHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid blind tasting id"))
		return
	}
	view, err := h.svc.Get(middleware.GetUserID(c), uint(id))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(view))
}

// SubmitScore handles POST /blind-tastings/:id/scores.
func (h *BlindTastingHandler) SubmitScore(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid blind tasting id"))
		return
	}
	var req dto.BlindScoreSubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, constants.MsgInvalidParam+": "+err.Error()))
		return
	}
	score, err := h.svc.SubmitScore(middleware.GetUserID(c), uint(id), req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.OK(score))
}

// Reveal handles POST /blind-tastings/:id/reveal.
func (h *BlindTastingHandler) Reveal(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid blind tasting id"))
		return
	}
	view, err := h.svc.Reveal(middleware.GetUserID(c), uint(id))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(view))
}

// SearchUsers handles GET /blind-tastings/user-search?keyword=.
func (h *BlindTastingHandler) SearchUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	briefs, err := h.svc.SearchUsers(middleware.GetUserID(c), keyword)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(briefs))
}
