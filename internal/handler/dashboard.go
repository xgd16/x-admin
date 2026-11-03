package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

type DashboardHandler struct {
	dashboard *service.DashboardService
}

func NewDashboardHandler(dashboard *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboard: dashboard}
}

func (h *DashboardHandler) Summary(c *gin.Context) {
	summary, err := h.dashboard.Summary(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, summary)
}
