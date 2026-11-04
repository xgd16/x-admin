package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/cache"
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
	locker    cache.Store // 可选：多副本部署时用于抢占执行权
}

// NewJobService 创建任务服务。executor 由 bootstrap 注入具体 handler 分发逻辑。
// locker 可选；传入缓存后，同一任务在多个副本间只允许一个实例执行。
func NewJobService(jobs JobRepository, scheduler JobScheduler, executor func(ctx context.Context, job *model.Job) error, lockers ...cache.Store) *JobService {
	svc := &JobService{jobs: jobs, scheduler: scheduler, executor: executor}
	if len(lockers) > 0 {
		svc.locker = lockers[0]
	}
	return svc
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
	if err := s.jobs.DeleteRunsByJob(ctx, id); err != nil {
		return err
	}
	return s.jobs.Delete(ctx, id)
}

// RunNow 立即执行一次任务，不改变调度计划。
func (s *JobService) RunNow(ctx context.Context, id uint64) error {
	return s.Execute(ctx, id, "manual")
}

// Runs 分页查询任务执行历史；jobID 为 0 时返回全部任务。
func (s *JobService) Runs(ctx context.Context, jobID uint64, q pagination.Query) ([]model.JobRun, int64, error) {
	if jobID > 0 {
		if _, err := s.jobs.Get(ctx, jobID); err != nil {
			if IsNotFound(err) {
				return nil, 0, apperr.ErrNotFound
			}
			return nil, 0, err
		}
	}
	q.Normalize()
	return s.jobs.ListRuns(ctx, jobID, q.Page, q.PageSize)
}

// Execute 执行任务并记录最后运行时间、结果与一条执行历史。
// 执行失败会写入 last_result，并返回错误给手动执行接口；
// 调度器回调会忽略该错误，避免同一周期重复重试。
func (s *JobService) Execute(ctx context.Context, id uint64, trigger ...string) error {
	job, err := s.jobs.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return apperr.ErrNotFound
		}
		return err
	}
	triggerName := triggerValue(trigger)
	// 多副本部署时通过缓存锁保证同一任务只有一个实例执行。
	if s.locker != nil {
		lockKey := jobLockKey(id)
		acquired, err := s.locker.SetNX(ctx, lockKey, "1", jobLockTTL)
		if err != nil {
			// 缓存故障时不让所有任务停摆，退化为单副本内的 SkipIfStillRunning。
			slog.Warn("acquire job lock failed", "job_id", id, "error", err)
		} else if !acquired {
			if triggerName == "manual" {
				return apperr.New(apperr.CodeConflict, "任务正在执行中，请稍后重试")
			}
			slog.Info("skip overlapping job", "job_id", id)
			return nil
		} else {
			defer func() { _ = s.locker.Del(context.Background(), lockKey) }()
		}
	}
	started := time.Now()
	result := "成功"
	status := model.StatusEnabled
	execErr := s.executor(ctx, job)
	if execErr != nil {
		status = model.StatusDisabled
		result = truncate(execErr.Error(), 255)
	}
	finished := time.Now()
	_ = s.jobs.UpdateFields(ctx, id, map[string]any{
		"last_run_at": started,
		"last_result": result,
	})
	run := &model.JobRun{
		JobID: job.ID, JobName: job.Name, Trigger: triggerName,
		Status: status, Result: result,
		StartedAt: started, FinishedAt: finished,
		DurationMS: finished.Sub(started).Milliseconds(),
	}
	if err := s.jobs.CreateRun(ctx, run); err != nil {
		slog.Warn("record job run failed", "job_id", id, "error", err)
	}
	// 历史只保留 30 天，避免高频任务把表撑大。
	if err := s.jobs.PruneRuns(ctx, started.AddDate(0, 0, -30)); err != nil {
		slog.Warn("prune job runs failed", "error", err)
	}
	if execErr != nil {
		return apperr.Wrap(execErr, apperr.CodeInternal, "任务执行失败")
	}
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
		_ = s.Execute(context.Background(), id, "cron")
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

// triggerValue 归一化执行来源，默认按调度触发处理。
func triggerValue(values []string) string {
	if len(values) == 0 || strings.TrimSpace(values[0]) == "" {
		return "cron"
	}
	if values[0] == "manual" {
		return "manual"
	}
	return "cron"
}

// jobLockTTL 是跨副本执行锁的兜底过期时间，避免进程崩溃后永久占用。
const jobLockTTL = 30 * time.Minute

// jobLockKey 生成任务执行锁的缓存键。
func jobLockKey(id uint64) string { return "job:lock:" + strconv.FormatUint(id, 10) }

// jobKey 生成调度器中的任务唯一名称。
func jobKey(id uint64) string { return "job:" + strconv.FormatUint(id, 10) }

// truncate 按字节截断字符串并去除尾部空白，同时避免切断多字节字符。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return strings.TrimSpace(s[:n])
}
