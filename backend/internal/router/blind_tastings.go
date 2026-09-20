package router

import (
	"github.com/gin-gonic/gin"

	"github.com/wjecoffeetaste/wjecoffeetaste/internal/config"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/handler"
	"github.com/wjecoffeetaste/wjecoffeetaste/internal/middleware"
)

func registerBlindTastingRoutes(v1 *gin.RouterGroup, cfg *config.Config, h *handler.BlindTastingHandler, limiter *middleware.RateLimiter) {
	blind := v1.Group("/blind-tastings", middleware.AuthRequired(cfg))
	blind.GET("", h.List)
	blind.GET("/user-search", h.SearchUsers)
	blind.GET("/:id", h.Get)
	blind.POST("", limiter.Limit(), h.Create)
	blind.POST("/:id/scores", limiter.Limit(), h.SubmitScore)
	blind.POST("/:id/reveal", limiter.Limit(), h.Reveal)
}
