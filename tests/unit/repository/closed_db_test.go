package repository_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"x-admin/internal/database"
	"x-admin/internal/model"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/repository"
)

// closedRepos 返回连接已关闭的仓储，用于覆盖数据库错误分支。
func closedRepos(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "closed.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, database.AutoMigrate(db))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	return db
}

func TestUserRepoErrors(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewUserRepo(closedRepos(t))
	user := &model.User{Username: "x", Password: "x"}
	_, err := repo.Get(ctx, 1)
	assert.Error(t, err)
	_, err = repo.FindByUsername(ctx, "x")
	assert.Error(t, err)
	_, _, err = repo.List(ctx, model.UserFilter{Query: pagination.Query{Page: 1, PageSize: 10}})
	assert.Error(t, err)
	assert.Error(t, repo.Create(ctx, user))
	assert.Error(t, repo.Update(ctx, user))
	assert.Error(t, repo.UpdateFields(ctx, 1, map[string]any{"nickname": "x"}))
	assert.Error(t, repo.SetPassword(ctx, 1, "h", true))
	assert.Error(t, repo.BumpSessionVersion(ctx, 1))
	_, _, err = repo.SessionState(ctx, 1)
	assert.Error(t, err)
	_, err = repo.ExistsUsername(ctx, "x", 0)
	assert.Error(t, err)
	assert.Error(t, repo.SetRoles(ctx, 1, []uint64{1}))
	assert.Error(t, repo.Delete(ctx, 1))
	_, err = repo.CountByStatus(ctx, nil)
	assert.Error(t, err)
}

func TestRoleAndMenuRepoErrors(t *testing.T) {
	ctx := context.Background()
	db := closedRepos(t)
	roles := repository.NewRoleRepo(db)
	menus := repository.NewMenuRepo(db)
	_, err := roles.Get(ctx, 1)
	assert.Error(t, err)
	_, err = roles.FindByCode(ctx, "x")
	assert.Error(t, err)
	_, _, err = roles.List(ctx, model.RoleFilter{Query: pagination.Query{Page: 1, PageSize: 10}})
	assert.Error(t, err)
	_, err = roles.All(ctx)
	assert.Error(t, err)
	_, err = roles.ExistsCode(ctx, "x", 0)
	assert.Error(t, err)
	_, err = roles.CountAll(ctx)
	assert.Error(t, err)
	assert.Error(t, roles.SetMenus(ctx, 1, []uint64{1}))
	_, err = roles.CountUsers(ctx, 1)
	assert.Error(t, err)
	assert.Error(t, roles.Delete(ctx, 1))

	_, err = menus.Get(ctx, 1)
	assert.Error(t, err)
	_, err = menus.All(ctx)
	assert.Error(t, err)
	_, err = menus.ForUser(ctx, 1)
	assert.Error(t, err)
	_, err = menus.MenuIDsByRole(ctx, 1)
	assert.Error(t, err)
	_, err = menus.CountChildren(ctx, 1)
	assert.Error(t, err)
	_, err = menus.CountAll(ctx)
	assert.Error(t, err)
	assert.Error(t, menus.Delete(ctx, 1))
}

func TestFileAndJobRepoErrors(t *testing.T) {
	ctx := context.Background()
	db := closedRepos(t)
	files := repository.NewFileRepo(db)
	jobs := repository.NewJobRepo(db)

	_, err := files.Get(ctx, 1)
	assert.Error(t, err)
	_, _, err = files.List(ctx, model.FileFilter{Query: pagination.Query{Page: 1, PageSize: 10}})
	assert.Error(t, err)
	_, err = files.TotalSize(ctx)
	assert.Error(t, err)
	_, err = files.CountAll(ctx)
	assert.Error(t, err)
	assert.Error(t, files.CreateShare(ctx, &model.FileShare{FileID: 1}))
	_, err = files.GetShareByHash(ctx, "x")
	assert.Error(t, err)
	_, err = files.ListShares(ctx, 1)
	assert.Error(t, err)
	assert.Error(t, files.RevokeShare(ctx, 1, 1))
	assert.Error(t, files.DeleteSharesByFile(ctx, 1))
	assert.Error(t, files.PruneShares(ctx, time.Now()))

	_, err = jobs.Get(ctx, 1)
	assert.Error(t, err)
	_, _, err = jobs.List(ctx, 1, 10)
	assert.Error(t, err)
	_, err = jobs.ListEnabled(ctx)
	assert.Error(t, err)
	_, err = jobs.CountAll(ctx)
	assert.Error(t, err)
	assert.Error(t, jobs.CreateRun(ctx, &model.JobRun{JobID: 1}))
	_, _, err = jobs.ListRuns(ctx, 1, 1, 10)
	assert.Error(t, err)
	assert.Error(t, jobs.PruneRuns(ctx, time.Now()))
	assert.Error(t, jobs.DeleteRunsByJob(ctx, 1))
}

func TestOtherRepoErrors(t *testing.T) {
	ctx := context.Background()
	db := closedRepos(t)

	depts := repository.NewDepartmentRepo(db)
	_, err := depts.Get(ctx, 1)
	assert.Error(t, err)
	_, err = depts.All(ctx)
	assert.Error(t, err)
	_, err = depts.CountChildren(ctx, 1)
	assert.Error(t, err)
	_, err = depts.CountUsers(ctx, 1)
	assert.Error(t, err)

	dicts := repository.NewDictRepo(db)
	_, err = dicts.GetType(ctx, 1)
	assert.Error(t, err)
	_, err = dicts.FindTypeByCode(ctx, "x")
	assert.Error(t, err)
	_, _, err = dicts.ListTypes(ctx, pagination.Query{Page: 1, PageSize: 10}, nil)
	assert.Error(t, err)
	_, err = dicts.Items(ctx, 1)
	assert.Error(t, err)
	_, err = dicts.GetItem(ctx, 1)
	assert.Error(t, err)
	assert.Error(t, dicts.CreateItem(ctx, &model.DictItem{}))
	assert.Error(t, dicts.UpdateItem(ctx, &model.DictItem{}))
	assert.Error(t, dicts.DeleteItem(ctx, 1))
	assert.Error(t, dicts.DeleteType(ctx, 1))

	params := repository.NewParameterRepo(db)
	_, err = params.Get(ctx, 1)
	assert.Error(t, err)
	_, err = params.GetByKey(ctx, "x")
	assert.Error(t, err)
	_, err = params.ListPublic(ctx)
	assert.Error(t, err)
	_, _, err = params.List(ctx, pagination.Query{Page: 1, PageSize: 10})
	assert.Error(t, err)
	_, err = params.ExistsKey(ctx, "x", 0)
	assert.Error(t, err)
	assert.Error(t, params.Delete(ctx, 1))

	logs := repository.NewLogRepo(db)
	_, _, err = logs.ListOperations(ctx, model.OperationLogFilter{Query: pagination.Query{Page: 1, PageSize: 10}})
	assert.Error(t, err)
	_, _, err = logs.ListLogins(ctx, model.LoginLogFilter{Query: pagination.Query{Page: 1, PageSize: 10}})
	assert.Error(t, err)
	assert.Error(t, logs.CreateOperation(ctx, &model.OperationLog{}))
	assert.Error(t, logs.CreateLogin(ctx, &model.LoginLog{}))
	assert.Error(t, logs.DeleteOperations(ctx, 1))
	assert.Error(t, logs.ClearOperations(ctx))
	assert.Error(t, logs.ClearLogins(ctx))
	_, err = logs.RecentOperations(ctx, 1)
	assert.Error(t, err)
	_, err = logs.LoginTrend(ctx, 7)
	assert.Error(t, err)
	assert.Error(t, logs.CleanBefore(ctx, 90))

	notifications := repository.NewNotificationRepo(db)
	assert.Error(t, notifications.Create(ctx, &model.Notification{}))
	_, _, err = notifications.List(ctx, 1, false, 1, 10)
	assert.Error(t, err)
	_, err = notifications.CountUnread(ctx, 1)
	assert.Error(t, err)
	assert.Error(t, notifications.MarkRead(ctx, 1, 1, time.Now()))
	_, err = notifications.MarkAllRead(ctx, 1, time.Now())
	assert.Error(t, err)
	assert.Error(t, notifications.CleanBefore(ctx, 90))

	passkeys := repository.NewPasskeyRepo(db)
	_, err = passkeys.ListByUser(ctx, 1)
	assert.Error(t, err)
	_, err = passkeys.GetByCredentialID(ctx, "x")
	assert.Error(t, err)
	_, err = passkeys.GetForUser(ctx, 1, 1)
	assert.Error(t, err)
	_, err = passkeys.CountByUser(ctx, 1)
	assert.Error(t, err)
	_, err = passkeys.CountAll(ctx)
	assert.Error(t, err)
	assert.Error(t, passkeys.RenameForUser(ctx, 1, 1, "x"))
	assert.Error(t, passkeys.DeleteForUser(ctx, 1, 1))
	assert.Error(t, passkeys.UpdateCredential(ctx, 1, "x", time.Now()))
}
