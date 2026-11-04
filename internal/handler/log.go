package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// LogHandler 处理操作日志与登录日志查询、清理接口。
type LogHandler struct {
	logs *service.LogService
}

// NewLogHandler 创建日志处理器。
func NewLogHandler(logs *service.LogService) *LogHandler { return &LogHandler{logs: logs} }

// Operations 分页查询操作日志。
func (h *LogHandler) Operations(c *gin.Context) {
	var f model.OperationLogFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	f.Normalize()
	list, total, err := h.logs.Operations(c.Request.Context(), f)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, f.Page, f.PageSize)
}

// ExportOperations 导出操作日志 Excel。
func (h *LogHandler) ExportOperations(c *gin.Context) {
	var f model.OperationLogFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	data, err := h.logs.ExportOperations(c.Request.Context(), f)
	if err != nil {
		response.Error(c, err)
		return
	}
	Attachment(c, "operation-logs.xlsx", data)
}

// DeleteOperations 按 ID 批量删除操作日志。
func (h *LogHandler) DeleteOperations(c *gin.Context) {
	var in idsInput
	if !Bind(c, &in) {
		return
	}
	if err := h.logs.DeleteOperations(c.Request.Context(), in.IDs...); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// ClearOperations 清空全部操作日志。
func (h *LogHandler) ClearOperations(c *gin.Context) {
	if err := h.logs.ClearOperations(c.Request.Context()); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// Logins 分页查询登录日志。
func (h *LogHandler) Logins(c *gin.Context) {
	var f model.LoginLogFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	f.Normalize()
	list, total, err := h.logs.Logins(c.Request.Context(), f)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, f.Page, f.PageSize)
}

// ExportLogins 导出登录日志 Excel。
func (h *LogHandler) ExportLogins(c *gin.Context) {
	var f model.LoginLogFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	data, err := h.logs.ExportLogins(c.Request.Context(), f)
	if err != nil {
		response.Error(c, err)
		return
	}
	Attachment(c, "login-logs.xlsx", data)
}

// ClearLogins 清空全部登录日志。
func (h *LogHandler) ClearLogins(c *gin.Context) {
	if err := h.logs.ClearLogins(c.Request.Context()); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
