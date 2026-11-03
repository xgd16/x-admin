package pkg_test

import (
	"bytes"
	"context"
	"errors"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/captcha"
)

func TestCaptchaGenerateAndVerify(t *testing.T) {
	ctx := context.Background()
	store := cache.NewMemory()
	gen := captcha.New(store, time.Minute,
		captcha.WithReader(bytes.NewReader(make([]byte, 64))),
		captcha.WithRand(rand.New(rand.NewSource(1))),
		captcha.WithIDFunc(func() string { return "fixed" }),
		captcha.WithClock(func() time.Time { return time.Unix(0, 0) }),
		captcha.WithLength(4),
	)
	challenge, err := gen.Generate(ctx)
	require.NoError(t, err)
	assert.Equal(t, "fixed", challenge.ID)
	assert.Equal(t, 60, challenge.ExpiresIn)
	assert.True(t, strings.HasPrefix(challenge.Image, "data:image/svg+xml;base64,"))

	code, err := store.Get(ctx, "captcha:fixed")
	require.NoError(t, err)
	assert.Equal(t, "AAAA", code)

	assert.True(t, gen.Verify(ctx, "fixed", "aaaa"))
	// one-time use
	assert.False(t, gen.Verify(ctx, "fixed", "AAAA"))
}

func TestCaptchaVerifyFailures(t *testing.T) {
	ctx := context.Background()
	gen := captcha.New(cache.NewMemory(), time.Minute, captcha.WithIDFunc(func() string { return "id" }))
	_, err := gen.Generate(ctx)
	require.NoError(t, err)

	assert.False(t, gen.Verify(ctx, "", "AAAA"))
	assert.False(t, gen.Verify(ctx, "id", ""))
	assert.False(t, gen.Verify(ctx, "id", "WRONG"))
	assert.False(t, gen.Verify(ctx, "id", "abcd"))
	assert.False(t, gen.Verify(ctx, "missing", "AAAA"))
}

func TestCaptchaStoreFailure(t *testing.T) {
	gen := captcha.New(failingStore{}, time.Minute)
	_, err := gen.Generate(context.Background())
	require.Error(t, err)
}

func TestCaptchaOptions(t *testing.T) {
	store := cache.NewMemory()
	gen := captcha.New(store, time.Minute, captcha.WithLength(0), captcha.WithLength(99), captcha.WithLength(6))
	challenge, err := gen.Generate(context.Background())
	require.NoError(t, err)
	code, err := store.Get(context.Background(), "captcha:"+challenge.ID)
	require.NoError(t, err)
	assert.Len(t, code, 6)
}

type failingStore struct{}

func (failingStore) Set(context.Context, string, string, time.Duration) error {
	return errors.New("set failed")
}
func (failingStore) Get(context.Context, string) (string, error) { return "", cache.ErrNotFound }
func (failingStore) Del(context.Context, ...string) error        { return nil }
func (failingStore) Exists(context.Context, string) (bool, error) {
	return false, nil
}
func (failingStore) Incr(context.Context, string, time.Duration) (int64, error) {
	return 0, nil
}
func (failingStore) Ping(context.Context) error { return nil }
func (failingStore) Close() error               { return nil }
