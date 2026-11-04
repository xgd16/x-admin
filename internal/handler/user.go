package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/excel"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// UserHandler 处理用户管理相关接口。
type UserHandler struct {
	users *service.UserService
}

// NewUserHandler 创建用户处理器。
func NewUserHandler(users *service.UserService) *UserHandler { return &UserHandler{users: users} }

// List 分页查询用户，支持关键字、部门、状态和角色过滤。
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

// Get 查询单个用户详情。
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

// Create 创建用户，可选地绑定部门与角色。
func (h *UserHandler) Create(c *gin.Context) {
	var in service.CreateUserInput
	if !Bind(c, &in) {
		return
	}
	user, err := h.users.Create(c.Request.Context(), in, CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Created(c, user)
}

// Update 更新用户资料、部门、状态与角色。
func (h *UserHandler) Update(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	var in service.UpdateUserInput
	if !Bind(c, &in) {
		return
	}
	user, err := h.users.Update(c.Request.Context(), id, in, CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, user)
}

// Delete 批量删除用户；内置管理员和当前登录账号不可删除。
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

// SetStatus 启用或停用用户；不能停用内置管理员和自己。
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

// ResetPassword 管理员重置指定用户的密码。
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

// AssignRoles 整体替换用户的角色列表。
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
	if err := h.users.AssignRoles(c.Request.Context(), id, in.RoleIDs, CurrentUserID(c)); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// Import 从上传的 Excel 批量创建用户，逐行返回失败原因。
func (h *UserHandler) Import(c *gin.Context) {
	const maxImportBytes int64 = 10 << 20 // 10MB，避免超大 Excel / zip bomb 占满内存
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportBytes)
	header, err := c.FormFile("file")
	if err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "请选择 Excel 文件"))
		return
	}
	if header.Size > maxImportBytes {
		response.Error(c, apperr.New(apperr.CodeInvalidArgument, "导入文件不能超过 10MB"))
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
	// 第一行是表头，至少需要一行数据。
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

// Export 按当前筛选条件导出全部用户为 Excel。
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

// Template 下载用户导入模板。
func (h *UserHandler) Template(c *gin.Context) {
	data, err := h.users.Template()
	if err != nil {
		response.Error(c, err)
		return
	}
	Attachment(c, "user-import-template.xlsx", data)
}
