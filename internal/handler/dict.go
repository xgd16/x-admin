package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

type DictHandler struct {
	dicts *service.DictService
}

func NewDictHandler(dicts *service.DictService) *DictHandler { return &DictHandler{dicts: dicts} }

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
