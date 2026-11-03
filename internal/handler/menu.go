package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// MenuHandler 处理菜单树与菜单 CRUD 接口。
type MenuHandler struct {
	menus *service.MenuService
}

// NewMenuHandler 创建菜单处理器。
func NewMenuHandler(menus *service.MenuService) *MenuHandler { return &MenuHandler{menus: menus} }

// Tree 返回完整菜单树，包含目录、菜单和按钮。
func (h *MenuHandler) Tree(c *gin.Context) {
	tree, err := h.menus.Tree(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, tree)
}

// Create 新增菜单节点。
func (h *MenuHandler) Create(c *gin.Context) {
	var in service.MenuInput
	if !Bind(c, &in) {
		return
	}
	menu, err := h.menus.Create(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, menu)
}

// Update 更新菜单节点，并阻止把节点挂到自己的子级下。
func (h *MenuHandler) Update(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in service.MenuInput
	if !Bind(c, &in) {
		return
	}
	menu, err := h.menus.Update(c.Request.Context(), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, menu)
}

// Delete 删除菜单；存在子节点时会被拒绝。
func (h *MenuHandler) Delete(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.menus.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
