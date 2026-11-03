// Package handler 负责 HTTP 协议层：参数绑定、校验入口与统一响应。
//
// 处理器不直接访问数据库，也不写业务规则；具体逻辑全部委托给 service。
package handler

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/authn"
	"x-admin/internal/pkg/response"
)

// Bind 解析 JSON 请求体，失败时写出参数错误响应并返回 false。
func Bind(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "请求参数不合法"))
		return false
	}
	return true
}

// PathID 解析路径参数 :id，要求为正整数。
func PathID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, apperr.New(apperr.CodeInvalidArgument, "ID 不合法"))
		return 0, false
	}
	return id, true
}

// CurrentUserID 返回当前登录用户 ID；未登录时返回 0。
func CurrentUserID(c *gin.Context) uint64 {
	if p, ok := authn.FromContext(c.Request.Context()); ok {
		return p.UserID
	}
	return 0
}

// idsInput 是批量操作的通用请求体。
type idsInput struct {
	IDs []uint64 `json:"ids" binding:"required"`
}

// Attachment 以附件形式下载字节内容，文件名同时提供 ASCII 与 UTF-8 编码。
func Attachment(c *gin.Context, filename string, content []byte) {
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		filename, url.QueryEscape(filename)))
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}
