// Package security 提供 JWT 与密码哈希等安全基础设施。
package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
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
	UserID   uint64 `json:"uid"`
	Username string `json:"username"`
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
func (m *TokenManager) Issue(userID uint64, username string) (string, time.Time, error) {
	now := m.now()
	expires := now.Add(m.ttl)
	claims := Claims{
		UserID:   userID,
		Username: username,
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

// RandomToken 返回 32 字节的密码学安全随机令牌（十六进制编码）。
// 用于 refresh token、MFA 挑战和 Passkey 会话等不透明令牌。
func RandomToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
