package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// DashboardHandler 处理看板数据接口。
type DashboardHandler struct {
	dashboard *service.DashboardService
}

// NewDashboardHandler 创建看板处理器。
func NewDashboardHandler(dashboard *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboard: dashboard}
}

// Summary 返回用户、角色、菜单、文件、任务统计，登录趋势和最近操作。
func (h *DashboardHandler) Summary(c *gin.Context) {
	summary, err := h.dashboard.Summary(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, summary)
}
