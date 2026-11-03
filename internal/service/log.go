package service

import (
	"context"
	"sync"
	"time"

	"x-admin/internal/model"
)

// LogService 通过有界队列异步写入审计日志，
// 请求延迟不依赖日志落库速度；队列满时丢弃而不是阻塞请求。
type LogService struct {
	repo  LogRepository
	queue chan *model.OperationLog
	done  chan struct{}
	wg    sync.WaitGroup
	once  sync.Once
	mu    sync.RWMutex
	ended bool
}

// NewLogService 创建日志服务并启动后台 worker。buffer <= 0 时默认 1024。
func NewLogService(repo LogRepository, buffer int) *LogService {
	if buffer <= 0 {
		buffer = 1024
	}
	s := &LogService{
		repo:  repo,
		queue: make(chan *model.OperationLog, buffer),
		done:  make(chan struct{}),
	}
	s.wg.Add(1)
	go s.worker()
	return s
}

// worker 持续消费队列，每条日志使用独立的 5 秒超时上下文。
func (s *LogService) worker() {
	defer s.wg.Done()
	for log := range s.queue {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = s.repo.CreateOperation(ctx, log)
		cancel()
	}
	close(s.done)
}

// RecordOperation 非阻塞地投递审计记录；队列已满或服务已关闭时直接丢弃。
func (s *LogService) RecordOperation(log *model.OperationLog) {
	if log == nil {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.ended {
		return
	}
	select {
	case s.queue <- log:
	default:
	}
}

// Close 关闭队列并等待 worker 把剩余日志写完，重复调用安全。
func (s *LogService) Close() {
	s.once.Do(func() {
		s.mu.Lock()
		s.ended = true
		close(s.queue)
		s.mu.Unlock()
		s.wg.Wait()
	})
}

// Operations 分页查询操作日志。
func (s *LogService) Operations(ctx context.Context, f model.OperationLogFilter) ([]model.OperationLog, int64, error) {
	f.Normalize()
	return s.repo.ListOperations(ctx, f)
}

// DeleteOperations 按 ID 批量删除操作日志。
func (s *LogService) DeleteOperations(ctx context.Context, ids ...uint64) error {
	return s.repo.DeleteOperations(ctx, ids...)
}

// ClearOperations 清空操作日志。
func (s *LogService) ClearOperations(ctx context.Context) error {
	return s.repo.ClearOperations(ctx)
}

// Logins 分页查询登录日志。
func (s *LogService) Logins(ctx context.Context, f model.LoginLogFilter) ([]model.LoginLog, int64, error) {
	f.Normalize()
	return s.repo.ListLogins(ctx, f)
}

// ClearLogins 清空登录日志。
func (s *LogService) ClearLogins(ctx context.Context) error {
	return s.repo.ClearLogins(ctx)
}
