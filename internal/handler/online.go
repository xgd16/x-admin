package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// OnlineHandler 处理管理员视角的在线会话查询与强制下线。
type OnlineHandler struct {
	online *service.OnlineService
}

// NewOnlineHandler 创建在线会话处理器。
func NewOnlineHandler(online *service.OnlineService) *OnlineHandler {
	return &OnlineHandler{online: online}
}

// List 返回在线会话分页列表，支持按用户与关键字筛选。
func (h *OnlineHandler) List(c *gin.Context) {
	var q service.OnlineQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	page, err := h.online.List(c.Request.Context(), q, CurrentSessionID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, page)
}

// Revoke 撤销某个用户的在线会话。
// 请求体带 session_id 时只撤销指定设备；不带时撤销该用户全部会话。
func (h *OnlineHandler) Revoke(c *gin.Context) {
	userID, ok := PathParamID(c, "userID")
	if !ok {
		return
	}
	var in struct {
		SessionID string `json:"session_id"`
	}
	if c.Request.ContentLength > 0 {
		if !Bind(c, &in) {
			return
		}
	}
	if in.SessionID != "" {
		if err := h.online.Revoke(c.Request.Context(), userID, in.SessionID); err != nil {
			response.Error(c, err)
			return
		}
		response.OK(c, gin.H{"revoked": 1})
		return
	}
	revoked, err := h.online.RevokeUser(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"revoked": revoked})
}
