// Package jobs 封装基于 robfig/cron 的定时任务调度。
package jobs

import (
	"sync"

	"github.com/robfig/cron/v3"
)

// Cron 用“任务名 → entry ID”的映射包装 robfig/cron。
// 服务层只需按业务 ID 替换或移除调度，不必自己记录 entry ID。
type Cron struct {
	engine  *cron.Cron
	mu      sync.Mutex
	entries map[string]cron.EntryID
}

// New 创建一个尚未启动的调度器。
func New() *Cron {
	return &Cron{engine: cron.New(), entries: make(map[string]cron.EntryID)}
}

// Validate 校验标准 5 段 Cron 表达式（分 时 日 月 周）。
func (c *Cron) Validate(spec string) error {
	_, err := cron.ParseStandard(spec)
	return err
}

// Add 注册或替换一个命名任务。
// 同名任务会先移除旧调度，保证一个业务任务始终只有一个 entry。
func (c *Cron) Add(name, spec string, fn func()) error {
	id, err := c.engine.AddFunc(spec, fn)
	if err != nil {
		return err
	}
	c.mu.Lock()
	if old, ok := c.entries[name]; ok {
		c.engine.Remove(old)
	}
	c.entries[name] = id
	c.mu.Unlock()
	return nil
}

// Remove 移除命名任务；任务不存在时静默返回。
func (c *Cron) Remove(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if id, ok := c.entries[name]; ok {
		c.engine.Remove(id)
		delete(c.entries, name)
	}
}

// Start 启动调度循环。
func (c *Cron) Start() { c.engine.Start() }

// Stop 停止调度循环，等待正在执行的任务结束。
func (c *Cron) Stop() { c.engine.Stop() }
