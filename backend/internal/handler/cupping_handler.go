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

// CuppingHandler exposes blind cupping endpoints.
type CuppingHandler struct {
	svc    *service.CuppingService
	logger *slog.Logger
}

// NewCuppingHandler creates a CuppingHandler.
func NewCuppingHandler(svc *service.CuppingService, logger *slog.Logger) *CuppingHandler {
	return &CuppingHandler{svc: svc, logger: logger}
}

// List handles GET /cuppings.
func (h *CuppingHandler) List(c *gin.Context) {
	items, err := h.svc.List(middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(items))
}

// Get handles GET /cuppings/:id.
func (h *CuppingHandler) Get(c *gin.Context) {
	id, ok := parseCuppingID(c)
	if !ok {
		return
	}
	view, err := h.svc.Get(id, middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(view))
}

// Create handles POST /cuppings.
func (h *CuppingHandler) Create(c *gin.Context) {
	var req dto.CuppingCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest,
			constants.MsgInvalidParam+": "+err.Error()))
		return
	}
	view, err := h.svc.Create(middleware.GetUserID(c), req.CoffeeBeanID, req.ParticipantIDs)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, dto.OK(view))
}

// Submit handles POST /cuppings/:id/submit.
func (h *CuppingHandler) Submit(c *gin.Context) {
	id, ok := parseCuppingID(c)
	if !ok {
		return
	}
	var req dto.CuppingScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest,
			constants.MsgInvalidParam+": "+err.Error()))
		return
	}
	view, err := h.svc.Submit(id, middleware.GetUserID(c), req)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(view))
}

// Reveal handles POST /cuppings/:id/reveal.
func (h *CuppingHandler) Reveal(c *gin.Context) {
	id, ok := parseCuppingID(c)
	if !ok {
		return
	}
	view, err := h.svc.Reveal(id, middleware.GetUserID(c))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, dto.OK(view))
}

func parseCuppingID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, constants.CodeBadRequest, "invalid cupping id"))
		return 0, false
	}
	return uint(id), true
}
