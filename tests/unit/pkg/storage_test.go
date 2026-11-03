package pkg_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/pkg/storage"
)

func TestLocalStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewLocal(t.TempDir(), "/uploads/",
		storage.WithClock(func() time.Time { return time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC) }),
		storage.WithIDFunc(func() string { return "abc123" }),
	)
	require.NoError(t, err)

	info, err := store.Save(ctx, "../../photo.PNG", strings.NewReader("hello"))
	require.NoError(t, err)
	assert.Equal(t, "2026/11/abc123.png", info.Key)
	assert.Equal(t, "photo.PNG", info.Name)
	assert.Equal(t, "/uploads/2026/11/abc123.png", info.URL)
	assert.Equal(t, int64(5), info.Size)
	assert.Contains(t, info.MIME, "text/plain")

	rc, err := store.Open(ctx, info.Key)
	require.NoError(t, err)
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, "hello", string(data))

	require.NoError(t, store.Delete(ctx, info.Key))
	require.NoError(t, store.Delete(ctx, info.Key))
}

func TestLocalStoreRejectsTraversalAndBadExt(t *testing.T) {
	ctx := context.Background()
	store, err := storage.NewLocal(t.TempDir(), "", storage.WithIDFunc(func() string { return "id" }))
	require.NoError(t, err)

	info, err := store.Save(ctx, "weird.$$$", strings.NewReader("x"))
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(info.Key, "/id.bin"))
	assert.NotContains(t, info.URL, "//")

	_, err = store.Open(ctx, "../secret")
	require.Error(t, err)
	_, err = store.Open(ctx, "/")
	require.Error(t, err)
	require.Error(t, store.Delete(ctx, ".."))
}

func TestLocalStoreRequiresRoot(t *testing.T) {
	_, err := storage.NewLocal("", "/uploads")
	require.Error(t, err)
}

func TestLocalStoreNewID(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir(), "/uploads")
	require.NoError(t, err)
	info, err := store.Save(context.Background(), "a.txt", strings.NewReader("abc"))
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(info.Key, ".txt"))
	assert.GreaterOrEqual(t, len(info.Key), len("2026/11/")+32+4)
}
