package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// DepartmentHandler 处理部门树与部门 CRUD 接口。
type DepartmentHandler struct {
	depts *service.DepartmentService
}

// NewDepartmentHandler 创建部门处理器。
func NewDepartmentHandler(depts *service.DepartmentService) *DepartmentHandler {
	return &DepartmentHandler{depts: depts}
}

// Tree 返回完整部门树。
func (h *DepartmentHandler) Tree(c *gin.Context) {
	tree, err := h.depts.Tree(c.Request.Context())
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, tree)
}

// Create 新增部门。
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

// Update 更新部门，并阻止把部门挂到自己的子级下。
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

// Delete 删除部门；存在子部门或下属用户时会被拒绝。
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
