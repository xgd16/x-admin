package service_test

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
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
