package service

import (
	"context"
	"strings"
	"time"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
)

// NotificationInput 是业务服务推送站内消息的载荷。
// Type 为空时默认使用 "info"。
type NotificationInput struct {
	UserID  uint64
	Title   string
	Content string
	Type    string
	Link    string
	ActorID uint64
}

// NotificationService 实现站内通知的查询与写入。
type NotificationService struct {
	notifications NotificationRepository
}

// NewNotificationService 创建通知服务。
func NewNotificationService(notifications NotificationRepository) *NotificationService {
	return &NotificationService{notifications: notifications}
}

// List 分页查询某个用户的通知。
func (s *NotificationService) List(ctx context.Context, userID uint64, q pagination.Query, unreadOnly bool) ([]model.Notification, int64, error) {
	q.Normalize()
	return s.notifications.List(ctx, userID, unreadOnly, q.Page, q.PageSize)
}

// UnreadCount 返回未读通知数量。
func (s *NotificationService) UnreadCount(ctx context.Context, userID uint64) (int64, error) {
	return s.notifications.CountUnread(ctx, userID)
}

// MarkRead 把一条通知标记为已读；消息不属于该用户时返回 404。
func (s *NotificationService) MarkRead(ctx context.Context, userID, id uint64) error {
	if err := s.notifications.MarkRead(ctx, userID, id, time.Now()); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	return nil
}

// MarkAllRead 把用户全部未读通知标记为已读。
func (s *NotificationService) MarkAllRead(ctx context.Context, userID uint64) (int64, error) {
	return s.notifications.MarkAllRead(ctx, userID, time.Now())
}

// Notify 写入一条通知。
// 通知是业务的附属动作，校验失败只返回错误，不影响触发它的主流程。
func (s *NotificationService) Notify(ctx context.Context, in NotificationInput) error {
	if in.UserID == 0 {
		return nil
	}
	title := truncateRunes(trim(in.Title), 128)
	if title == "" {
		return apperr.New(apperr.CodeInvalidArgument, "通知标题不能为空")
	}
	kind := trim(in.Type)
	switch kind {
	case model.NotificationInfo, model.NotificationSuccess, model.NotificationWarning, model.NotificationError:
	default:
		kind = model.NotificationInfo
	}
	// 只允许站内相对路径，防止把通知变成任意外链跳转。
	link := truncateRunes(trim(in.Link), 255)
	if link != "" && !strings.HasPrefix(link, "/") {
		link = ""
	}
	return s.notifications.Create(ctx, &model.Notification{
		UserID:  in.UserID,
		Title:   title,
		Content: truncateRunes(trim(in.Content), 500),
		Type:    kind,
		Link:    link,
		ActorID: in.ActorID,
	})
}

// CleanBefore 删除超过保留期的通知。
func (s *NotificationService) CleanBefore(ctx context.Context, days int) error {
	return s.notifications.CleanBefore(ctx, days)
}

// notify 是 nil 安全的推送辅助函数，供可选依赖 Notifier 的服务使用。
func notify(ctx context.Context, n Notifier, in NotificationInput) {
	if n == nil {
		return
	}
	_ = n.Notify(ctx, in)
}

// truncateRunes 按字符（而非字节）截断，避免切断中文。
func truncateRunes(v string, limit int) string {
	runes := []rune(v)
	if len(runes) <= limit {
		return v
	}
	return string(runes[:limit])
}
