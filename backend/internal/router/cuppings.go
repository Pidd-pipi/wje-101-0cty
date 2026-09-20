package router

import (
	"github.com/gin-gonic/gin"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/config"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/handler"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/middleware"
)

func registerCuppingRoutes(v1 *gin.RouterGroup, cfg *config.Config, h *handler.CuppingHandler, limiter *middleware.RateLimiter) {
	cuppings := v1.Group("/cuppings", middleware.AuthRequired(cfg))
	cuppings.GET("", h.List)
	cuppings.GET("/:id", h.Get)
	cuppings.POST("", limiter.Limit(), h.Create)
	cuppings.POST("/:id/submit", limiter.Limit(), h.Submit)
	cuppings.POST("/:id/reveal", limiter.Limit(), h.Reveal)
}
