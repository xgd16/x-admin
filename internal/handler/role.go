package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

type RoleHandler struct {
	roles *service.RoleService
}

func NewRoleHandler(roles *service.RoleService) *RoleHandler { return &RoleHandler{roles: roles} }

func (h *RoleHandler) List(c *gin.Context) {
	var f model.RoleFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	f.Normalize()
	list, total, err := h.roles.List(c.Request.Context(), f)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, f.Page, f.PageSize)
}

func (h *RoleHandler) All(c *gin.Context) {
	list, err := h.roles.All(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, list)
}

func (h *RoleHandler) Create(c *gin.Context) {
	var in service.RoleInput
	if !Bind(c, &in) {
		return
	}
	role, err := h.roles.Create(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, role)
}

func (h *RoleHandler) Update(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in service.RoleInput
	if !Bind(c, &in) {
		return
	}
	role, err := h.roles.Update(c.Request.Context(), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, role)
}

func (h *RoleHandler) Delete(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.roles.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *RoleHandler) Menus(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	ids, err := h.roles.Menus(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, ids)
}

func (h *RoleHandler) AssignMenus(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in struct {
		MenuIDs []uint64 `json:"menu_ids"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.roles.AssignMenus(c.Request.Context(), id, in.MenuIDs); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
