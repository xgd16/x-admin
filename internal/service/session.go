package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/cache"
)

// sessionRecord 是缓存中的一条登录会话元数据。
type sessionRecord struct {
	IP         string    `json:"ip"`
	UserAgent  string    `json:"ua"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	Version    int64     `json:"version"`
	Remember   bool      `json:"remember"`
}

// SessionInfo 是个人中心展示的在线设备。
type SessionInfo struct {
	ID         string    `json:"id"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	Device     string    `json:"device"`
	Current    bool      `json:"current"`
	Remember   bool      `json:"remember"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// sessionKey 返回某个登录会话的缓存键。
func sessionKey(userID uint64, sessionID string) string {
	return fmt.Sprintf("session:%d:%s", userID, sessionID)
}

// sessionPrefix 返回某个用户全部会话的键前缀。
func sessionPrefix(userID uint64) string {
	return fmt.Sprintf("session:%d:", userID)
}

// SessionActive 判断会话是否仍然有效，供鉴权中间件校验 access token。
// CheckFileAccess 校验文件访问凭证对应的账号状态与会话版本。
// 改密、停用账号后，旧的 /uploads Cookie 会立即失效。
func (s *AuthService) CheckFileAccess(ctx context.Context, userID uint64, sessionVersion int64, sessionID string) error {
	if sessionID == "" {
		return apperr.ErrUnauthorized
	}
	active, err := s.SessionActive(ctx, userID, sessionID)
	if err != nil {
		return err
	}
	if !active {
		return apperr.New(apperr.CodeTokenExpired, "登录已过期，请重新登录")
	}
	status, version, err := s.users.SessionState(ctx, userID)
	if err != nil {
		if IsNotFound(err) {
			return apperr.ErrUnauthorized
		}
		return err
	}
	if status != model.StatusEnabled {
		return apperr.New(apperr.CodeForbidden, "账号已停用")
	}
	if version != sessionVersion {
		return apperr.New(apperr.CodeTokenExpired, "登录状态已更新，请重新登录")
	}
	return nil
}

func (s *AuthService) SessionActive(ctx context.Context, userID uint64, sessionID string) (bool, error) {
	if userID == 0 || sessionID == "" {
		return false, nil
	}
	ok, err := s.store.Exists(ctx, sessionKey(userID, sessionID))
	if err != nil {
		if err == cache.ErrNotFound {
			return false, nil
		}
		return false, err
	}
	return ok, nil
}

// touchSession 写入或刷新会话元数据，并保留首次登录时间。
func (s *AuthService) touchSession(ctx context.Context, userID uint64, sessionID string, version int64, meta LoginMeta, ttl time.Duration, remember bool) {
	if sessionID == "" {
		return
	}
	now := time.Now()
	record := sessionRecord{
		IP: meta.IP, UserAgent: meta.UserAgent, Version: version,
		Remember: remember, CreatedAt: now, LastSeenAt: now,
	}
	if raw, err := s.store.Get(ctx, sessionKey(userID, sessionID)); err == nil {
		var existing sessionRecord
		if json.Unmarshal([]byte(raw), &existing) == nil {
			if !existing.CreatedAt.IsZero() {
				record.CreatedAt = existing.CreatedAt
			}
			// RefreshSession 不携带请求元数据，保留原有 IP / UA。
			if meta.IP == "" && meta.UserAgent == "" {
				record.IP = existing.IP
				record.UserAgent = existing.UserAgent
			}
		}
	}
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	_ = s.store.Set(ctx, sessionKey(userID, sessionID), string(data), ttl)
}

// Sessions 返回用户当前版本下的全部在线设备，并清理版本已过期的会话。
func (s *AuthService) Sessions(ctx context.Context, userID uint64, currentSessionID string) ([]SessionInfo, error) {
	user, err := s.users.Get(ctx, userID)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrUnauthorized
		}
		return nil, err
	}
	keys, err := s.store.Keys(ctx, sessionPrefix(userID))
	if err != nil {
		return nil, err
	}
	rows := make([]SessionInfo, 0, len(keys))
	for _, key := range keys {
		raw, err := s.store.Get(ctx, key)
		if err != nil {
			continue
		}
		var record sessionRecord
		if json.Unmarshal([]byte(raw), &record) != nil {
			_ = s.store.Del(ctx, key)
			continue
		}
		// 改密、启停账号等操作会递增 session_version，
		// 旧版本会话已经无法用于刷新，直接从列表移除。
		if record.Version != user.SessionVersion {
			_ = s.store.Del(ctx, key)
			continue
		}
		id := strings.TrimPrefix(key, sessionPrefix(userID))
		rows = append(rows, SessionInfo{
			ID: id, IP: record.IP, UserAgent: record.UserAgent,
			Device:     describeDevice(record.UserAgent),
			Current:    id == currentSessionID,
			Remember:   record.Remember,
			CreatedAt:  record.CreatedAt,
			LastSeenAt: record.LastSeenAt,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Current != rows[j].Current {
			return rows[i].Current
		}
		return rows[i].LastSeenAt.After(rows[j].LastSeenAt)
	})
	return rows, nil
}

// RevokeSession 撤销用户自己的一个登录会话。
func (s *AuthService) RevokeSession(ctx context.Context, userID uint64, sessionID string) error {
	if userID == 0 || sessionID == "" {
		return apperr.New(apperr.CodeInvalidArgument, "会话 ID 不合法")
	}
	return s.store.Del(ctx, sessionKey(userID, sessionID))
}

// RevokeOtherSessions 撤销除当前设备外的全部会话，返回撤销数量。
func (s *AuthService) RevokeOtherSessions(ctx context.Context, userID uint64, currentSessionID string) (int, error) {
	keys, err := s.store.Keys(ctx, sessionPrefix(userID))
	if err != nil {
		return 0, err
	}
	revoked := 0
	for _, key := range keys {
		if strings.TrimPrefix(key, sessionPrefix(userID)) == currentSessionID {
			continue
		}
		if err := s.store.Del(ctx, key); err != nil {
			return revoked, err
		}
		revoked++
	}
	return revoked, nil
}

// RevokeAllSessions 撤销用户的全部会话，包括当前设备。
func (s *AuthService) RevokeAllSessions(ctx context.Context, userID uint64) (int, error) {
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

// describeDevice 根据 User-Agent 给出便于识别的设备名称。
func describeDevice(userAgent string) string {
	ua := strings.ToLower(userAgent)
	var platform string
	switch {
	case strings.Contains(ua, "iphone"):
		platform = "iPhone"
	case strings.Contains(ua, "ipad"):
		platform = "iPad"
	case strings.Contains(ua, "android"):
		platform = "Android"
	case strings.Contains(ua, "mac os"), strings.Contains(ua, "macintosh"):
		platform = "macOS"
	case strings.Contains(ua, "windows"):
		platform = "Windows"
	case strings.Contains(ua, "linux"):
		platform = "Linux"
	}
	var browser string
	switch {
	case strings.Contains(ua, "edg/"):
		browser = "Edge"
	case strings.Contains(ua, "chrome/"), strings.Contains(ua, "crios/"):
		browser = "Chrome"
	case strings.Contains(ua, "firefox/"):
		browser = "Firefox"
	case strings.Contains(ua, "safari/"), strings.Contains(ua, "version/"):
		browser = "Safari"
	}
	switch {
	case platform != "" && browser != "":
		return platform + " · " + browser
	case platform != "":
		return platform
	case browser != "":
		return browser
	}
	return "未知设备"
}
