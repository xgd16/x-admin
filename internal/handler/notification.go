package handler

import (
	"github.com/gin-gonic/gin"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/pkg/response"
	"x-admin/internal/service"
)

// NotificationHandler 处理当前用户的站内通知。
type NotificationHandler struct {
	notifications *service.NotificationService
}

// NewNotificationHandler 创建通知处理器。
func NewNotificationHandler(notifications *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{notifications: notifications}
}

// List 分页查询当前用户的通知；unread=1 时只看未读。
func (h *NotificationHandler) List(c *gin.Context) {
	var q pagination.Query
	if err := c.ShouldBindQuery(&q); err != nil {
		response.Error(c, apperr.Wrap(err, apperr.CodeInvalidArgument, "查询参数不合法"))
		return
	}
	q.Normalize()
	unreadOnly := c.Query("unread") == "1"
	list, total, err := h.notifications.List(c.Request.Context(), CurrentUserID(c), q, unreadOnly)
	if err != nil {
		response.Error(c, err)
		return
	}
	response.Page(c, list, total, q.Page, q.PageSize)
}

// UnreadCount 返回当前用户的未读通知数量，用于命令栏角标。
func (h *NotificationHandler) UnreadCount(c *gin.Context) {
	count, err := h.notifications.UnreadCount(c.Request.Context(), CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"count": count})
}

// MarkRead 将当前用户的一条通知标记为已读。
func (h *NotificationHandler) MarkRead(c *gin.Context) {
	id, ok := PathID(c)
	if !ok {
		return
	}
	if err := h.notifications.MarkRead(c.Request.Context(), CurrentUserID(c), id); err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, nil)
}

// MarkAllRead 将当前用户的全部未读通知标记为已读，返回更新条数。
func (h *NotificationHandler) MarkAllRead(c *gin.Context) {
	updated, err := h.notifications.MarkAllRead(c.Request.Context(), CurrentUserID(c))
	if err != nil {
		response.Error(c, err)
		return
	}
	response.OK(c, gin.H{"updated": updated})
}
