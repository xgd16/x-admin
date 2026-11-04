package service_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/storage"
	"x-admin/internal/pkg/totp"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

type profileUpdateFieldsFail struct{ service.UserRepository }

func (f profileUpdateFieldsFail) UpdateFields(context.Context, uint64, map[string]any) error {
	return errRepo
}

type profileSetPasswordFail struct{ service.UserRepository }

func (f profileSetPasswordFail) SetPassword(context.Context, uint64, string, bool) error {
	return errRepo
}

type profileBumpFail struct{ service.UserRepository }

func (f profileBumpFail) BumpSessionVersion(context.Context, uint64) error { return errRepo }

func TestProfileServiceRepositoryFailures(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	profile := func(users service.UserRepository) *service.ProfileService {
		return service.NewProfileService(users, app.Hasher, 8, "x-admin", app.Services.Notifications)
	}

	_, err := profile(userGetFail{app.Repos.Users}).Get(ctx, admin.ID)
	assert.ErrorIs(t, err, errRepo)
	_, err = profile(profileUpdateFieldsFail{app.Repos.Users}).Update(ctx, admin.ID, service.ProfileInput{Nickname: "x"})
	assert.ErrorIs(t, err, errRepo)
	err = profile(profileSetPasswordFail{app.Repos.Users}).ChangePassword(ctx, admin.ID, "Admin@123456", "NewPass123")
	assert.ErrorIs(t, err, errRepo)
	_, err = profile(profileUpdateFieldsFail{app.Repos.Users}).SetupTOTP(ctx, admin.ID)
	assert.ErrorIs(t, err, errRepo)

	// 已写入密钥后，开启和关闭双重验证的数据库错误分支。
	setup, err := profile(app.Repos.Users).SetupTOTP(ctx, admin.ID)
	require.NoError(t, err)
	code, err := totp.Code(setup.Secret, time.Now())
	require.NoError(t, err)
	_, err = profile(profileUpdateFieldsFail{app.Repos.Users}).EnableTOTP(ctx, admin.ID, "Admin@123456", code)
	assert.ErrorIs(t, err, errRepo)

	require.NoError(t, app.Repos.Users.UpdateFields(ctx, admin.ID, map[string]any{
		"totp_enabled": true, "totp_secret": setup.Secret, "totp_recovery_codes": "[]",
	}))
	err = profile(profileBumpFail{app.Repos.Users}).DisableTOTP(ctx, admin.ID, "Admin@123456", code)
	assert.ErrorIs(t, err, errRepo)
}

type fileRepoFail struct{ service.FileRepository }

func (f fileRepoFail) List(context.Context, model.FileFilter) ([]model.File, int64, error) {
	return nil, 0, errRepo
}
func (f fileRepoFail) Create(context.Context, *model.File) error { return errRepo }
func (f fileRepoFail) CreateShare(context.Context, *model.FileShare) error {
	return errRepo
}
func (f fileRepoFail) ListShares(context.Context, uint64) ([]model.FileShare, error) {
	return nil, errRepo
}
func (f fileRepoFail) RevokeShare(context.Context, uint64, uint64) error { return errRepo }

type failStorage struct{ storage.Store }

func (f failStorage) Save(context.Context, string, io.Reader) (storage.Info, error) {
	return storage.Info{}, errRepo
}
func (f failStorage) Open(context.Context, string) (io.ReadCloser, error) { return nil, errRepo }
func (f failStorage) Delete(context.Context, string) error                { return errRepo }

func TestFileServiceFailures(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	file, err := app.Services.Files.Upload(ctx, "ok.txt", strings.NewReader("x"), 1)
	require.NoError(t, err)

	failing := service.NewFileService(fileRepoFail{app.Repos.Files}, app.Storage, 1)
	_, _, err = failing.List(ctx, model.FileFilter{})
	assert.ErrorIs(t, err, errRepo)
	_, err = failing.Upload(ctx, "x.txt", strings.NewReader("x"), 1)
	assert.ErrorIs(t, err, errRepo)
	_, _, err = failing.CreateShare(ctx, file.ID, 1, time.Hour)
	assert.ErrorIs(t, err, errRepo)
	_, err = failing.Shares(ctx, file.ID)
	assert.ErrorIs(t, err, errRepo)
	err = failing.RevokeShare(ctx, file.ID, 1)
	assert.ErrorIs(t, err, errRepo)

	storeFail := service.NewFileService(app.Repos.Files, failStorage{Store: app.Storage}, 1)
	_, err = storeFail.Upload(ctx, "x.txt", strings.NewReader("x"), 1)
	assert.ErrorIs(t, err, errRepo)
	_, _, err = storeFail.Download(ctx, file.ID)
	assert.ErrorIs(t, err, errRepo)
	err = storeFail.Delete(ctx, file.ID)
	assert.ErrorIs(t, err, errRepo)
}

type jobRepoFail struct{ service.JobRepository }

func (f jobRepoFail) ListEnabled(context.Context) ([]model.Job, error) { return nil, errRepo }
func (f jobRepoFail) DeleteRunsByJob(context.Context, uint64) error    { return errRepo }
func (f jobRepoFail) UpdateFields(context.Context, uint64, map[string]any) error {
	return errRepo
}
func (f jobRepoFail) CreateRun(context.Context, *model.JobRun) error { return errRepo }

func TestJobServiceRepositoryFailures(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	job := &model.Job{Name: "fail", Cron: "0 1 * * *", Handler: "noop", Status: 1}
	require.NoError(t, app.Repos.Jobs.Create(ctx, job))

	jobs := service.NewJobService(jobRepoFail{app.Repos.Jobs}, newFakeScheduler(), func(context.Context, *model.Job) error { return nil })
	require.NoError(t, jobs.Execute(ctx, job.ID))
	err := jobs.Delete(ctx, job.ID)
	assert.ErrorIs(t, err, errRepo)
	assert.ErrorIs(t, jobs.Boot(ctx), errRepo)
}

type passkeyRepoFail struct{ service.PasskeyRepository }

func (f passkeyRepoFail) ListByUser(context.Context, uint64) ([]model.Passkey, error) {
	return nil, errRepo
}
func (f passkeyRepoFail) CountByUser(context.Context, uint64) (int64, error) {
	return 0, errRepo
}
func (f passkeyRepoFail) CountAll(context.Context) (int64, error) { return 0, errRepo }
func (f passkeyRepoFail) GetForUser(context.Context, uint64, uint64) (*model.Passkey, error) {
	return nil, errRepo
}
func (f passkeyRepoFail) DeleteForUser(context.Context, uint64, uint64) error { return errRepo }
func (f passkeyRepoFail) RenameForUser(context.Context, uint64, uint64, string) error {
	return errRepo
}

func TestPasskeyServiceRepositoryFailures(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	admin := app.SeedAdmin(t)
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "test", RPID: "localhost", RPOrigins: []string{"http://localhost"},
	})
	require.NoError(t, err)
	pk := service.NewPasskeyService(app.Repos.Users, passkeyRepoFail{app.Repos.Passkeys}, app.Hasher,
		app.Cache, wa, time.Minute, app.Services.Notifications)

	_, err = pk.List(ctx, admin.ID)
	assert.ErrorIs(t, err, errRepo)
	_, err = pk.BeginRegistration(ctx, admin.ID, "Admin@123456")
	assert.ErrorIs(t, err, errRepo)
	_, err = pk.BeginLogin(ctx)
	assert.ErrorIs(t, err, errRepo)
	err = pk.Rename(ctx, admin.ID, 1, "设备")
	assert.ErrorIs(t, err, errRepo)
	err = pk.Delete(ctx, admin.ID, 1)
	assert.ErrorIs(t, err, errRepo)
}

func TestPasskeyServiceTakeCacheFailure(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	app.SeedAdmin(t)
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "test", RPID: "localhost", RPOrigins: []string{"http://localhost"},
	})
	require.NoError(t, err)
	pk := service.NewPasskeyService(app.Repos.Users, app.Repos.Passkeys, app.Hasher,
		failTakeStore{Store: app.Cache}, wa, time.Minute, app.Services.Notifications)
	_, err = pk.FinishRegistration(ctx, 1, "sid", "设备", nil)
	assert.ErrorIs(t, err, errRepo)
	_, err = pk.FinishLogin(ctx, "sid", nil)
	assert.ErrorIs(t, err, errRepo)
}

type failTakeStore struct{ cache.Store }

func (f failTakeStore) Get(context.Context, string) (string, error)  { return "", errRepo }
func (f failTakeStore) Take(context.Context, string) (string, error) { return "", errRepo }
