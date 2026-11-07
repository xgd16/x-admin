package pkg_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/pkg/security"
)

func TestBcryptHasher(t *testing.T) {
	h := security.NewBcryptHasher(4)
	hash, err := h.Hash("Admin@123456")
	require.NoError(t, err)
	assert.NotEqual(t, "Admin@123456", hash)
	assert.True(t, h.Verify(hash, "Admin@123456"))
	assert.False(t, h.Verify(hash, "wrong"))
	assert.False(t, h.Verify("not-a-hash", "Admin@123456"))

	low := security.NewBcryptHasher(0)
	assert.Equal(t, 10, low.Cost)
	high := security.NewBcryptHasher(100)
	assert.Equal(t, 10, high.Cost)
}

func TestTokenManager(t *testing.T) {
	m := security.NewTokenManager("0123456789abcdef", "x-admin", time.Hour)
	token, expires, err := m.Issue(7, "alice", 3, "sid-1")
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(time.Hour), expires, 2*time.Second)
	assert.Equal(t, time.Hour, m.TTL())

	claims, err := m.Parse(token)
	require.NoError(t, err)
	assert.Equal(t, uint64(7), claims.UserID)
	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, int64(3), claims.SessionVersion)
	assert.Equal(t, "sid-1", claims.SessionID)
	assert.Equal(t, "x-admin", claims.Issuer)

	_, err = m.Parse("bad")
	require.ErrorIs(t, err, security.ErrInvalidToken)

	other := security.NewTokenManager("another-secret-123", "x-admin", time.Hour)
	_, err = other.Parse(token)
	require.ErrorIs(t, err, security.ErrInvalidToken)

	expired := security.NewTokenManager("0123456789abcdef", "x-admin", -time.Second)
	expiredToken, _, err := expired.Issue(1, "bob", 0, "")
	require.NoError(t, err)
	_, err = expired.Parse(expiredToken)
	require.True(t, errors.Is(err, security.ErrExpiredToken))
}

func TestRandomToken(t *testing.T) {
	token, err := security.RandomToken()
	require.NoError(t, err)
	assert.Len(t, token, 64)
	other, err := security.RandomToken()
	require.NoError(t, err)
	assert.NotEqual(t, token, other)
}

func TestFileAccessTokens(t *testing.T) {
	m := security.NewTokenManager("0123456789abcdef", "x-admin", time.Hour)
	tok := m.IssueFileAccess(7, 3, "sid-1", time.Hour)
	uid, ver, sid, err := m.ParseFileAccess(tok)
	require.NoError(t, err)
	assert.Equal(t, uint64(7), uid)
	assert.Equal(t, int64(3), ver)
	assert.Equal(t, "sid-1", sid)

	// 另一把密钥签发无法通过签名校验。
	other := security.NewTokenManager("other-secret-00000", "x-admin", time.Hour)
	_, _, _, err = other.ParseFileAccess(tok)
	require.ErrorIs(t, err, security.ErrInvalidToken)

	// 签发时间在未来的凭证未过期，可以校验通过。
	future := m.IssueFileAccess(7, 3, "sid-1", time.Minute)
	_, _, _, err = m.ParseFileAccess(future)
	require.NoError(t, err)
}

func TestParseFileAccessRejectsMalformed(t *testing.T) {
	m := security.NewTokenManager("0123456789abcdef", "x-admin", time.Hour)
	validSig := m.IssueFileAccess(1, 1, "sid", time.Hour)
	parts := strings.Split(validSig, ":")
	require.Len(t, parts, 5)

	cases := map[string]string{
		"空串":       "",
		"段数不足":     "a:b:c:d",
		"段数过多":     "a:b:c:d:e:f",
		"UID 非数字":  "x:1:sid:9999999999:aa",
		"UID 为零":   "0:1:sid:9999999999:aa",
		"版本非数字":    "1:x:sid:9999999999:aa",
		"会话 ID 为空": "1:1::9999999999:aa",
		"过期时间非数字":  "1:1:sid:bad:aa",
		"凭证已过期":    "1:1:sid:1:aa",
		"签名错误":     parts[0] + ":" + parts[1] + ":" + parts[2] + ":" + parts[3] + ":deadbeef",
	}
	for name, raw := range cases {
		_, _, _, err := m.ParseFileAccess(raw)
		require.Error(t, err, name)
	}
}

func TestParseRejectsZeroUserID(t *testing.T) {
	m := security.NewTokenManager("0123456789abcdef", "x-admin", time.Hour)
	token, err := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, security.Claims{}).SignedString([]byte("0123456789abcdef"))
	require.NoError(t, err)
	_, err = m.Parse(token)
	require.ErrorIs(t, err, security.ErrInvalidToken)
}

func TestHashRejectsOverlongPassword(t *testing.T) {
	h := security.NewBcryptHasher(4)
	_, err := h.Hash(strings.Repeat("a", 100))
	require.Error(t, err)
}
