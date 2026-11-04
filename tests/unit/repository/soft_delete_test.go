package repository_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/tests/testsupport"
)

// TestSoftDeleteFreesUniqueKeys 覆盖删除后重建同名记录的场景。
// 业务层的存在性检查会自动排除软删除行，数据库唯一索引也必须同步释放。
func TestSoftDeleteFreesUniqueKeys(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)

	t.Run("user", func(t *testing.T) {
		user := &model.User{Username: "reuse_me", Password: "x", Status: model.StatusEnabled}
		require.NoError(t, app.Repos.Users.Create(ctx, user))
		require.NoError(t, app.Repos.Users.Delete(ctx, user.ID))

		exists, err := app.Repos.Users.ExistsUsername(ctx, "reuse_me", 0)
		require.NoError(t, err)
		require.False(t, exists)

		recreated := &model.User{Username: "reuse_me", Password: "y", Status: model.StatusEnabled}
		require.NoError(t, app.Repos.Users.Create(ctx, recreated))
		require.NotEqual(t, user.ID, recreated.ID)
	})

	t.Run("role", func(t *testing.T) {
		role := &model.Role{Name: "运营", Code: "reuse_role", Status: model.StatusEnabled}
		require.NoError(t, app.Repos.Roles.Create(ctx, role))
		require.NoError(t, app.Repos.Roles.Delete(ctx, role.ID))

		recreated := &model.Role{Name: "运营 2", Code: "reuse_role", Status: model.StatusEnabled}
		require.NoError(t, app.Repos.Roles.Create(ctx, recreated))
	})

	t.Run("dict type", func(t *testing.T) {
		dict := &model.DictType{Name: "状态", Code: "reuse_dict", Status: model.StatusEnabled}
		require.NoError(t, app.Repos.Dicts.CreateType(ctx, dict))
		require.NoError(t, app.Repos.Dicts.DeleteType(ctx, dict.ID))

		recreated := &model.DictType{Name: "状态 2", Code: "reuse_dict", Status: model.StatusEnabled}
		require.NoError(t, app.Repos.Dicts.CreateType(ctx, recreated))
	})

	t.Run("parameter", func(t *testing.T) {
		param := &model.Parameter{Key: "reuse.key", Name: "参数", Value: "1"}
		require.NoError(t, app.Repos.Params.Create(ctx, param))
		require.NoError(t, app.Repos.Params.Delete(ctx, param.ID))

		recreated := &model.Parameter{Key: "reuse.key", Name: "参数 2", Value: "2"}
		require.NoError(t, app.Repos.Params.Create(ctx, recreated))
	})
}
