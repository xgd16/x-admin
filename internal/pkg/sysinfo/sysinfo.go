// Package sysinfo 采集主机、进程和运行时指标，供系统监控页使用。
//
// 所有采集都采用尽力而为策略：单个指标失败时返回零值，不阻塞其余指标。
// 这样数据库或某个文件系统异常时，监控页仍能展示可用的数据。
package sysinfo

import (
	"context"
	"os"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// Snapshot 是一次系统指标采集结果。
type Snapshot struct {
	Host        HostInfo    `json:"host"`
	CPU         CPUInfo     `json:"cpu"`
	Memory      MemoryInfo  `json:"memory"`
	Swap        SwapInfo    `json:"swap"`
	Load        LoadInfo    `json:"load"`
	Disks       []DiskInfo  `json:"disks"`
	Network     NetworkInfo `json:"network"`
	Process     ProcessInfo `json:"process"`
	CollectedAt time.Time   `json:"collected_at"`
}

// HostInfo 描述操作系统与主机信息。
type HostInfo struct {
	Hostname       string `json:"hostname"`
	OS             string `json:"os"`
	Platform       string `json:"platform"`
	PlatformFamily string `json:"platform_family"`
	Version        string `json:"version"`
	KernelVersion  string `json:"kernel_version"`
	Arch           string `json:"arch"`
	Virtualization string `json:"virtualization"`
	BootTime       uint64 `json:"boot_time"`
	UptimeSeconds  uint64 `json:"uptime_seconds"`
	Procs          uint64 `json:"procs"`
}

// CPUInfo 描述 CPU 型号、核心数与当前使用率。
type CPUInfo struct {
	Model        string  `json:"model"`
	PhysicalCore int     `json:"physical_cores"`
	LogicalCore  int     `json:"logical_cores"`
	Mhz          float64 `json:"mhz"`
	UsagePercent float64 `json:"usage_percent"`
}

// MemoryInfo 描述物理内存使用情况。
type MemoryInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Available   uint64  `json:"available"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// SwapInfo 描述交换分区使用情况。
type SwapInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"used_percent"`
}

// LoadInfo 描述系统负载；Windows 上可能为 0。
type LoadInfo struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// DiskInfo 描述一个挂载点的磁盘使用情况。
type DiskInfo struct {
	Device      string  `json:"device"`
	Mountpoint  string  `json:"mountpoint"`
	Fstype      string  `json:"fstype"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// NetworkInfo 汇总全部网卡的累计收发指标。
type NetworkInfo struct {
	Interfaces  int    `json:"interfaces"`
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
	ErrorsIn    uint64 `json:"errors_in"`
	ErrorsOut   uint64 `json:"errors_out"`
	DropsIn     uint64 `json:"drops_in"`
	DropsOut    uint64 `json:"drops_out"`
}

// ProcessInfo 描述当前 Go 进程的资源占用。
type ProcessInfo struct {
	PID     int32  `json:"pid"`
	Threads int32  `json:"threads"`
	RSS     uint64 `json:"rss"`
	VMS     uint64 `json:"vms"`
	Swap    uint64 `json:"swap"`
}

// DatabaseStats 是数据库连接池与探测结果，由基础设施层填充。
type DatabaseStats struct {
	Status         string  `json:"status"`
	LatencyMS      float64 `json:"latency_ms"`
	Open           int     `json:"open"`
	InUse          int     `json:"in_use"`
	Idle           int     `json:"idle"`
	MaxOpen        int     `json:"max_open"`
	WaitCount      int64   `json:"wait_count"`
	WaitDurationMS int64   `json:"wait_duration_ms"`
}

// Collect 采集当前系统快照。ctx 用于限制磁盘、进程等可能较慢的调用。
func Collect(ctx context.Context) Snapshot {
	snap := Snapshot{CollectedAt: time.Now(), Disks: []DiskInfo{}}

	if info, err := host.InfoWithContext(ctx); err == nil && info != nil {
		snap.Host = HostInfo{
			Hostname:       info.Hostname,
			OS:             info.OS,
			Platform:       info.Platform,
			PlatformFamily: info.PlatformFamily,
			Version:        info.PlatformVersion,
			KernelVersion:  info.KernelVersion,
			Arch:           info.KernelArch,
			Virtualization: joinVirtualization(info.VirtualizationSystem, info.VirtualizationRole),
			BootTime:       info.BootTime,
			UptimeSeconds:  info.Uptime,
			Procs:          info.Procs,
		}
	}

	if percents, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(percents) > 0 {
		snap.CPU.UsagePercent = percents[0]
	}
	if infos, err := cpu.InfoWithContext(ctx); err == nil && len(infos) > 0 {
		snap.CPU.Model = infos[0].ModelName
		snap.CPU.Mhz = infos[0].Mhz
	}
	if physical, err := cpu.Counts(false); err == nil {
		snap.CPU.PhysicalCore = physical
	}
	if logical, err := cpu.Counts(true); err == nil {
		snap.CPU.LogicalCore = logical
	}

	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil && vm != nil {
		snap.Memory = MemoryInfo{
			Total: vm.Total, Used: vm.Used, Available: vm.Available,
			Free: vm.Free, UsedPercent: vm.UsedPercent,
		}
	}
	if swap, err := mem.SwapMemoryWithContext(ctx); err == nil && swap != nil {
		snap.Swap = SwapInfo{Total: swap.Total, Used: swap.Used, UsedPercent: swap.UsedPercent}
	}

	if avg, err := load.AvgWithContext(ctx); err == nil && avg != nil {
		snap.Load = LoadInfo{Load1: avg.Load1, Load5: avg.Load5, Load15: avg.Load15}
	}

	if partitions, err := disk.PartitionsWithContext(ctx, false); err == nil {
		snap.Disks = collectDisks(ctx, partitions)
	}

	if counters, err := gnet.IOCountersWithContext(ctx, false); err == nil && len(counters) > 0 {
		row := counters[0]
		snap.Network = NetworkInfo{
			Interfaces: len(counters), BytesSent: row.BytesSent, BytesRecv: row.BytesRecv,
			PacketsSent: row.PacketsSent, PacketsRecv: row.PacketsRecv,
			ErrorsIn: row.Errin, ErrorsOut: row.Errout, DropsIn: row.Dropin, DropsOut: row.Dropout,
		}
	}

	if p, err := process.NewProcessWithContext(ctx, int32(os.Getpid())); err == nil && p != nil {
		snap.Process.PID = p.Pid
		if memory, err := p.MemoryInfoWithContext(ctx); err == nil && memory != nil {
			snap.Process.RSS = memory.RSS
			snap.Process.VMS = memory.VMS
			snap.Process.Swap = memory.Swap
		}
		if threads, err := p.NumThreadsWithContext(ctx); err == nil {
			snap.Process.Threads = threads
		}
	}

	return snap
}

// collectDisks 过滤无容量或重复的挂载点，并限制返回数量，避免容器内出现大量伪文件系统。
func collectDisks(ctx context.Context, partitions []disk.PartitionStat) []DiskInfo {
	const maxDisks = 12
	out := make([]DiskInfo, 0, len(partitions))
	seen := make(map[string]struct{}, len(partitions))
	for _, part := range partitions {
		if _, ok := seen[part.Mountpoint]; ok {
			continue
		}
		usage, err := disk.UsageWithContext(ctx, part.Mountpoint)
		if err != nil || usage == nil || usage.Total == 0 {
			continue
		}
		seen[part.Mountpoint] = struct{}{}
		out = append(out, DiskInfo{
			Device: part.Device, Mountpoint: part.Mountpoint, Fstype: part.Fstype,
			Total: usage.Total, Used: usage.Used, Free: usage.Free, UsedPercent: usage.UsedPercent,
		})
		if len(out) >= maxDisks {
			break
		}
	}
	return out
}

// joinVirtualization 合并虚拟化平台与角色，例如 "docker guest"。
func joinVirtualization(system, role string) string {
	switch {
	case system == "":
		return role
	case role == "":
		return system
	default:
		return system + " " + role
	}
}
