package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// ParameterHandler 处理系统参数管理接口。
type ParameterHandler struct {
	params *service.ParameterService
}

// NewParameterHandler 创建参数处理器。
func NewParameterHandler(params *service.ParameterService) *ParameterHandler {
	return &ParameterHandler{params: params}
}

// Public 返回公开参数，无需登录即可访问。
func (h *ParameterHandler) Public(c *gin.Context) {
	list, err := h.params.Public(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, list)
}

// List 分页查询系统参数。
func (h *ParameterHandler) List(c *gin.Context) {
	var q pagination.Query
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	q.Normalize()
	list, total, err := h.params.List(c.Request.Context(), q)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// Create 新增系统参数。
func (h *ParameterHandler) Create(c *gin.Context) {
	var in service.ParameterInput
	if !Bind(c, &in) {
		return
	}
	p, err := h.params.Create(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, p)
}

// Update 更新系统参数。
func (h *ParameterHandler) Update(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in service.ParameterInput
	if !Bind(c, &in) {
		return
	}
	p, err := h.params.Update(c.Request.Context(), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, p)
}

// Delete 删除系统参数。
func (h *ParameterHandler) Delete(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.params.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
