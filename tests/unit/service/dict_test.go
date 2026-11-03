package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func TestDictTypeAndItems(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)

	typ, err := app.Services.Dicts.CreateType(ctx, service.DictTypeInput{Name: "用户状态", Code: "user_status", Status: 1, Remark: "r"})
	require.NoError(t, err)
	assert.Equal(t, "user_status", typ.Code)

	types, total, err := app.Services.Dicts.ListTypes(ctx, pagination.Query{Page: 1, PageSize: 10}, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, types, 1)

	item, err := app.Services.Dicts.CreateItem(ctx, service.DictItemInput{
		DictTypeID: typ.ID, Label: "正常", Value: "1", Sort: 1, Status: 1, TagColor: "green",
	})
	require.NoError(t, err)
	assert.Equal(t, "1", item.Value)

	items, err := app.Services.Dicts.Items(ctx, typ.ID)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	updated, err := app.Services.Dicts.UpdateItem(ctx, item.ID, service.DictItemInput{
		DictTypeID: typ.ID, Label: "启用", Value: "1", Sort: 2, Status: 0, TagColor: "blue",
	})
	require.NoError(t, err)
	assert.Equal(t, "启用", updated.Label)

	updatedType, err := app.Services.Dicts.UpdateType(ctx, typ.ID, service.DictTypeInput{Name: "状态", Code: "user_status", Status: 0})
	require.NoError(t, err)
	assert.Equal(t, "状态", updatedType.Name)

	require.NoError(t, app.Services.Dicts.DeleteItem(ctx, item.ID))
	require.NoError(t, app.Services.Dicts.DeleteType(ctx, typ.ID))
}

func TestDictValidation(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	typ, err := app.Services.Dicts.CreateType(ctx, service.DictTypeInput{Name: "类型", Code: "kind", Status: 1})
	require.NoError(t, err)

	badTypes := []service.DictTypeInput{
		{Name: "", Code: "kind2", Status: 1},
		{Name: "X", Code: "Bad", Status: 1},
		{Name: "X", Code: "kind2", Status: 9},
	}
	for _, in := range badTypes {
		_, err := app.Services.Dicts.CreateType(ctx, in)
		assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	}
	_, err = app.Services.Dicts.CreateType(ctx, service.DictTypeInput{Name: "X", Code: "kind", Status: 1})
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	badItems := []service.DictItemInput{
		{DictTypeID: 0, Label: "L", Value: "V", Status: 1},
		{DictTypeID: typ.ID, Label: "", Value: "V", Status: 1},
		{DictTypeID: typ.ID, Label: "L", Value: "", Status: 1},
		{DictTypeID: typ.ID, Label: "L", Value: "V", Status: 5},
		{DictTypeID: 999, Label: "L", Value: "V", Status: 1},
	}
	for _, in := range badItems {
		_, err := app.Services.Dicts.CreateItem(ctx, in)
		assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	}

	_, err = app.Services.Dicts.UpdateType(ctx, 404, service.DictTypeInput{Name: "X", Code: "kind2", Status: 1})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	_, err = app.Services.Dicts.UpdateItem(ctx, 404, service.DictItemInput{DictTypeID: typ.ID, Label: "L", Value: "V", Status: 1})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	_, err = app.Services.Dicts.Items(ctx, 999)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	require.NoError(t, app.Services.Dicts.DeleteType(ctx, typ.ID))
	err = app.Services.Dicts.DeleteType(ctx, typ.ID)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	err = app.Services.Dicts.DeleteItem(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}
