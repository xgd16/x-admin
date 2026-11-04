package service_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/security"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func TestFileUploadListDownloadDelete(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	files := app.Services.Files

	file, err := files.Upload(ctx, "hello.txt", strings.NewReader("hello"), 1)
	require.NoError(t, err)
	assert.Equal(t, "hello.txt", file.Name)
	assert.Equal(t, int64(5), file.Size)
	assert.Equal(t, "txt", file.Ext)
	assert.Contains(t, file.MIME, "text/plain")
	assert.NotEmpty(t, file.URL)

	list, total, err := files.List(ctx, func() model.FileFilter {
		f := model.FileFilter{Query: paginationQuery()}
		f.Keyword = "hello"
		return f
	}())
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, list, 1)

	record, rc, err := files.Download(ctx, file.ID)
	require.NoError(t, err)
	data, _ := io.ReadAll(rc)
	require.NoError(t, rc.Close())
	assert.Equal(t, "hello", string(data))
	assert.Equal(t, file.Key, record.Key)

	require.NoError(t, files.Delete(ctx, file.ID))
	err = files.Delete(ctx, file.ID)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	_, _, err = files.Download(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestFileListFiltersByKind(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)

	fixtures := []model.File{
		{Name: "cover.png", Key: "cover.png", URL: "/uploads/cover.png", MIME: "image/png", Ext: "png"},
		{Name: "clip.mp4", Key: "clip.mp4", URL: "/uploads/clip.mp4", MIME: "video/mp4", Ext: "mp4"},
		{Name: "guide.pdf", Key: "guide.pdf", URL: "/uploads/guide.pdf", MIME: "application/pdf", Ext: "pdf"},
		{Name: "report.docx", Key: "report.docx", URL: "/uploads/report.docx", MIME: "application/zip", Ext: "docx"},
		{Name: "bundle.zip", Key: "bundle.zip", URL: "/uploads/bundle.zip", MIME: "application/zip", Ext: "zip"},
		{Name: "blob.bin", Key: "blob.bin", URL: "/uploads/blob.bin", MIME: "application/octet-stream", Ext: "bin"},
	}
	for i := range fixtures {
		require.NoError(t, app.Repos.Files.Create(ctx, &fixtures[i]))
	}

	names := func(kind string) []string {
		list, _, err := app.Services.Files.List(ctx, model.FileFilter{Query: paginationQuery(), Kind: kind})
		require.NoError(t, err)
		result := make([]string, len(list))
		for i, file := range list {
			result[i] = file.Name
		}
		return result
	}

	assert.ElementsMatch(t, []string{"cover.png"}, names("image"))
	assert.ElementsMatch(t, []string{"clip.mp4"}, names("video"))
	assert.ElementsMatch(t, []string{"guide.pdf", "report.docx"}, names("document"))
	assert.ElementsMatch(t, []string{"bundle.zip"}, names("archive"))
	assert.ElementsMatch(t, []string{"blob.bin"}, names("other"))
	assert.ElementsMatch(t, []string{"cover.png", "clip.mp4"}, names("image,video"))
	assert.Len(t, names("all"), len(fixtures))
}

func TestFileUploadLimits(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	files := app.Services.Files

	_, err := files.Upload(ctx, "", strings.NewReader("x"), 1)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	big := strings.NewReader(strings.Repeat("a", 1024*1024+1))
	_, err = files.Upload(ctx, "big.txt", big, 1)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	file, err := files.Upload(ctx, "lost.txt", strings.NewReader("data"), 1)
	require.NoError(t, err)
	require.NoError(t, app.Storage.Delete(ctx, file.Key))
	_, _, err = files.Download(ctx, file.ID)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

var _ = service.FileService{}

func TestFileShareLifecycle(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	files := app.Services.Files

	file, err := files.Upload(ctx, "share.txt", strings.NewReader("share"), 1)
	require.NoError(t, err)
	share, token, err := files.CreateShare(ctx, file.ID, 1, -1)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(time.Minute), share.ExpiresAt, 5*time.Second)
	require.NotEmpty(t, token)

	path, err := files.ResolveShare(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, file.URL, path)

	rows, err := files.Shares(ctx, file.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].RevokedAt)

	require.NoError(t, files.RevokeShare(ctx, file.ID, share.ID))
	_, err = files.ResolveShare(ctx, token)
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)
	err = files.RevokeShare(ctx, file.ID, share.ID)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	// 非法令牌、错误文件、空令牌。
	_, err = files.ResolveShare(ctx, "")
	assert.Equal(t, apperr.CodeUnauthorized, apperr.From(err).Code)
	_, err = files.ResolveShare(ctx, "missing")
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)
	_, _, err = files.CreateShare(ctx, 404, 1, time.Hour)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	_, err = files.Shares(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	_, err = files.Get(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestFileShareRejectsNonUploadURLAndClampsTTL(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	files := app.Services.Files

	external := &model.File{Name: "external", Key: "external", URL: "https://example.com/a.png"}
	require.NoError(t, app.Repos.Files.Create(ctx, external))
	_, _, err := files.CreateShare(ctx, external.ID, 1, time.Hour)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	file, err := files.Upload(ctx, "ttl.txt", strings.NewReader("x"), 1)
	require.NoError(t, err)
	share, _, err := files.CreateShare(ctx, file.ID, 1, 100*24*time.Hour)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), share.ExpiresAt, 5*time.Second)
}

func TestFileDeleteRemovesShares(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	files := app.Services.Files

	file, err := files.Upload(ctx, "delete-share.txt", strings.NewReader("x"), 1)
	require.NoError(t, err)
	share, token, err := files.CreateShare(ctx, file.ID, 1, time.Hour)
	require.NoError(t, err)
	require.NoError(t, files.Delete(ctx, file.ID))
	_, err = files.ResolveShare(ctx, token)
	assert.Equal(t, apperr.CodeTokenExpired, apperr.From(err).Code)
	_, err = app.Repos.Files.GetShareByHash(ctx, security.HashToken(token))
	assert.Error(t, err)
	_ = share
}
