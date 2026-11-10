package service

import (
	"context"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"x-admin/internal/pkg/cache"
	"x-admin/internal/pkg/sysinfo"
)

// DBStatsProvider 由基础设施层实现，返回数据库连接池与探测指标。
type DBStatsProvider interface {
	Stats(ctx context.Context) (sysinfo.DatabaseStats, error)
}

// AppInfo 是监控页展示的进程元信息。
type AppInfo struct {
	Name          string    `json:"name"`
	Env           string    `json:"env"`
	Version       string    `json:"version"`
	PID           int       `json:"pid"`
	StartedAt     time.Time `json:"started_at"`
	UptimeSeconds int64     `json:"uptime_seconds"`
}

// RuntimeInfo 是 Go 运行时指标。
type RuntimeInfo struct {
	GoVersion     string     `json:"go_version"`
	Goroutines    int        `json:"goroutines"`
	GOMAXPROCS    int        `json:"gomaxprocs"`
	Alloc         uint64     `json:"alloc"`
	TotalAlloc    uint64     `json:"total_alloc"`
	Sys           uint64     `json:"sys"`
	HeapAlloc     uint64     `json:"heap_alloc"`
	HeapInuse     uint64     `json:"heap_inuse"`
	HeapIdle      uint64     `json:"heap_idle"`
	HeapReleased  uint64     `json:"heap_released"`
	StackInuse    uint64     `json:"stack_inuse"`
	StackSys      uint64     `json:"stack_sys"`
	NumGC         uint32     `json:"num_gc"`
	GCCPUFraction float64    `json:"gc_cpu_fraction"`
	LastGC        *time.Time `json:"last_gc"`
	LastGCPauseMS float64    `json:"last_gc_pause_ms"`
}

// CacheStats 是缓存探测结果。
type CacheStats struct {
	Driver    string  `json:"driver"`
	Status    string  `json:"status"`
	LatencyMS float64 `json:"latency_ms"`
}

// MonitorOverview 是系统监控页的完整快照。
type MonitorOverview struct {
	App      AppInfo               `json:"app"`
	System   sysinfo.Snapshot      `json:"system"`
	Runtime  RuntimeInfo           `json:"runtime"`
	Database sysinfo.DatabaseStats `json:"database"`
	Cache    CacheStats            `json:"cache"`
}

// MonitorService 聚合主机、运行时、数据库与缓存指标。
type MonitorService struct {
	cache       cache.Store
	cacheDriver string
	dbStats     DBStatsProvider
	appName     string
	env         string
	version     string
	startedAt   time.Time
}

// NewMonitorService 创建系统监控服务；version 为空时从构建信息读取。
func NewMonitorService(
	store cache.Store,
	cacheDriver string,
	dbStats DBStatsProvider,
	appName, env, version string,
) *MonitorService {
	if version == "" {
		version = buildVersion()
	}
	return &MonitorService{
		cache: store, cacheDriver: cacheDriver, dbStats: dbStats,
		appName: appName, env: env, version: version, startedAt: time.Now(),
	}
}

// Overview 返回一次系统监控快照。
// 数据库或缓存探测失败不阻塞整体响应，而是以 status=down 展示。
func (s *MonitorService) Overview(ctx context.Context) *MonitorOverview {
	now := time.Now()
	out := &MonitorOverview{
		App: AppInfo{
			Name: s.appName, Env: s.env, Version: s.version,
			PID: os.Getpid(), StartedAt: s.startedAt,
			UptimeSeconds: int64(now.Sub(s.startedAt) / time.Second),
		},
		Cache: CacheStats{Driver: s.cacheDriver, Status: "down"},
	}
	out.Runtime = collectRuntime()

	// 先探测依赖，再做可能受网络挂载影响的主机采集，
	// 避免慢磁盘拖到请求超时后把正常的数据库/缓存误报为 down。
	start := time.Now()
	if err := s.cache.Ping(ctx); err == nil {
		out.Cache.Status = "up"
	}
	out.Cache.LatencyMS = float64(time.Since(start).Microseconds()) / 1000

	if s.dbStats != nil {
		stats, err := s.dbStats.Stats(ctx)
		if err != nil {
			stats.Status = "down"
		}
		out.Database = stats
	}

	out.System = sysinfo.Collect(ctx)
	return out
}

// collectRuntime 读取 Go 运行时指标。
func collectRuntime() RuntimeInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	info := RuntimeInfo{
		GoVersion: runtime.Version(), Goroutines: runtime.NumGoroutine(),
		GOMAXPROCS: runtime.GOMAXPROCS(0),
		Alloc:      m.Alloc, TotalAlloc: m.TotalAlloc, Sys: m.Sys,
		HeapAlloc: m.HeapAlloc, HeapInuse: m.HeapInuse, HeapIdle: m.HeapIdle,
		HeapReleased: m.HeapReleased, StackInuse: m.StackInuse, StackSys: m.StackSys,
		NumGC: m.NumGC, GCCPUFraction: m.GCCPUFraction,
	}
	if m.LastGC > 0 {
		last := time.Unix(0, int64(m.LastGC))
		info.LastGC = &last
	}
	// PauseNs 是环形缓冲，NumGC 指向最近一次 GC 的暂停时间。
	if m.NumGC > 0 {
		info.LastGCPauseMS = float64(m.PauseNs[(m.NumGC+255)%256]) / 1e6
	}
	return info
}

// buildVersion 从 Go 构建信息读取主模块版本。
func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}
	return "dev"
}
