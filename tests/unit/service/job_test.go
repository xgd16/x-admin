package service_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

type fakeScheduler struct {
	validateErr error
	addErr      error
	added       map[string]string
	removed     []string
}

func newFakeScheduler() *fakeScheduler {
	return &fakeScheduler{added: make(map[string]string)}
}

func (f *fakeScheduler) Validate(string) error { return f.validateErr }
func (f *fakeScheduler) Add(name, spec string, _ func()) error {
	if f.addErr != nil {
		return f.addErr
	}
	f.added[name] = spec
	return nil
}
func (f *fakeScheduler) Remove(name string) { f.removed = append(f.removed, name) }

func newJobService(app *testsupport.App, scheduler service.JobScheduler, exec func(context.Context, *model.Job) error) *service.JobService {
	return service.NewJobService(app.Repos.Jobs, scheduler, exec)
}

func TestJobCRUDAndScheduling(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	sched := newFakeScheduler()
	jobs := newJobService(app, sched, func(context.Context, *model.Job) error { return nil })

	job, err := jobs.Create(ctx, service.JobInput{Name: "清理", Cron: "0 3 * * *", Handler: "clean_logs", Params: `{"days":7}`})
	require.NoError(t, err)
	assert.Equal(t, int8(1), job.Status)
	assert.Equal(t, "0 3 * * *", sched.added["job:1"])

	disabled := int8(0)
	job2, err := jobs.Create(ctx, service.JobInput{Name: "停用", Cron: "0 4 * * *", Handler: "noop", Status: &disabled})
	require.NoError(t, err)
	assert.Equal(t, int8(0), job2.Status)
	assert.NotContains(t, sched.added, "job:2")

	updated, err := jobs.Update(ctx, job.ID, service.JobInput{Name: "清理2", Cron: "0 5 * * *", Handler: "clean_logs", Status: &disabled})
	require.NoError(t, err)
	assert.Equal(t, "清理2", updated.Name)
	assert.Contains(t, sched.removed, "job:1")

	enabled := int8(1)
	_, err = jobs.Update(ctx, job.ID, service.JobInput{Name: "清理2", Cron: "0 5 * * *", Handler: "clean_logs", Status: &enabled})
	require.NoError(t, err)
	assert.Equal(t, "0 5 * * *", sched.added["job:1"])

	require.NoError(t, jobs.SetStatus(ctx, job.ID, 0))
	assert.Equal(t, 3, len(sched.removed))
	require.NoError(t, jobs.SetStatus(ctx, job.ID, 1))
	require.NoError(t, jobs.RunNow(ctx, job.ID))

	require.NoError(t, jobs.Delete(ctx, job.ID))
	err = jobs.Delete(ctx, job.ID)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestJobValidationAndErrors(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	sched := newFakeScheduler()
	jobs := newJobService(app, sched, func(context.Context, *model.Job) error { return nil })

	bad := []service.JobInput{
		{Name: "", Cron: "0 1 * * *", Handler: "noop"},
		{Name: "X", Cron: "0 1 * * *", Handler: ""},
	}
	for _, in := range bad {
		_, err := jobs.Create(ctx, in)
		assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	}

	sched.validateErr = errors.New("bad cron")
	_, err := jobs.Create(ctx, service.JobInput{Name: "X", Cron: "bad", Handler: "noop"})
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	sched.validateErr = nil

	sched.addErr = errors.New("scheduler down")
	_, err = jobs.Create(ctx, service.JobInput{Name: "X", Cron: "0 1 * * *", Handler: "noop"})
	assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)

	_, err = jobs.Update(ctx, 404, service.JobInput{Name: "X", Cron: "0 1 * * *", Handler: "noop"})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	err = jobs.SetStatus(ctx, 404, 1)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	err = jobs.SetStatus(ctx, 1, 9)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	err = jobs.RunNow(ctx, 404)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestJobExecuteRecordsFailure(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	sched := newFakeScheduler()
	job := &model.Job{Name: "boom", Cron: "0 1 * * *", Handler: "noop", Status: 1}
	require.NoError(t, app.Repos.Jobs.Create(ctx, job))
	jobs := newJobService(app, sched, func(context.Context, *model.Job) error {
		return errors.New(strings.Repeat("x", 300))
	})
	err := jobs.Execute(ctx, job.ID)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
	got, err := app.Repos.Jobs.Get(ctx, job.ID)
	require.NoError(t, err)
	assert.Len(t, got.LastResult, 255)
	assert.NotNil(t, got.LastRunAt)
}

func TestJobBootAndList(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	require.NoError(t, app.Repos.Jobs.Create(ctx, &model.Job{Name: "a", Cron: "0 1 * * *", Handler: "noop", Status: 1}))
	require.NoError(t, app.Repos.Jobs.Create(ctx, &model.Job{Name: "b", Cron: "0 2 * * *", Handler: "noop", Status: 0}))

	sched := newFakeScheduler()
	jobs := newJobService(app, sched, func(context.Context, *model.Job) error { return nil })
	require.NoError(t, jobs.Boot(ctx))
	assert.Len(t, sched.added, 1)

	list, total, err := jobs.List(ctx, pagination.Query{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)
}

func TestJobRunsHistoryAndCleanup(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	sched := newFakeScheduler()
	job := &model.Job{Name: "history", Cron: "0 1 * * *", Handler: "noop", Status: 1}
	require.NoError(t, app.Repos.Jobs.Create(ctx, job))
	jobs := newJobService(app, sched, func(context.Context, *model.Job) error { return nil })

	require.NoError(t, jobs.RunNow(ctx, job.ID))
	require.NoError(t, jobs.Execute(ctx, job.ID, "cron"))

	rows, total, err := jobs.Runs(ctx, job.ID, pagination.Query{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	require.Len(t, rows, 2)
	triggers := []string{rows[0].Trigger, rows[1].Trigger}
	assert.ElementsMatch(t, []string{"manual", "cron"}, triggers)
	assert.Equal(t, int8(model.StatusEnabled), rows[0].Status)
	assert.NotEmpty(t, rows[0].JobName)

	// 不存在的任务返回 404。
	_, _, err = jobs.Runs(ctx, 404, pagination.Query{Page: 1, PageSize: 10})
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	// 删除任务时执行记录一并清理。
	require.NoError(t, jobs.Delete(ctx, job.ID))
	_, total, err = app.Repos.Jobs.ListRuns(ctx, job.ID, 1, 10)
	require.NoError(t, err)
	assert.Zero(t, total)
}

func TestJobExecuteFailureWritesRun(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	sched := newFakeScheduler()
	job := &model.Job{Name: "boom-run", Cron: "0 1 * * *", Handler: "noop", Status: 1}
	require.NoError(t, app.Repos.Jobs.Create(ctx, job))
	jobs := newJobService(app, sched, func(context.Context, *model.Job) error {
		return errors.New("执行器异常")
	})
	require.Error(t, jobs.Execute(ctx, job.ID, "manual"))
	rows, total, err := app.Repos.Jobs.ListRuns(ctx, job.ID, 1, 10)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, int8(model.StatusDisabled), rows[0].Status)
	assert.Contains(t, rows[0].Result, "执行器异常")
	assert.Equal(t, "manual", rows[0].Trigger)
}

func TestJobCrossReplicaExecutionLock(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	sched := newFakeScheduler()
	job := &model.Job{Name: "locked", Cron: "0 1 * * *", Handler: "noop", Status: 1}
	require.NoError(t, app.Repos.Jobs.Create(ctx, job))
	calls := 0
	jobs := service.NewJobService(app.Repos.Jobs, sched, func(context.Context, *model.Job) error {
		calls++
		return nil
	}, app.Cache)

	// 模拟另一个副本持有锁：手动执行返回冲突，Cron 触发直接跳过。
	require.NoError(t, app.Cache.Set(ctx, "job:lock:"+fmt.Sprint(job.ID), "1", time.Minute))
	err := jobs.RunNow(ctx, job.ID)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	require.NoError(t, jobs.Execute(ctx, job.ID, "cron"))
	assert.Zero(t, calls)

	// 锁释放后可以正常执行，并在结束后自动释放。
	require.NoError(t, app.Cache.Del(ctx, "job:lock:"+fmt.Sprint(job.ID)))
	require.NoError(t, jobs.RunNow(ctx, job.ID))
	assert.Equal(t, 1, calls)
	exists, err := app.Cache.Exists(ctx, "job:lock:"+fmt.Sprint(job.ID))
	require.NoError(t, err)
	assert.False(t, exists)
}

type failLockStore struct{ cache.Store }

func (f failLockStore) SetNX(context.Context, string, string, time.Duration) (bool, error) {
	return false, errors.New("lock unavailable")
}

func TestJobRunsAcrossAllJobsAndLockFailureFallback(t *testing.T) {
	ctx := context.Background()
	app := testsupport.New(t)
	sched := newFakeScheduler()
	first := &model.Job{Name: "first", Cron: "0 1 * * *", Handler: "noop", Status: 1}
	second := &model.Job{Name: "second", Cron: "0 2 * * *", Handler: "noop", Status: 1}
	require.NoError(t, app.Repos.Jobs.Create(ctx, first))
	require.NoError(t, app.Repos.Jobs.Create(ctx, second))

	calls := 0
	jobs := service.NewJobService(app.Repos.Jobs, sched, func(context.Context, *model.Job) error {
		calls++
		return nil
	}, failLockStore{Store: app.Cache})

	// 缓存锁故障时退化为继续执行，而不是让任务全部停摆。
	require.NoError(t, jobs.RunNow(ctx, first.ID))
	require.NoError(t, jobs.RunNow(ctx, second.ID))
	assert.Equal(t, 2, calls)

	rows, total, err := jobs.Runs(ctx, 0, pagination.Query{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, rows, 2)
}
