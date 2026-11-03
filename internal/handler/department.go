package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

type DepartmentHandler struct {
	depts *service.DepartmentService
}

func NewDepartmentHandler(depts *service.DepartmentService) *DepartmentHandler {
	return &DepartmentHandler{depts: depts}
}

func (h *DepartmentHandler) Tree(c *gin.Context) {
	tree, err := h.depts.Tree(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, tree)
}

func (h *DepartmentHandler) Create(c *gin.Context) {
	var in service.DepartmentInput
	if !Bind(c, &in) {
		return
	}
	dept, err := h.depts.Create(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, dept)
}

func (h *DepartmentHandler) Update(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in service.DepartmentInput
	if !Bind(c, &in) {
		return
	}
	dept, err := h.depts.Update(c.Request.Context(), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, dept)
}

func (h *DepartmentHandler) Delete(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.depts.Delete(c.Request.Context(), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}
