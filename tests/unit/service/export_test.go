package service_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/model"
	"x-admin/internal/pkg/apperr"
	"x-admin/internal/pkg/excel"
	"x-admin/internal/pkg/pagination"
	"x-admin/internal/service"
	"x-admin/tests/testsupport"
)

func pageOne() pagination.Query { return paginationQuery() }

func TestExportSubmitUnknownModule(t *testing.T) {
	app := testsupport.New(t)
	ctx := context.Background()

	_, err := app.Services.Exports.Submit(ctx, 1, "no_such_module", nil)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeInvalidArgument, apperr.From(err).Code)
	assert.Contains(t, err.Error(), "未知的导出模块")
}

func TestExportTaskLifecycle(t *testing.T) {
	app := testsupport.New(t)
	ctx := context.Background()
	admin := app.SeedAdmin(t)

	task, err := app.Services.Exports.Submit(ctx, admin.ID, "users", nil)
	require.NoError(t, err)
	assert.Equal(t, model.ExportPending, task.Status)
	assert.Equal(t, "用户列表", task.Title)

	fresh := waitExportTask(t, app, task.ID)
	require.Equal(t, model.ExportSuccess, fresh.Status, fresh.Error)
	assert.Equal(t, int64(1), fresh.Processed)
	assert.True(t, strings.HasPrefix(fresh.FileName, "users_"), fresh.FileName)
	assert.True(t, strings.HasSuffix(fresh.FileName, ".xlsx"))
	assert.NotZero(t, fresh.FileSize)

	// 完成后写入站内通知。
	list, total, err := app.Repos.Notifications.List(ctx, admin.ID, false, 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	assert.Contains(t, list[0].Title, "导出完成")
	assert.Equal(t, "/system/exports", list[0].Link)

	// 下载：归属校验 + 文件可读。
	_, rc, err := app.Services.Exports.OpenFile(ctx, admin.ID, task.ID)
	require.NoError(t, err)
	require.NoError(t, rc.Close())

	// 其他用户不可见。
	other := app.CreateUser(t, "outsider", "Passw0rd123", model.StatusEnabled)
	_, _, err = app.Services.Exports.OpenFile(ctx, other.ID, task.ID)
	require.ErrorIs(t, err, apperr.ErrNotFound)
	require.ErrorIs(t, app.Services.Exports.Cancel(ctx, other.ID, task.ID), apperr.ErrNotFound)

	// 删除记录并清理文件。
	key := fresh.FileKey
	require.NoError(t, app.Services.Exports.Delete(ctx, admin.ID, task.ID))
	require.ErrorIs(t, app.Services.Exports.Delete(ctx, admin.ID, task.ID), apperr.ErrNotFound)
	_, err = app.Storage.Open(ctx, key)
	require.Error(t, err, "产物文件应随记录一并删除")
}

func TestExportCancelRunningTask(t *testing.T) {
	app := testsupport.New(t)
	ctx := context.Background()

	release := make(chan struct{})
	app.Services.Exports.Register("slow", func(context.Context, uint64, json.RawMessage) (service.ExportJob, error) {
		return service.ExportJob{
			Module: "slow", Title: "慢任务", Headers: []string{"A"},
			Fetch: func(context.Context, uint64, int) ([][]any, uint64, error) {
				<-release
				return nil, 0, nil
			},
		}, nil
	})
	task, err := app.Services.Exports.Submit(ctx, 1, "slow", nil)
	require.NoError(t, err)

	deadline := time.Now().Add(5 * time.Second)
	for {
		fresh, err := app.Repos.Exports.Get(ctx, task.ID)
		require.NoError(t, err)
		if fresh.Status == model.ExportRunning {
			break
		}
		require.False(t, time.Now().After(deadline), "task never started")
		time.Sleep(5 * time.Millisecond)
	}
	require.NoError(t, app.Services.Exports.Cancel(ctx, 1, task.ID))
	close(release)

	fresh := waitExportTask(t, app, task.ID)
	assert.Equal(t, model.ExportCanceled, fresh.Status)

	// 已结束的任务不能重复取消。
	require.Error(t, app.Services.Exports.Cancel(ctx, 1, task.ID))
}

func TestExportMaxRowsGuard(t *testing.T) {
	app := testsupport.New(t)
	ctx := context.Background()

	// testsupport 里 MaxRows=100：fetch 每批返回 250 行，必须触发上限失败。
	app.Services.Exports.Register("huge", func(context.Context, uint64, json.RawMessage) (service.ExportJob, error) {
		return service.ExportJob{
			Module: "huge", Title: "超大", Headers: []string{"A"},
			Fetch: func(_ context.Context, afterID uint64, limit int) ([][]any, uint64, error) {
				rows := make([][]any, 250)
				for i := range rows {
					rows[i] = []any{afterID + uint64(i)}
				}
				return rows, afterID + 250, nil
			},
		}, nil
	})
	task, err := app.Services.Exports.Submit(ctx, 1, "huge", nil)
	require.NoError(t, err)
	fresh := waitExportTask(t, app, task.ID)
	assert.Equal(t, model.ExportFailed, fresh.Status)
	assert.Contains(t, fresh.Error, "上限")

	// 游标不前进：立即失败而不是死循环。
	app.Services.Exports.Register("stuck", func(context.Context, uint64, json.RawMessage) (service.ExportJob, error) {
		return service.ExportJob{
			Module: "stuck", Title: "卡住", Headers: []string{"A"},
			Fetch: func(context.Context, uint64, int) ([][]any, uint64, error) {
				return [][]any{{"v"}}, 7, nil
			},
		}, nil
	})
	task, err = app.Services.Exports.Submit(ctx, 1, "stuck", nil)
	require.NoError(t, err)
	fresh = waitExportTask(t, app, task.ID)
	assert.Equal(t, model.ExportFailed, fresh.Status)
	assert.Contains(t, fresh.Error, "游标")
}

func TestExportCleanBefore(t *testing.T) {
	app := testsupport.New(t)
	ctx := context.Background()

	// 手工放置一条已结束的记录与对应文件，再通过清理任务删除。
	info, err := app.Storage.Save(ctx, "old.xlsx", strings.NewReader("xlsx-bytes"))
	require.NoError(t, err)
	yesterday := time.Now().AddDate(0, 0, -1)
	old := &model.ExportTask{
		UserID: 1, Module: "users", Title: "旧任务", Status: model.ExportSuccess,
		FileName: "old.xlsx", FileKey: info.Key, FileSize: info.Size, FinishedAt: &yesterday,
	}
	require.NoError(t, app.Repos.Exports.Create(ctx, old))

	require.NoError(t, app.Services.Exports.CleanBefore(ctx, time.Now()))
	_, err = app.Repos.Exports.Get(ctx, old.ID)
	require.Error(t, err, "过期记录应被物理删除")
	_, err = app.Storage.Open(ctx, info.Key)
	require.Error(t, err, "过期文件应被删除")
}

func TestExportTaskListFilterByStatus(t *testing.T) {
	app := testsupport.New(t)
	ctx := context.Background()
	admin := app.SeedAdmin(t)

	task, err := app.Services.Exports.Submit(ctx, admin.ID, "users", nil)
	require.NoError(t, err)
	waitExportTask(t, app, task.ID)

	done := model.ExportSuccess
	list, total, err := app.Services.Exports.Tasks(ctx, admin.ID, &done, pageOne())
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, list, 1)

	pending := model.ExportPending
	list, total, err = app.Services.Exports.Tasks(ctx, admin.ID, &pending, pageOne())
	require.NoError(t, err)
	assert.EqualValues(t, 0, total)
	assert.Empty(t, list)
}

func TestFilenameTemplateDefaults(t *testing.T) {
	app := testsupport.New(t)
	ctx := context.Background()

	// 无参数时回退默认：模块 + 紧凑时间。
	tpl, err := app.Services.Exports.FilenameTemplate(ctx)
	require.NoError(t, err)
	require.Len(t, tpl.Segments, 2)
	assert.Equal(t, service.FilenameSegmentModule, tpl.Segments[0].Type)
	assert.Equal(t, service.FilenameSegmentTime, tpl.Segments[1].Type)

	now := time.Date(2026, 11, 8, 19, 58, 0, 0, time.Local)
	name := service.ComposeFilename(tpl, "users", "用户列表", now)
	assert.Equal(t, "users_20261108195800", name)
}

func TestFilenameTemplateRoundtrip(t *testing.T) {
	app := testsupport.New(t)
	ctx := context.Background()

	in := service.FilenameTemplate{Segments: []service.FilenameSegment{
		{Type: service.FilenameSegmentText, Value: " 审计导出 "},
		{Type: service.FilenameSegmentTitle},
		{Type: service.FilenameSegmentTime, Format: "2006-01-02"},
	}}
	saved, err := app.Services.Exports.UpdateFilenameTemplate(ctx, in)
	require.NoError(t, err)
	assert.Equal(t, "审计导出", saved.Segments[0].Value, "文本应去空白")

	loaded, err := app.Services.Exports.FilenameTemplate(ctx)
	require.NoError(t, err)
	require.Len(t, loaded.Segments, 3)
	assert.Equal(t, saved, loaded)

	// 非法字符在拼接时被清洗。
	now := time.Date(2026, 11, 8, 19, 58, 0, 0, time.Local)
	name := service.ComposeFilename(loaded, "users", "用户:列表/一", now)
	assert.Equal(t, "审计导出_用户-列表-一_2026-11-08", name)
}

func TestFilenameTemplateValidation(t *testing.T) {
	app := testsupport.New(t)
	ctx := context.Background()

	cases := []struct {
		name string
		tpl  service.FilenameTemplate
	}{
		{"空片段", service.FilenameTemplate{}},
		{"未知类型", service.FilenameTemplate{Segments: []service.FilenameSegment{{Type: "evil"}}}},
		{"时间格式不在白名单", service.FilenameTemplate{Segments: []service.FilenameSegment{
			{Type: service.FilenameSegmentTime, Format: "2006-01-02 15:04:05"},
		}}},
	}
	for _, tc := range cases {
		_, err := app.Services.Exports.UpdateFilenameTemplate(ctx, tc.tpl)
		assert.Error(t, err, tc.name)
	}

	// 文本超长被截断而不是报错。
	long := strings.Repeat("长", 40)
	saved, err := app.Services.Exports.UpdateFilenameTemplate(ctx, service.FilenameTemplate{
		Segments: []service.FilenameSegment{{Type: service.FilenameSegmentText, Value: long}},
	})
	require.NoError(t, err)
	assert.Equal(t, 32, utf8.RuneCountInString(saved.Segments[0].Value), "超长文本按字符截断")

	// 重复的标题/模块/时间片段被静默去重。
	dedup, err := app.Services.Exports.UpdateFilenameTemplate(ctx, service.FilenameTemplate{
		Segments: []service.FilenameSegment{
			{Type: service.FilenameSegmentTitle}, {Type: service.FilenameSegmentTitle},
			{Type: service.FilenameSegmentModule},
		},
	})
	require.NoError(t, err)
	assert.Len(t, dedup.Segments, 2)
}

func TestNewMapsExportJob(t *testing.T) {
	rows := []map[string]any{
		{"name": "alice", "score": 92.5, "created": time.Date(2026, 11, 8, 19, 58, 0, 0, time.Local)},
		{"name": "bob", "score": nil, "extra": map[string]any{"k": "v"}},
	}

	job := service.NewMapsExportJob("report", "成绩报表", service.MapExportSpec{
		Order: []string{"score", "name", "created", "missing", "extra"},
		Names: map[string]string{"score": "得分", "name": "姓名", "created": "创建时间"},
	}, rows)

	// 表头顺序按 Order，命名映射生效，未知 key 保留原文。
	assert.Equal(t, []string{"得分", "姓名", "创建时间", "missing", "extra"}, job.Headers)
	assert.Equal(t, int64(2), mustTotal(t, job))

	// 第一批（limit=1）：score nil 转空串、time 格式化、缺失 key 空列。
	batch, next, err := job.Fetch(context.Background(), 0, 1)
	require.NoError(t, err)
	assert.Equal(t, [][]any{{92.5, "alice", "2026-11-08 19:58:00", "", ""}}, batch)
	assert.Equal(t, uint64(1), next)

	// 第二批：嵌套 map 序列化为 JSON；下一批游标为 2。
	batch, next, err = job.Fetch(context.Background(), next, 1)
	require.NoError(t, err)
	assert.Equal(t, [][]any{{"", "bob", "", "", `{"k":"v"}`}}, batch)
	assert.Equal(t, uint64(2), next)

	// 第三批：数据取尽，游标归 0 通知框架收尾。
	batch, next, err = job.Fetch(context.Background(), next, 1)
	require.NoError(t, err)
	assert.Empty(t, batch)
	assert.Equal(t, uint64(0), next)

	// 游标越界安全。
	batch, next, err = job.Fetch(context.Background(), 99, 10)
	require.NoError(t, err)
	assert.Empty(t, batch)
	assert.Equal(t, uint64(0), next)
}

func TestNewMapsExportJobDefaultOrder(t *testing.T) {
	// 未指定 Order 时取 key 并集按字典序，保证列序稳定可预期。
	rows := []map[string]any{
		{"b": 1, "a": 2},
		{"c": 3, "a": 4},
	}
	job := service.NewMapsExportJob("report", "报表", service.MapExportSpec{}, rows)
	assert.Equal(t, []string{"a", "b", "c"}, job.Headers)
	assert.Equal(t, int64(2), mustTotal(t, job))

	batch, next, err := job.Fetch(context.Background(), 0, 10)
	require.NoError(t, err)
	assert.Equal(t, [][]any{{2, 1, ""}, {4, "", 3}}, batch)
	// 一次性取完剩余数据后，next 指向结尾，下一批返回 0 收尾。
	assert.Equal(t, uint64(2), next)
	batch, next, err = job.Fetch(context.Background(), next, 10)
	require.NoError(t, err)
	assert.Empty(t, batch)
	assert.Equal(t, uint64(0), next)
}

func TestMapsExportJobEndToEnd(t *testing.T) {
	app := testsupport.New(t)
	ctx := context.Background()
	admin := app.SeedAdmin(t)

	rows := []map[string]any{
		{"username": "alice", "status": 1},
		{"username": "bob", "status": 0},
	}
	app.Services.Exports.Register("members", func(context.Context, uint64, json.RawMessage) (service.ExportJob, error) {
		return service.NewMapsExportJob("members", "会员列表", service.MapExportSpec{
			Order: []string{"username", "status"},
			Names: map[string]string{"username": "账号", "status": "状态"},
		}, rows), nil
	})

	task, err := app.Services.Exports.Submit(ctx, admin.ID, "members", nil)
	require.NoError(t, err)
	fresh := waitExportTask(t, app, task.ID)
	require.Equal(t, model.ExportSuccess, fresh.Status, fresh.Error)
	assert.Equal(t, int64(2), fresh.Processed)

	rc, err := app.Storage.Open(ctx, fresh.FileKey)
	require.NoError(t, err)
	rowsRead, err := excel.Read(rc)
	require.NoError(t, err)
	_ = rc.Close()
	assert.Equal(t, [][]string{{"账号", "状态"}, {"alice", "1"}, {"bob", "0"}}, rowsRead)
}

// mustTotal 调用任务的 Total 并要求成功。
func mustTotal(t *testing.T, job service.ExportJob) int64 {
	t.Helper()
	total, err := job.Total(context.Background())
	require.NoError(t, err)
	return total
}
