package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// DictHandler 处理字典类型与字典项接口。
type DictHandler struct {
	dicts *service.DictService
}

// NewDictHandler 创建字典处理器。
func NewDictHandler(dicts *service.DictService) *DictHandler { return &DictHandler{dicts: dicts} }

// ListTypes 分页查询字典类型，status 只接受 0/1。
func (h *DictHandler) ListTypes(c *gin.Context) {
	var q pagination.Query
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	q.Normalize()
	var status *int8
	if raw := c.Query("status"); raw == "0" || raw == "1" {
		v := int8(raw[0] - '0')
		status = &v
	}
	list, total, err := h.dicts.ListTypes(c.Request.Context(), q, status)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// CreateType 新增字典类型。
func (h *DictHandler) CreateType(c *gin.Context) {
	var in service.DictTypeInput
	if !Bind(c, &in) {
		return
	}
	t, err := h.dicts.CreateType(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, t)
}

// UpdateType 更新字典类型。
func (h *DictHandler) UpdateType(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in service.DictTypeInput
	if !Bind(c, &in) {
		return
	}
	t, err := h.dicts.UpdateType(c.Request.Context(), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, t)
}

// DeleteType 删除字典类型及其字典项。
func (h *DictHandler) DeleteType(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.dicts.DeleteType(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// Items 返回指定字典类型下的全部字典项。
func (h *DictHandler) Items(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	items, err := h.dicts.Items(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, items)
}

// CreateItem 新增字典项。
func (h *DictHandler) CreateItem(c *gin.Context) {
	var in service.DictItemInput
	if !Bind(c, &in) {
		return
	}
	item, err := h.dicts.CreateItem(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, item)
}

// UpdateItem 更新字典项。
func (h *DictHandler) UpdateItem(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in service.DictItemInput
	if !Bind(c, &in) {
		return
	}
	item, err := h.dicts.UpdateItem(c.Request.Context(), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, item)
}

// DeleteItem 删除字典项。
func (h *DictHandler) DeleteItem(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.dicts.DeleteItem(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
