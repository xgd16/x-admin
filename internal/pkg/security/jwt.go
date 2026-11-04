// Package security 提供 JWT 与密码哈希等安全基础设施。
package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWT 解析错误：令牌无效或已过期。
var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

// Claims 是 access token 的负载。
type Claims struct {
	UserID         uint64 `json:"uid"`
	Username       string `json:"username"`
	SessionVersion int64  `json:"sv,omitempty"`  // 与用户当前 session_version 不一致时令牌立即失效
	SessionID      string `json:"sid,omitempty"` // 登录会话 ID，用于在线设备列表与单设备撤销
	jwt.RegisteredClaims
}

// TokenManager 负责签发和校验 HS256 access token。
type TokenManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

// NewTokenManager 创建令牌管理器。
func NewTokenManager(secret, issuer string, ttl time.Duration) *TokenManager {
	return &TokenManager{secret: []byte(secret), issuer: issuer, ttl: ttl, now: time.Now}
}

// TTL 返回 access token 的有效期。
func (m *TokenManager) TTL() time.Duration { return m.ttl }

// Issue 为用户签发 access token，并返回过期时间。
// sessionVersion 让安全变更后的旧令牌立即失效；sessionID 用于定位登录会话。
func (m *TokenManager) Issue(userID uint64, username string, sessionVersion int64, sessionID string) (string, time.Time, error) {
	now := m.now()
	expires := now.Add(m.ttl)
	claims := Claims{
		UserID:         userID,
		Username:       username,
		SessionVersion: sessionVersion,
		SessionID:      sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   strconv.FormatUint(userID, 10),
			ID:        randomID(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expires),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expires, nil
}

// Parse 校验签名算法、签发者和过期时间，并返回 claims。
// 显式拒绝非 HS256 算法，防止算法混淆攻击。
func (m *TokenManager) Parse(raw string) (*Claims, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithExpirationRequired())
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}
	if !parsed.Valid || claims.UserID == 0 {
		return nil, ErrInvalidToken
	}
	return claims, nil
}

// randomID 生成 JWT 的 jti，随机源故障时退回时间戳。
func randomID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b[:])
}

// HashToken 返回不透明令牌的 SHA-256 摘要，用于数据库索引而不保存明文。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// RandomToken 返回 32 字节的密码学安全随机令牌（十六进制编码）。
// 用于 refresh token、MFA 挑战和 Passkey 会话等不透明令牌。
func RandomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// FileAccessCookie 是浏览器访问 /uploads 静态文件时携带的 HttpOnly Cookie。
const FileAccessCookie = "x_admin_files"

// fileAccessContext 是文件访问令牌的签名上下文，避免与其他 HMAC 用途混用。
const fileAccessContext = "x-admin/file-access:v1:"

// IssueFileAccess 签发短期文件访问凭证。
// 仍会在每次请求时校验用户状态与 session_version，因此可安全设置较长有效期。
func (m *TokenManager) IssueFileAccess(userID uint64, sessionVersion int64, sessionID string, ttl time.Duration) string {
	expires := m.now().Add(ttl).Unix()
	payload := strconv.FormatUint(userID, 10) + ":" + strconv.FormatInt(sessionVersion, 10) + ":" +
		sessionID + ":" + strconv.FormatInt(expires, 10)
	return payload + ":" + m.signFileAccess(payload)
}

// ParseFileAccess 校验文件访问凭证的签名与过期时间。
func (m *TokenManager) ParseFileAccess(raw string) (uint64, int64, string, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 5 {
		return 0, 0, "", ErrInvalidToken
	}
	userID, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || userID == 0 {
		return 0, 0, "", ErrInvalidToken
	}
	version, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return 0, 0, "", ErrInvalidToken
	}
	sessionID := parts[2]
	if sessionID == "" {
		return 0, 0, "", ErrInvalidToken
	}
	expires, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil || expires <= m.now().Unix() {
		return 0, 0, "", ErrExpiredToken
	}
	payload := strings.Join(parts[:4], ":")
	expected := m.signFileAccess(payload)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(parts[4])) != 1 {
		return 0, 0, "", ErrInvalidToken
	}
	return userID, version, sessionID, nil
}

// signFileAccess 使用 JWT 密钥派生的 HMAC-SHA256 签名文件访问凭证。
func (m *TokenManager) signFileAccess(payload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(fileAccessContext + payload))
	return hex.EncodeToString(mac.Sum(nil))
}
