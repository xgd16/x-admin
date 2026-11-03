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

// JobScheduler 抽象定时任务调度器，便于测试替换。
type JobScheduler interface {
	Validate(spec string) error
	Add(name, spec string, fn func()) error
	Remove(name string)
}

// JobInput 是创建/更新定时任务的输入。
// Status 为 nil 时创建默认启用、更新时保持原状态。
type JobInput struct {
	Name    string `json:"name"`
	Cron    string `json:"cron"`
	Handler string `json:"handler"`
	Params  string `json:"params"`
	Status  *int8  `json:"status"`
	Remark  string `json:"remark"`
}

// JobService 维护任务定义，并同步调度器的注册状态。
type JobService struct {
	jobs      JobRepository
	scheduler JobScheduler
	executor  func(ctx context.Context, job *model.Job) error
}

// NewJobService 创建任务服务。executor 由 bootstrap 注入具体 handler 分发逻辑。
func NewJobService(jobs JobRepository, scheduler JobScheduler, executor func(ctx context.Context, job *model.Job) error) *JobService {
	return &JobService{jobs: jobs, scheduler: scheduler, executor: executor}
}

// List 分页查询定时任务。
func (s *JobService) List(ctx context.Context, q pagination.Query) ([]model.Job, int64, error) {
	q.Normalize()
	return s.jobs.List(ctx, q.Page, q.PageSize)
}

// Create 创建任务；启用状态下立即注册调度。
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

// Update 更新任务并重建调度：先移除旧 entry，再按新状态决定是否注册。
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

// SetStatus 启用或停用任务，并同步调度器。
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

// Delete 删除任务并移除调度。
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

// RunNow 立即执行一次任务，不改变调度计划。
func (s *JobService) RunNow(ctx context.Context, id uint64) error {
	return s.Execute(ctx, id)
}

// Execute 执行任务并记录最后运行时间与结果。
// 执行失败会写入 last_result，但不向上返回错误，避免调度器重复重试。
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

// Boot 把所有启用任务注册到调度器，应用启动时调用。
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

// register 把任务注册到调度器；触发时使用独立 context，避免受请求生命周期影响。
func (s *JobService) register(job *model.Job) error {
	id := job.ID
	return s.scheduler.Add(jobKey(id), job.Cron, func() {
		_ = s.Execute(context.Background(), id)
	})
}

// validate 校验任务名称、执行器和 Cron 表达式。
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

// jobKey 生成调度器中的任务唯一名称。
func jobKey(id uint64) string { return "job:" + strconv.FormatUint(id, 10) }

// truncate 按字节截断字符串并去除尾部空白。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n])
}
