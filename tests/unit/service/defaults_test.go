package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func TestServiceConstructorDefaults(t *testing.T) {
	app := testsupport.New(t)
	users := app.Repos.Users
	roles := app.Repos.Roles
	menus := app.Repos.Menus
	depts := app.Repos.Depts
	files := app.Repos.Files
	notifier := app.Services.Notifications

	assert.NotNil(t, service.NewUserService(users, roles, menus, depts, app.Hasher, 0, notifier))
	assert.NotNil(t, service.NewProfileService(users, app.Hasher, 0, "", notifier))
	assert.NotNil(t, service.NewRoleService(roles, menus))
	assert.NotNil(t, service.NewFileService(files, app.Storage, 0))
	assert.NotNil(t, service.NewPasskeyService(users, app.Repos.Passkeys, app.Hasher, app.Cache, nil, 0, notifier))
	assert.NotNil(t, service.NewAuthService(users, menus, app.Repos.Logs, app.Cache, nil, app.Hasher, nil, notifier, service.AuthOptions{}))
	assert.NotNil(t, service.NewLogService(app.Repos.Logs, 0))
}

type failExistsStore struct{ cache.Store }

func (f failExistsStore) Exists(context.Context, string) (bool, error) {
	return false, errors.New("exists failed")
}

type failDelStore struct{ cache.Store }

func (f failDelStore) Del(context.Context, ...string) error { return errors.New("del failed") }

func TestSessionCacheFailures(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	app.SeedAdmin(t)
	broken := service.NewAuthService(app.Repos.Users, app.Repos.Menus, app.Repos.Logs,
		failExistsStore{Store: app.Cache}, app.Tokens, app.Hasher, stubCaptcha{ok: true},
		app.Services.Notifications, service.AuthOptions{})

	_, err := broken.SessionActive(ctx, 1, "sid")
	require.Error(t, err)

	require.NoError(t, app.Cache.Set(ctx, "session:1:a", "{}", time.Hour))
	require.NoError(t, app.Cache.Set(ctx, "session:1:b", "{}", time.Hour))
	failingDel := service.NewAuthService(app.Repos.Users, app.Repos.Menus, app.Repos.Logs,
		failDelStore{Store: app.Cache}, app.Tokens, app.Hasher, stubCaptcha{ok: true},
		app.Services.Notifications, service.AuthOptions{})
	_, err = failingDel.RevokeOtherSessions(ctx, 1, "a")
	require.Error(t, err)
	_, err = failingDel.RevokeAllSessions(ctx, 1)
	require.Error(t, err)
}

func TestJobBootRegistrationFailure(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	job := &model.Job{Name: "boot", Cron: "0 1 * * *", Handler: "noop", Status: 1}
	require.NoError(t, app.Repos.Jobs.Create(ctx, job))
	sched := newFakeScheduler()
	sched.addErr = errors.New("add failed")
	jobs := service.NewJobService(app.Repos.Jobs, sched, func(context.Context, *model.Job) error { return nil })
	assert.Error(t, jobs.Boot(ctx))
}
