package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/excel"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/pkg/storage"
)

// ExportFetchFunc 分批取数：afterID 为上一批最后一行的业务主键（0 表示从头开始），
// 返回本批行数据与下一批游标；nextCursor 为 0 或行数为空表示没有更多数据。
type ExportFetchFunc func(ctx context.Context, afterID uint64, limit int) (rows [][]any, nextCursor uint64, err error)

// ExportTotalFunc 预估导出总行数，仅用于进度展示；失败时可省略（进度会缺少总量）。
type ExportTotalFunc func(ctx context.Context) (int64, error)

// ExportBuilder 由业务模块注册：把提交参数解析为一次可执行的导出任务。
// params 是前端提交任务时携带的 JSON 筛选条件，由各业务自行解析校验。
type ExportBuilder func(ctx context.Context, userID uint64, params json.RawMessage) (ExportJob, error)

// ExportJob 描述一次可执行的导出任务。
type ExportJob struct {
	Module  string          // 模块标识，与注册名一致
	Title   string          // 展示标题，同时作为工作表名
	Headers []string        // 表头
	Total   ExportTotalFunc // 可为 nil
	Fetch   ExportFetchFunc
}

// ExportOptions 是异步导出的运行参数。
type ExportOptions struct {
	Workers   int   // 并发执行的任务数
	QueueSize int   // 等待队列长度，超出后拒绝提交
	BatchSize int   // 每批游标取数的行数
	MaxRows   int64 // 单任务行数上限，超出后任务失败
}

// ExportService 提供异步导出的提交、后台执行、进度查询与文件下载。
// 任务在进程内的有界队列中排队，重启后未完成任务会被标记失败。
type ExportService struct {
	tasks    ExportTaskRepository
	store    storage.Store
	notifier Notifier
	params   ParameterRepository
	opts     ExportOptions

	mu       sync.RWMutex
	builders map[string]ExportBuilder
	queue    chan exportItem
	wg       sync.WaitGroup
	closed   bool
}

// exportItem 是队列里的一次投递：任务记录连同构建好的执行计划。
type exportItem struct {
	task *model.ExportTask
	job  ExportJob
}

// NewExportService 创建导出服务，配置缺省时使用内置默认值。
func NewExportService(tasks ExportTaskRepository, store storage.Store, notifier Notifier, params ParameterRepository, opts ExportOptions) *ExportService {
	if opts.Workers <= 0 {
		opts.Workers = 2
	}
	if opts.QueueSize <= 0 {
		opts.QueueSize = 16
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 2000
	}
	if opts.MaxRows <= 0 {
		opts.MaxRows = 200000
	}
	return &ExportService{
		tasks: tasks, store: store, notifier: notifier, params: params, opts: opts,
		builders: make(map[string]ExportBuilder),
		queue:    make(chan exportItem, opts.QueueSize),
	}
}

// Register 注册模块的导出构建器；module 与前端提交的 module 字段一致。
func (s *ExportService) Register(module string, b ExportBuilder) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.builders[module] = b
}

// Boot 启动后台 worker，并把上次进程遗留的未完成任务标记失败。
func (s *ExportService) Boot() {
	ctx := context.WithoutCancel(context.Background())
	for {
		left, err := s.tasks.ListUnfinished(ctx, 100)
		if err != nil || len(left) == 0 {
			if err != nil {
				slog.Warn("export: scan leftover tasks failed", "error", err)
			}
			break
		}
		now := time.Now()
		for _, t := range left {
			_ = s.tasks.UpdateFields(ctx, t.ID, map[string]any{
				"status": model.ExportFailed, "error": "服务重启导致任务中断，请重新导出", "finished_at": now,
			})
		}
		if len(left) < 100 {
			break
		}
	}
	for i := 0; i < s.opts.Workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
}

// Close 关闭队列并等待 worker 处理完剩余任务，重复调用安全。
func (s *ExportService) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	close(s.queue)
	s.mu.Unlock()
	s.wg.Wait()
}

// worker 消费提交队列，逐个执行导出任务。
func (s *ExportService) worker() {
	defer s.wg.Done()
	for item := range s.queue {
		s.run(item)
	}
}

// Submit 提交异步导出任务并立即返回记录；执行进度通过任务列表轮询。
func (s *ExportService) Submit(ctx context.Context, userID uint64, module string, params json.RawMessage) (*model.ExportTask, error) {
	if userID == 0 {
		return nil, apperr.New(apperr.CodeInvalidArgument, "缺少导出人")
	}
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, apperr.New(apperr.CodeInternal, "导出服务已停止")
	}
	builder, ok := s.builders[module]
	s.mu.RUnlock()
	if !ok {
		return nil, apperr.New(apperr.CodeInvalidArgument, "未知的导出模块")
	}
	// 排队与执行中的任务总量受队列容量约束，超限直接拒绝，避免无界堆积。
	unfinished, err := s.tasks.CountUnfinished(ctx, userID)
	if err != nil {
		return nil, err
	}
	if unfinished >= int64(s.opts.QueueSize) {
		return nil, apperr.New(apperr.CodeTooManyRequests, "导出任务过多，请等待已有任务完成")
	}
	job, err := builder(ctx, userID, params)
	if err != nil {
		return nil, err
	}
	task := &model.ExportTask{
		UserID: userID,
		Module: module,
		Title:  truncateRunes(job.Title, 128),
		Status: model.ExportPending,
	}
	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, err
	}
	select {
	case s.queue <- exportItem{task: task, job: job}:
	default:
		// 与并发提交竞争时队列可能已满：撤回记录并拒绝。
		_ = s.tasks.Delete(ctx, task.ID)
		return nil, apperr.New(apperr.CodeTooManyRequests, "导出任务过多，请稍后再试")
	}
	return task, nil
}

// Tasks 分页查询某个用户的导出任务，按创建时间倒序。
func (s *ExportService) Tasks(ctx context.Context, userID uint64, status *int8, q pagination.Query) ([]model.ExportTask, int64, error) {
	q.Normalize()
	return s.tasks.List(ctx, userID, status, q.Page, q.PageSize)
}

// Cancel 取消任务：排队中的任务会在出队时收尾，执行中的由 worker 在下一批前停止。
func (s *ExportService) Cancel(ctx context.Context, userID, id uint64) error {
	task, err := s.ownTask(ctx, userID, id)
	if err != nil {
		return err
	}
	if task.Status != model.ExportPending && task.Status != model.ExportRunning {
		return apperr.New(apperr.CodeInvalidArgument, "任务已结束，无法取消")
	}
	return s.tasks.UpdateFields(ctx, id, map[string]any{"canceled": true})
}

// Delete 删除任务记录并清理产物文件；未结束的任务需要先取消。
func (s *ExportService) Delete(ctx context.Context, userID, id uint64) error {
	task, err := s.ownTask(ctx, userID, id)
	if err != nil {
		return err
	}
	if task.Status == model.ExportPending || task.Status == model.ExportRunning {
		_ = s.tasks.UpdateFields(ctx, id, map[string]any{"canceled": true})
		return apperr.New(apperr.CodeInvalidArgument, "任务尚未结束，已请求取消，请稍后删除")
	}
	s.removeFile(ctx, task.FileKey)
	return s.tasks.Delete(ctx, id)
}

// OpenFile 校验归属后打开导出文件，供下载接口流式读取。
func (s *ExportService) OpenFile(ctx context.Context, userID, id uint64) (*model.ExportTask, io.ReadCloser, error) {
	task, err := s.ownTask(ctx, userID, id)
	if err != nil {
		return nil, nil, err
	}
	if task.Status != model.ExportSuccess || task.FileKey == "" {
		return nil, nil, apperr.New(apperr.CodeInvalidArgument, "任务尚未完成或文件已清理")
	}
	rc, err := s.store.Open(ctx, task.FileKey)
	if err != nil {
		return nil, nil, err
	}
	return task, rc, nil
}

// CleanBefore 删除结束时间早于 before 的任务记录与产物文件，供定时任务调用。
func (s *ExportService) CleanBefore(ctx context.Context, before time.Time) error {
	const pageSize = 500
	for {
		tasks, err := s.tasks.ListExpired(ctx, before, pageSize)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			return nil
		}
		ids := make([]uint64, 0, len(tasks))
		for _, t := range tasks {
			s.removeFile(ctx, t.FileKey)
			ids = append(ids, t.ID)
		}
		if err := s.tasks.HardDelete(ctx, ids); err != nil {
			return err
		}
		if len(tasks) < pageSize {
			return nil
		}
	}
}

// run 执行一个出队任务：跳过已取消的任务，否则进入生成流程并兜底收敛状态。
func (s *ExportService) run(item exportItem) {
	// 任务生命周期独立于发起请求，必须使用不可取消的背景上下文。
	ctx := context.WithoutCancel(context.Background())
	task, err := s.tasks.Get(ctx, item.task.ID)
	if err != nil {
		slog.Warn("export: load task failed", "task", item.task.ID, "error", err)
		return
	}
	if task.Canceled {
		s.cancelFinish(ctx, task)
		return
	}
	if err := s.tasks.UpdateFields(ctx, task.ID, map[string]any{"status": model.ExportRunning}); err != nil {
		slog.Error("export: mark running failed", "task", task.ID, "error", err)
	}
	s.generate(ctx, task, item.job)
}

// generate 是任务主流程：游标分批取数、流式写表、落盘并更新进度。
func (s *ExportService) generate(ctx context.Context, task *model.ExportTask, job ExportJob) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("export: task panicked", "task", task.ID, "module", task.Module,
				"panic", r, "stack", string(debug.Stack()))
			s.fail(ctx, task, "导出过程发生内部错误")
		}
	}()

	// 预估总量，失败不影响任务执行。
	if job.Total != nil {
		if total, err := job.Total(ctx); err == nil && total >= 0 {
			_ = s.tasks.UpdateFields(ctx, task.ID, map[string]any{"total": total})
		}
	}

	book, err := excel.NewStreamBook(job.Title, job.Headers)
	if err != nil {
		s.fail(ctx, task, "创建表格失败: "+err.Error())
		return
	}

	var (
		written int64
		cursor  uint64
	)
	for {
		// 每批开始前检查取消标志，及时停止并清理。
		fresh, err := s.tasks.Get(ctx, task.ID)
		if err != nil {
			s.fail(ctx, task, "查询任务状态失败")
			_ = book.Close()
			return
		}
		if fresh.Canceled {
			_ = book.Close()
			s.cancelFinish(ctx, fresh)
			return
		}

		rows, next, err := job.Fetch(ctx, cursor, s.opts.BatchSize)
		if err != nil {
			s.fail(ctx, task, "读取数据失败: "+err.Error())
			_ = book.Close()
			return
		}
		for _, row := range rows {
			if err := book.WriteRow(row); err != nil {
				s.fail(ctx, task, "写入表格失败: "+err.Error())
				_ = book.Close()
				return
			}
		}
		written += int64(len(rows))
		if len(rows) > 0 {
			_ = s.tasks.UpdateFields(ctx, task.ID, map[string]any{"processed": written})
		}

		if written >= s.opts.MaxRows {
			_ = book.Close()
			s.fail(ctx, task, fmt.Sprintf("数据量超过单次导出上限 %d 行，请缩小筛选范围", s.opts.MaxRows))
			return
		}
		if len(rows) == 0 || next == 0 {
			break
		}
		// 游标必须前进，业务实现错误时立即失败而不是死循环。
		if next <= cursor {
			s.fail(ctx, task, "数据读取异常：导出游标未前进")
			_ = book.Close()
			return
		}
		cursor = next
	}

	// 收尾前最后一次检查取消标志，避免取消请求在最后一批取数之后丢失。
	fresh, err := s.tasks.Get(ctx, task.ID)
	if err != nil {
		s.fail(ctx, task, "查询任务状态失败")
		_ = book.Close()
		return
	}
	if fresh.Canceled {
		_ = book.Close()
		s.cancelFinish(ctx, fresh)
		return
	}

	// 成功收尾：写盘 → 存储登记 → 更新记录 → 站内通知。
	// 文件名按全局模板实时拼接，配置变更对下一次导出立即生效。
	tpl, err := s.FilenameTemplate(ctx)
	if err != nil {
		s.fail(ctx, task, "读取文件名模板失败: "+err.Error())
		return
	}
	filename := ComposeFilename(tpl, job.Module, job.Title, time.Now()) + ".xlsx"
	var buf bytes.Buffer
	if err := book.SaveTo(&buf); err != nil {
		s.fail(ctx, task, "生成表格失败: "+err.Error())
		_ = book.Close()
		return
	}
	_ = book.Close()
	info, err := s.store.Save(ctx, filename, &buf)
	if err != nil {
		s.fail(ctx, task, "保存导出文件失败: "+err.Error())
		return
	}
	now := time.Now()
	err = s.tasks.UpdateFields(ctx, task.ID, map[string]any{
		"status":      model.ExportSuccess,
		"processed":   written,
		"file_key":    info.Key,
		"file_name":   filename,
		"file_size":   info.Size,
		"finished_at": now,
	})
	if err != nil {
		slog.Error("export: mark success failed", "task", task.ID, "error", err)
	}
	s.notifyResult(ctx, task, written, "")
}

// fail 把任务标记为失败并发送失败通知。
func (s *ExportService) fail(ctx context.Context, task *model.ExportTask, msg string) {
	now := time.Now()
	if err := s.tasks.UpdateFields(ctx, task.ID, map[string]any{
		"status": model.ExportFailed, "error": truncateRunes(msg, 512), "finished_at": now,
	}); err != nil {
		slog.Error("export: mark failed", "task", task.ID, "error", err)
	}
	s.notifyResult(ctx, task, 0, msg)
}

// cancelFinish 把已取消的任务收敛到取消状态。
func (s *ExportService) cancelFinish(ctx context.Context, task *model.ExportTask) {
	now := time.Now()
	if err := s.tasks.UpdateFields(ctx, task.ID, map[string]any{
		"status": model.ExportCanceled, "finished_at": now,
	}); err != nil {
		slog.Warn("export: mark canceled", "task", task.ID, "error", err)
	}
}

// notifyResult 推送完成/失败站内通知，点击跳转导出记录页。
func (s *ExportService) notifyResult(ctx context.Context, task *model.ExportTask, written int64, failMsg string) {
	if s.notifier == nil {
		return
	}
	in := NotificationInput{UserID: task.UserID, Link: "/system/exports"}
	if failMsg == "" {
		in.Type = model.NotificationSuccess
		in.Title = fmt.Sprintf("导出完成：%s", task.Title)
		in.Content = fmt.Sprintf("共导出 %d 行，前往导出记录页下载文件。", written)
	} else {
		in.Type = model.NotificationError
		in.Title = fmt.Sprintf("导出失败：%s", task.Title)
		in.Content = failMsg
	}
	_ = s.notifier.Notify(ctx, in)
}

// removeFile 删除产物文件，失败只记录告警，不阻断记录清理。
func (s *ExportService) removeFile(ctx context.Context, key string) {
	if key == "" {
		return
	}
	if err := s.store.Delete(ctx, key); err != nil {
		slog.Warn("export: delete file failed", "key", key, "error", err)
	}
}

// ownTask 读取任务并校验归属；不存在或不属于该用户时统一返回 404，避免探测他人任务。
func (s *ExportService) ownTask(ctx context.Context, userID, id uint64) (*model.ExportTask, error) {
	task, err := s.tasks.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, apperr.ErrNotFound
		}
		return nil, err
	}
	if task.UserID != userID {
		return nil, apperr.ErrNotFound
	}
	return task, nil
}

// MapExportSpec 定义基于 []map 数据的导出列。
type MapExportSpec struct {
	// Order 决定导出列的顺序，元素为 map 的 key；
	// 为空时取全部数据 key 的并集并按字典序排列，保证列序稳定。
	Order []string
	// Names 把 key 映射为表头名称；未映射的 key 直接使用 key 原文作为表头。
	// Order 中存在但数据中没有的 key 会导出为空列。
	Names map[string]string
}

// NewMapsExportJob 把 []map[string]any 数据包装为可执行的导出任务，
// 供业务 builder 在能一次性取得数据（或数据由上游组装）时直接复用框架：
// 表头顺序来自 spec.Order，表头名称来自 spec.Names，行值经 MapCellValue 转换。
// Fetch 按调用方的 BatchSize 对内存切片分批写表，进度条随批推进。
func NewMapsExportJob(module, title string, spec MapExportSpec, rows []map[string]any) ExportJob {
	order := spec.Order
	if len(order) == 0 {
		order = mapKeyUnion(rows)
	}
	headers := make([]string, len(order))
	for i, key := range order {
		if name := spec.Names[key]; name != "" {
			headers[i] = name
		} else {
			headers[i] = key
		}
	}
	return ExportJob{
		Module:  module,
		Title:   title,
		Headers: headers,
		Total: func(context.Context) (int64, error) {
			return int64(len(rows)), nil
		},
		Fetch: func(_ context.Context, afterID uint64, limit int) ([][]any, uint64, error) {
			// 内存切片没有业务主键，用“已取行数”作为游标：0 表示从头，
			// next 为下一批起始下标，取完返回 0 让框架收尾。
			start := int(afterID)
			if start >= len(rows) {
				return nil, 0, nil
			}
			if limit <= 0 || start+limit > len(rows) {
				limit = len(rows) - start
			}
			batch := make([][]any, 0, limit)
			for _, row := range rows[start : start+limit] {
				cells := make([]any, len(order))
				for i, key := range order {
					cells[i] = MapCellValue(row[key])
				}
				batch = append(batch, cells)
			}
			return batch, uint64(start + len(batch)), nil
		},
	}
}

// mapKeyUnion 收集全部行的 key 并集并按字典序排序。
func mapKeyUnion(rows []map[string]any) []string {
	seen := make(map[string]struct{})
	for _, row := range rows {
		for key := range row {
			seen[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// MapCellValue 把 map 值转换为 xlsx 单元格值：nil 转空串，时间格式化，
// 切片与嵌套 map 序列化为 JSON，其余类型原样交给 excelize（保留数字语义）。
func MapCellValue(v any) any {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case bool:
		return value
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return value
	case time.Time:
		return value.Format("2006-01-02 15:04:05")
	case *time.Time:
		if value == nil {
			return ""
		}
		return value.Format("2006-01-02 15:04:05")
	case fmt.Stringer:
		return value.String()
	case json.RawMessage:
		return string(value)
	case map[string]any, []any:
		raw, err := json.Marshal(value)
		if err != nil {
			return fmt.Sprintf("%v", value)
		}
		return string(raw)
	default:
		return fmt.Sprintf("%v", value)
	}
}
