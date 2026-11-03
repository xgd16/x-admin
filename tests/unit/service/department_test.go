package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func TestDepartmentCRUD(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)

	root, err := app.Services.Depts.Create(ctx, service.DepartmentInput{Name: "总公司", Status: 1, Sort: 1})
	require.NoError(t, err)
	child, err := app.Services.Depts.Create(ctx, service.DepartmentInput{Name: "技术部", ParentID: root.ID, Leader: "陈", Phone: "13800000000", Email: "a@b.com", Status: 1})
	require.NoError(t, err)

	tree, err := app.Services.Depts.Tree(ctx)
	require.NoError(t, err)
	require.Len(t, tree, 1)
	assert.Len(t, tree[0].Children, 1)

	updated, err := app.Services.Depts.Update(ctx, child.ID, service.DepartmentInput{Name: "研发部", ParentID: root.ID, Status: 1})
	require.NoError(t, err)
	assert.Equal(t, "研发部", updated.Name)

	_, err = app.Services.Depts.Update(ctx, root.ID, service.DepartmentInput{Name: "总公司", ParentID: child.ID, Status: 1})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = app.Services.Depts.Update(ctx, root.ID, service.DepartmentInput{Name: "总公司", ParentID: root.ID, Status: 1})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	_, err = app.Services.Depts.Update(ctx, 404, service.DepartmentInput{Name: "X", Status: 1})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	err = app.Services.Depts.Delete(ctx, root.ID)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	err = app.Services.Depts.Delete(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	app.CreateUser(t, "alice", "Passw0rd", model.StatusEnabled)
	require.NoError(t, app.Repos.Users.UpdateFields(ctx, 1, map[string]any{"dept_id": child.ID}))
	err = app.Services.Depts.Delete(ctx, child.ID)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	require.NoError(t, app.Repos.Users.UpdateFields(ctx, 1, map[string]any{"dept_id": nil}))
	require.NoError(t, app.Services.Depts.Delete(ctx, child.ID))
	require.NoError(t, app.Services.Depts.Delete(ctx, root.ID))
}

func TestDepartmentValidation(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	cases := []service.DepartmentInput{
		{Name: "", Status: 1},
		{Name: "X", Email: "bad", Status: 1},
		{Name: "X", Phone: "bad", Status: 1},
		{Name: "X", Status: 4},
		{Name: "X", ParentID: 999, Status: 1},
	}
	for _, in := range cases {
		_, err := app.Services.Depts.Create(ctx, in)
		require.Error(t, err)
		assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	}
}
