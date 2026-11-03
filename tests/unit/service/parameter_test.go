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

func TestParameterCRUDAndPublicAccess(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)

	p, err := app.Services.Params.Create(ctx, service.ParameterInput{Key: "site.name", Value: "x-admin", Name: "站点名称", Public: true})
	require.NoError(t, err)
	assert.Equal(t, "site.name", p.Key)

	list, total, err := app.Services.Params.List(ctx, pagination.Query{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, list, 1)

	public, err := app.Services.Params.GetByKey(ctx, "site.name")
	require.NoError(t, err)
	assert.Equal(t, "x-admin", public.Value)

	private, err := app.Services.Params.Create(ctx, service.ParameterInput{Key: "secret.key", Value: "x", Name: "私有", Public: false})
	require.NoError(t, err)
	_, err = app.Services.Params.GetByKey(ctx, "secret.key")
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	_, err = app.Services.Params.GetByKey(ctx, "missing")
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	updated, err := app.Services.Params.Update(ctx, private.ID, service.ParameterInput{Key: "secret.key", Value: "y", Name: "私有2", Public: true})
	require.NoError(t, err)
	assert.Equal(t, "y", updated.Value)

	require.NoError(t, app.Services.Params.Delete(ctx, private.ID))
	err = app.Services.Params.Delete(ctx, private.ID)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestParameterValidation(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	_, err := app.Services.Params.Create(ctx, service.ParameterInput{Key: "bad key", Name: "X"})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = app.Services.Params.Create(ctx, service.ParameterInput{Key: "valid.key", Name: ""})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	_, err = app.Services.Params.Create(ctx, service.ParameterInput{Key: "valid.key", Name: "X"})
	require.NoError(t, err)
	_, err = app.Services.Params.Create(ctx, service.ParameterInput{Key: "valid.key", Name: "Y"})
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	_, err = app.Services.Params.Update(ctx, 404, service.ParameterInput{Key: "valid.key", Name: "Y"})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}
