package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/excel"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

type UserHandler struct {
	users *service.UserService
}

func NewUserHandler(users *service.UserService) *UserHandler { return &UserHandler{users: users} }

func (h *UserHandler) List(c *gin.Context) {
	var f model.UserFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	f.Normalize()
	list, total, err := h.users.List(c.Request.Context(), f)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, f.Page, f.PageSize)
}

func (h *UserHandler) Get(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	user, err := h.users.Get(c.Request.Context(), id)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

func (h *UserHandler) Create(c *gin.Context) {
	var in service.CreateUserInput
	if !Bind(c, &in) {
		return
	}
	user, err := h.users.Create(c.Request.Context(), in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, user)
}

func (h *UserHandler) Update(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in service.UpdateUserInput
	if !Bind(c, &in) {
		return
	}
	user, err := h.users.Update(c.Request.Context(), id, in)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

func (h *UserHandler) Delete(c *gin.Context) {
	var in idsInput
	if !Bind(c, &in) {
		return
	}
	if err := h.users.Delete(c.Request.Context(), in.IDs, CurrentUserID(c)); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *UserHandler) SetStatus(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in struct {
		Status int8 `json:"status"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.users.SetStatus(c.Request.Context(), id, in.Status, CurrentUserID(c)); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *UserHandler) ResetPassword(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in struct {
		Password string `json:"password" binding:"required"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.users.ResetPassword(c.Request.Context(), id, in.Password); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *UserHandler) AssignRoles(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in struct {
		RoleIDs []uint64 `json:"role_ids"`
	}
	if !Bind(c, &in) {
		return
	}
	if err := h.users.AssignRoles(c.Request.Context(), id, in.RoleIDs); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

func (h *UserHandler) Import(c *gin.Context) {
	header, err := c.FormFile("file")
	if err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "请选择 Excel 文件"))
		return
	}
	file, err := header.Open()
	if err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "无法读取文件"))
		return
	}
	defer file.Close()
	rows, err := excel.Read(file)
	if err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "Excel 解析失败"))
		return
	}
	if len(rows) < 2 {
		response.Error(c, apperr.New(apperr.CodeInvalidArgument, "导入文件没有数据"))
		return
	}
	importRows := make([]service.UserImportRow, 0, len(rows)-1)
	for _, row := range rows[1:] {
		importRows = append(importRows, service.ImportRowFromCells(row))
	}
	result, err := h.users.Import(c.Request.Context(), importRows)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, result)
}

func (h *UserHandler) Export(c *gin.Context) {
	var f model.UserFilter
	if err := c.ShouldBindQuery(&f); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	data, err := h.users.Export(c.Request.Context(), f)
	if err != nil {
		response.Error(c, err)
		return
	}
	Attachment(c, "users.xlsx", data)
}

func (h *UserHandler) Template(c *gin.Context) {
	data, err := h.users.Template()
	if err != nil {
		response.Error(c, err)
		return
	}
	Attachment(c, "user-import-template.xlsx", data)
}
