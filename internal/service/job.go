package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/pagination"
)

type JobScheduler interface {
	Validate(spec string) error
	Add(name, spec string, fn func()) error
	Remove(name string)
}

type JobInput struct {
	Name    string `json:"name"`
	Cron    string `json:"cron"`
	Handler string `json:"handler"`
	Params  string `json:"params"`
	Status  *int8  `json:"status"`
	Remark  string `json:"remark"`
}

type JobService struct {
	jobs      JobRepository
	scheduler JobScheduler
	executor  func(ctx context.Context, job *model.Job) error
}

func NewJobService(jobs JobRepository, scheduler JobScheduler, executor func(ctx context.Context, job *model.Job) error) *JobService {
	return &JobService{jobs: jobs, scheduler: scheduler, executor: executor}
}

func (s *JobService) List(ctx context.Context, q pagination.Query) ([]model.Job, int64, error) {
	q.Normalize()
	return s.jobs.List(ctx, q.Page, q.PageSize)
}

func (s *JobService) Create(ctx context.Context, in JobInput) (*model.Job, error) {
	if err := s.validate(in); err != nil {
		return nil, err
	}
	status := model.StatusEnabled
	if in.Status != nil {
		if err := ValidateStatus(*in.Status); err != nil {
			return nil, err
		}
		status = *in.Status
	}
	job := &model.Job{
		Name: trim(in.Name), Cron: trim(in.Cron), Handler: trim(in.Handler),
		Params: in.Params, Status: status, Remark: trim(in.Remark),
	}
	if err := s.jobs.Create(ctx, job); err != nil {
		return nil, err
	}
	if job.Status == model.StatusEnabled {
		if err := s.register(job); err != nil {
			return nil, err
		}
	}
	return job, nil
}

func (s *JobService) Update(ctx context.Context, id uint64, in JobInput) (*model.Job, error) {
	job, err := s.jobs.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if err := s.validate(in); err != nil {
		return nil, err
	}
	status := job.Status
	if in.Status != nil {
		if err := ValidateStatus(*in.Status); err != nil {
			return nil, err
		}
		status = *in.Status
	}
	job.Name = trim(in.Name)
	job.Cron = trim(in.Cron)
	job.Handler = trim(in.Handler)
	job.Params = in.Params
	job.Status = status
	job.Remark = trim(in.Remark)
	if err := s.jobs.Update(ctx, job); err != nil {
		return nil, err
	}
	s.scheduler.Remove(jobKey(id))
	if job.Status == model.StatusEnabled {
		if err := s.register(job); err != nil {
			return nil, err
		}
	}
	return job, nil
}

func (s *JobService) SetStatus(ctx context.Context, id uint64, status int8) error {
	if err := ValidateStatus(status); err != nil {
		return err
	}
	job, err := s.jobs.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	if err := s.jobs.UpdateFields(ctx, id, map[string]any{"status": status}); err != nil {
		return err
	}
	job.Status = status
	s.scheduler.Remove(jobKey(id))
	if status == model.StatusEnabled {
		return s.register(job)
	}
	return nil
}

func (s *JobService) Delete(ctx context.Context, id uint64) error {
	if _, err := s.jobs.Get(ctx, id); err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	s.scheduler.Remove(jobKey(id))
	return s.jobs.Delete(ctx, id)
}

func (s *JobService) RunNow(ctx context.Context, id uint64) error {
	return s.Execute(ctx, id)
}

func (s *JobService) Execute(ctx context.Context, id uint64) error {
	job, err := s.jobs.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	runAt := time.Now()
	result := "成功"
	if err := s.executor(ctx, job); err != nil {
		result = truncate(err.Error(), 255)
	}
	_ = s.jobs.UpdateFields(ctx, id, map[string]any{
		"last_run_at": runAt,
		"last_result": result,
	})
	return nil
}

func (s *JobService) Boot(ctx context.Context) error {
	jobs, err := s.jobs.ListEnabled(ctx)
	if err != nil {
		return err
	}
	for i := range jobs {
		if err := s.register(&jobs[i]); err != nil {
			return err
		}
	}
	return nil
}

func (s *JobService) register(job *model.Job) error {
	id := job.ID
	return s.scheduler.Add(jobKey(id), job.Cron, func() {
		_ = s.Execute(context.Background(), id)
	})
}

func (s *JobService) validate(in JobInput) error {
	if trim(in.Name) == "" {
		return apperr.New(apperr.CodeInvalidArgument, "任务名称不能为空")
	}
	if trim(in.Handler) == "" {
		return apperr.New(apperr.CodeInvalidArgument, "执行器不能为空")
	}
	if err := s.scheduler.Validate(trim(in.Cron)); err != nil {
		return apperr.New(apperr.CodeInvalidArgument, "Cron 表达式不合法")
	}
	return nil
}

func jobKey(id uint64) string { return "job:" + strconv.FormatUint(id, 10) }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n])
}
