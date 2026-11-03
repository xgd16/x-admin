package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

type LogHandler struct {
	logs *service.LogService
}

func NewLogHandler(logs *service.LogService) *LogHandler { return &LogHandler{logs: logs} }

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

func (h *LogHandler) ClearOperations(c *gin.Context) {
	if err := h.logs.ClearOperations(c.Request.Context()); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

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

func (h *LogHandler) ClearLogins(c *gin.Context) {
	if err := h.logs.ClearLogins(c.Request.Context()); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
