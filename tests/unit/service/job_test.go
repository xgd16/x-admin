package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
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
	require.NoError(t, jobs.Execute(ctx, job.ID))
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
