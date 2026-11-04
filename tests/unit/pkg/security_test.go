package pkg_test

import (
	"errors"
	"testing"
	"time"

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
