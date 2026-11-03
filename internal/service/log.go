package service

import (
	"context"
	"sync"
	"time"

	"x-admin/internal/model"
)

// LogService writes audit records through a bounded queue so request latency
// never depends on log persistence.
type LogService struct {
	repo  LogRepository
	queue chan *model.OperationLog
	done  chan struct{}
	wg    sync.WaitGroup
	once  sync.Once
	mu    sync.RWMutex
	ended bool
}

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

func (s *LogService) worker() {
	defer s.wg.Done()
	for log := range s.queue {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = s.repo.CreateOperation(ctx, log)
		cancel()
	}
	close(s.done)
}

// RecordOperation never blocks: when the queue is full the audit record is
// dropped rather than slowing the request.
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

func (s *LogService) Close() {
	s.once.Do(func() {
		s.mu.Lock()
		s.ended = true
		close(s.queue)
		s.mu.Unlock()
		s.wg.Wait()
	})
}

func (s *LogService) Operations(ctx context.Context, f model.OperationLogFilter) ([]model.OperationLog, int64, error) {
	f.Normalize()
	return s.repo.ListOperations(ctx, f)
}

func (s *LogService) DeleteOperations(ctx context.Context, ids ...uint64) error {
	return s.repo.DeleteOperations(ctx, ids...)
}

func (s *LogService) ClearOperations(ctx context.Context) error {
	return s.repo.ClearOperations(ctx)
}

func (s *LogService) Logins(ctx context.Context, f model.LoginLogFilter) ([]model.LoginLog, int64, error) {
	f.Normalize()
	return s.repo.ListLogins(ctx, f)
}

func (s *LogService) ClearLogins(ctx context.Context) error {
	return s.repo.ClearLogins(ctx)
}
