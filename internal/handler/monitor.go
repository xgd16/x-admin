package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// MonitorHandler 处理系统监控接口。
type MonitorHandler struct {
	monitor *service.MonitorService
}

// NewMonitorHandler 创建系统监控处理器。
func NewMonitorHandler(monitor *service.MonitorService) *MonitorHandler {
	return &MonitorHandler{monitor: monitor}
}

// Overview 返回主机、运行时、数据库与缓存指标。
// 采集设置 3 秒上限，避免某个慢文件系统拖住整个请求。
func (h *MonitorHandler) Overview(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	response.OK(c, h.monitor.Overview(ctx))
}
