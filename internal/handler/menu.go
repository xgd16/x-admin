package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

type MenuHandler struct {
	menus *service.MenuService
}

func NewMenuHandler(menus *service.MenuService) *MenuHandler { return &MenuHandler{menus: menus} }

func (h *MenuHandler) Tree(c *gin.Context) {
	tree, err := h.menus.Tree(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, tree)
}

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
