package pkg_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
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

// errReader 在读取时立即失败。
type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("read failure") }

func TestNewLocalRejectsRootUnderFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "occupied")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0o600))
	_, err := storage.NewLocal(filepath.Join(file, "sub"), "/uploads")
	require.Error(t, err)
}

func TestSaveFailurePaths(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	fixed := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	store, err := storage.NewLocal(root, "/uploads",
		storage.WithClock(func() time.Time { return fixed }),
		storage.WithIDFunc(func() string { return "f" }),
	)
	require.NoError(t, err)

	// 头部读取阶段即失败。
	_, err = store.Save(ctx, "a.txt", errReader{})
	require.Error(t, err)

	// 头部读取成功后拷贝中断，失败后不应残留文件。
	_, err = store.Save(ctx, "b.txt", io.MultiReader(bytes.NewReader(make([]byte, 512)), errReader{}))
	require.Error(t, err)
	_, err = store.Open(ctx, "2026/11/f.txt")
	require.Error(t, err)

	// 目标路径已是一个目录时创建文件失败。
	require.NoError(t, os.MkdirAll(filepath.Join(root, "2026", "11", "f.bin"), 0o755))
	_, err = store.Save(ctx, "c.bin", bytes.NewReader([]byte("x")))
	require.Error(t, err)

	// 存储根目录被文件占用时，月份子目录创建失败。
	otherRoot := filepath.Join(t.TempDir(), "root")
	store2, err := storage.NewLocal(otherRoot, "/uploads",
		storage.WithClock(func() time.Time { return fixed }),
		storage.WithIDFunc(func() string { return "f" }),
	)
	require.NoError(t, err)
	require.NoError(t, os.Remove(otherRoot))
	require.NoError(t, os.WriteFile(otherRoot, []byte("not a dir"), 0o600))
	_, err = store2.Save(ctx, "d.bin", bytes.NewReader([]byte("x")))
	require.Error(t, err)
}

func TestSanitizeExtTooLong(t *testing.T) {
	store, err := storage.NewLocal(t.TempDir(), "/uploads", storage.WithIDFunc(func() string { return "f" }))
	require.NoError(t, err)
	info, err := store.Save(context.Background(), "a.abcdefghijkl", bytes.NewReader([]byte("x")))
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(info.Key, ".bin"))
}
