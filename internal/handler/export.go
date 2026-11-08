package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// ExportHandler 处理异步导出任务的提交、进度查询、取消与下载。
type ExportHandler struct {
	exports *service.ExportService
}

// NewExportHandler 创建导出任务处理器。
func NewExportHandler(exports *service.ExportService) *ExportHandler {
	return &ExportHandler{exports: exports}
}

// exportSubmitInput 是提交导出任务的请求体；params 原样传给业务构建器。
type exportSubmitInput struct {
	Module string          `json:"module" binding:"required"`
	Params json.RawMessage `json:"params"`
}

// List 分页查询当前用户的导出任务，可选按状态过滤。
func (h *ExportHandler) List(c *gin.Context) {
	var q pagination.Query
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	var status *int8
	if raw := c.Query("status"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 8)
		if err != nil {
			response.Error(c, apperr.New(apperr.CodeInvalidArgument, "状态参数不合法"))
			return
		}
		s := int8(v)
		status = &s
	}
	list, total, err := h.exports.Tasks(c.Request.Context(), CurrentUserID(c), status, q)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// Submit 提交一个异步导出任务，返回任务记录（含进度字段）。
func (h *ExportHandler) Submit(c *gin.Context) {
	var in exportSubmitInput
	if !Bind(c, &in) {
		return
	}
	task, err := h.exports.Submit(c.Request.Context(), CurrentUserID(c), in.Module, in.Params)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, task)
}

// Cancel 请求取消一个排队中或执行中的任务。
func (h *ExportHandler) Cancel(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.exports.Cancel(c.Request.Context(), CurrentUserID(c), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// Delete 删除一条已结束的任务记录及其产物文件。
func (h *ExportHandler) Delete(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.exports.Delete(c.Request.Context(), CurrentUserID(c), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// FilenameTemplate 返回当前生效的导出文件名模板（缺失/非法时返回默认值）。
func (h *ExportHandler) FilenameTemplate(c *gin.Context) {
	tpl, err := h.exports.FilenameTemplate(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, tpl)
}

// UpdateFilenameTemplate 校验并保存导出文件名模板，返回规整后的结果。
func (h *ExportHandler) UpdateFilenameTemplate(c *gin.Context) {
	var in service.FilenameTemplate
	if !Bind(c, &in) {
		return
	}
	normalized, err := h.exports.UpdateFilenameTemplate(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, normalized)
}

// Download 以流式方式返回导出文件，并设置中文兼容的下载文件名。
func (h *ExportHandler) Download(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	task, rc, err := h.exports.OpenFile(c.Request.Context(), CurrentUserID(c), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	defer rc.Close()
	disposition := `attachment; filename="` + task.FileName + `"; filename*=UTF-8''` + url.QueryEscape(task.FileName)
	c.DataFromReader(http.StatusOK, task.FileSize,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", rc,
		map[string]string{"Content-Disposition": disposition})
}
