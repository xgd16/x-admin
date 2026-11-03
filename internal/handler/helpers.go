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

func Bind(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "请求参数不合法"))
		return false
	}
	return true
}

func PathID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		response.Error(c, apperr.New(apperr.CodeInvalidArgument, "ID 不合法"))
		return 0, false
	}
	return id, true
}

func CurrentUserID(c *gin.Context) uint64 {
	if p, ok := authn.FromContext(c.Request.Context()); ok {
		return p.UserID
	}
	return 0
}

type idsInput struct {
	IDs []uint64 `json:"ids" binding:"required"`
}

func Attachment(c *gin.Context, filename string, content []byte) {
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		filename, url.QueryEscape(filename)))
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", content)
}
