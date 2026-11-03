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

func TestMenuCreateAndTree(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)

	dir, err := app.Services.Menus.Create(ctx, service.MenuInput{Name: "系统管理", Type: model.MenuDirectory, Status: 1, Visible: true, Sort: 1})
	require.NoError(t, err)
	menu, err := app.Services.Menus.Create(ctx, service.MenuInput{
		Name: "用户管理", Type: model.MenuMenu, ParentID: dir.ID, Path: "/users", Component: "users", Permission: "system:user:list", Status: 1, Visible: true,
	})
	require.NoError(t, err)
	button, err := app.Services.Menus.Create(ctx, service.MenuInput{
		Name: "新建", Type: model.MenuButton, ParentID: menu.ID, Permission: "system:user:create", Status: 1, Visible: true,
	})
	require.NoError(t, err)
	assert.Equal(t, model.MenuButton, button.Type)

	tree, err := app.Services.Menus.Tree(ctx)
	require.NoError(t, err)
	require.Len(t, tree, 1)
	assert.Equal(t, "系统管理", tree[0].Name)
	require.Len(t, tree[0].Children, 1)
	assert.Equal(t, "用户管理", tree[0].Children[0].Name)
	require.Len(t, tree[0].Children[0].Children, 1)
}

func TestMenuValidation(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	parent := &model.Menu{Name: "P", Type: model.MenuDirectory, Status: 1, Visible: true}
	require.NoError(t, app.Repos.Menus.Create(ctx, parent))

	cases := []service.MenuInput{
		{Name: "", Type: model.MenuMenu, Path: "/x", Status: 1},
		{Name: "X", Type: 0, Status: 1},
		{Name: "X", Type: model.MenuMenu, Status: 1},
		{Name: "X", Type: model.MenuButton, Status: 1},
		{Name: "X", Type: model.MenuButton, Permission: "bad", Status: 1},
		{Name: "X", Type: model.MenuMenu, Path: "/x", ParentID: 999, Status: 1},
		{Name: "X", Type: model.MenuMenu, Path: "/x", Status: 3},
	}
	for _, in := range cases {
		_, err := app.Services.Menus.Create(ctx, in)
		require.Error(t, err)
		assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	}
}

func TestMenuUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	parent, err := app.Services.Menus.Create(ctx, service.MenuInput{Name: "P", Type: model.MenuDirectory, Status: 1, Visible: true})
	require.NoError(t, err)
	child, err := app.Services.Menus.Create(ctx, service.MenuInput{Name: "C", Type: model.MenuMenu, ParentID: parent.ID, Path: "/c", Status: 1, Visible: true})
	require.NoError(t, err)

	err = app.Services.Menus.Delete(ctx, parent.ID)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	_, err = app.Services.Menus.Update(ctx, parent.ID, service.MenuInput{Name: "P", Type: model.MenuDirectory, ParentID: parent.ID, Status: 1})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	_, err = app.Services.Menus.Update(ctx, parent.ID, service.MenuInput{Name: "P", Type: model.MenuDirectory, ParentID: child.ID, Status: 1})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)

	updated, err := app.Services.Menus.Update(ctx, child.ID, service.MenuInput{
		Name: "C2", Type: model.MenuMenu, ParentID: parent.ID, Path: "/c2", Permission: "system:menu:list", Status: 1, Visible: false,
	})
	require.NoError(t, err)
	assert.Equal(t, "C2", updated.Name)
	assert.False(t, updated.Visible)

	_, err = app.Services.Menus.Update(ctx, 404, service.MenuInput{Name: "X", Type: model.MenuMenu, Path: "/x", Status: 1})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	err = app.Services.Menus.Delete(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	require.NoError(t, app.Services.Menus.Delete(ctx, child.ID))
	require.NoError(t, app.Services.Menus.Delete(ctx, parent.ID))
}
