package service

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/pagination"
)

// OnlineSessionInfo 是管理员视角的一条在线会话。
type OnlineSessionInfo struct {
	ID         string    `json:"id"`
	UserID     uint64    `json:"user_id"`
	Username   string    `json:"username"`
	Nickname   string    `json:"nickname"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	Device     string    `json:"device"`
	Remember   bool      `json:"remember"`
	Current    bool      `json:"current"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// OnlineSessionPage 是在线会话分页结果，额外带回去重后的在线用户数。
type OnlineSessionPage struct {
	List     []OnlineSessionInfo `json:"list"`
	Total    int64               `json:"total"`
	Page     int                 `json:"page"`
	PageSize int                 `json:"page_size"`
	Users    int                 `json:"users"`
}

// OnlineQuery 是在线会话列表的筛选条件。
type OnlineQuery struct {
	pagination.Query
	UserID *uint64 `form:"user_id"`
}

// OnlineService 从缓存中的会话记录聚合管理员视角的在线列表。
// 会话键由认证模块写入，格式固定为 session:<userID>:<sessionID>。
type OnlineService struct {
	users UserRepository
	store cache.Store
}

// NewOnlineService 创建在线会话服务。
func NewOnlineService(users UserRepository, store cache.Store) *OnlineService {
	return &OnlineService{users: users, store: store}
}

// List 返回全部在线会话，按最近活动时间倒序，并在读取时清理损坏、
// 账号停用或会话版本已过期的记录。
func (s *OnlineService) List(ctx context.Context, q OnlineQuery, currentSessionID string) (*OnlineSessionPage, error) {
	q.Normalize()
	keys, err := s.store.Keys(ctx, "session:")
	if err != nil {
		return nil, err
	}

	// 先解析键拿到用户 ID，再批量加载用户，避免逐条查询。
	type parsedKey struct {
		key       string
		userID    uint64
		sessionID string
	}
	parsed := make([]parsedKey, 0, len(keys))
	userIDs := make([]uint64, 0, len(keys))
	seenUsers := make(map[uint64]struct{}, len(keys))
	for _, key := range keys {
		userID, sessionID, ok := splitSessionKey(key)
		if !ok {
			continue
		}
		parsed = append(parsed, parsedKey{key: key, userID: userID, sessionID: sessionID})
		if _, ok := seenUsers[userID]; !ok {
			seenUsers[userID] = struct{}{}
			userIDs = append(userIDs, userID)
		}
	}

	users, err := s.users.ListByIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	byID := make(map[uint64]model.User, len(users))
	for _, user := range users {
		byID[user.ID] = user
	}

	keyword := strings.ToLower(strings.TrimSpace(q.Keyword))
	rows := make([]OnlineSessionInfo, 0, len(parsed))
	for _, item := range parsed {
		raw, err := s.store.Get(ctx, item.key)
		if err != nil {
			if err == cache.ErrNotFound {
				continue
			}
			return nil, err
		}
		var record sessionRecord
		if json.Unmarshal([]byte(raw), &record) != nil {
			_ = s.store.Del(ctx, item.key)
			continue
		}
		user, ok := byID[item.userID]
		// 用户已删除、已停用或会话版本已变化时，旧会话已无法用于鉴权，
		// 顺手删除缓存记录，避免列表长期显示幽灵会话。
		if !ok || user.Status != model.StatusEnabled || record.Version != user.SessionVersion {
			_ = s.store.Del(ctx, item.key)
			continue
		}
		if q.UserID != nil && item.userID != *q.UserID {
			continue
		}
		device := describeDevice(record.UserAgent)
		if keyword != "" && !sessionMatches(keyword, user, record.IP, device) {
			continue
		}
		rows = append(rows, OnlineSessionInfo{
			ID:         item.sessionID,
			UserID:     user.ID,
			Username:   user.Username,
			Nickname:   user.Nickname,
			IP:         record.IP,
			UserAgent:  record.UserAgent,
			Device:     device,
			Remember:   record.Remember,
			Current:    item.sessionID == currentSessionID,
			CreatedAt:  record.CreatedAt,
			LastSeenAt: record.LastSeenAt,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].LastSeenAt.After(rows[j].LastSeenAt)
	})

	usersOnline := make(map[uint64]struct{}, len(rows))
	for _, row := range rows {
		usersOnline[row.UserID] = struct{}{}
	}

	total := int64(len(rows))
	start := q.Offset()
	if start > len(rows) {
		start = len(rows)
	}
	end := start + q.PageSize
	if end > len(rows) {
		end = len(rows)
	}
	return &OnlineSessionPage{
		List:     rows[start:end],
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
		Users:    len(usersOnline),
	}, nil
}

// Revoke 强制撤销某个用户的一台在线设备。
func (s *OnlineService) Revoke(ctx context.Context, userID uint64, sessionID string) error {
	if userID == 0 || sessionID == "" {
		return apperr.New(apperr.CodeInvalidArgument, "会话 ID 不合法")
	}
	key := sessionKey(userID, sessionID)
	ok, err := s.store.Exists(ctx, key)
	if err != nil {
		return err
	}
	if !ok {
		return apperr.ErrNotFound
	}
	return s.store.Del(ctx, key)
}

// RevokeUser 强制撤销某个用户的全部在线会话，返回撤销数量。
func (s *OnlineService) RevokeUser(ctx context.Context, userID uint64) (int, error) {
	if userID == 0 {
		return 0, apperr.New(apperr.CodeInvalidArgument, "用户 ID 不合法")
	}
	keys, err := s.store.Keys(ctx, sessionPrefix(userID))
	if err != nil {
		return 0, err
	}
	if len(keys) == 0 {
		return 0, nil
	}
	if err := s.store.Del(ctx, keys...); err != nil {
		return 0, err
	}
	return len(keys), nil
}

// splitSessionKey 把 session:<userID>:<sessionID> 拆成用户与会话 ID。
func splitSessionKey(key string) (uint64, string, bool) {
	rest := strings.TrimPrefix(key, "session:")
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return 0, "", false
	}
	// 使用 strconv.ParseUint 解析用户 ID，避免手写循环遗漏边界。
	userID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || userID == 0 {
		return 0, "", false
	}
	return userID, parts[1], true
}

// sessionMatches 判断会话是否命中关键字（用户名、昵称、IP 或设备）。
func sessionMatches(keyword string, user model.User, ip, device string) bool {
	return strings.Contains(strings.ToLower(user.Username), keyword) ||
		strings.Contains(strings.ToLower(user.Nickname), keyword) ||
		strings.Contains(strings.ToLower(ip), keyword) ||
		strings.Contains(strings.ToLower(device), keyword)
}
