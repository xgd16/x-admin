package jobs

import (
	"sync"

	"github.com/robfig/cron/v3"
)

// Cron wraps robfig/cron with named entries so services can replace a schedule
// without tracking entry ids.
type Cron struct {
	engine  *cron.Cron
	mu      sync.Mutex
	entries map[string]cron.EntryID
}

func New() *Cron {
	return &Cron{engine: cron.New(), entries: make(map[string]cron.EntryID)}
}

func (c *Cron) Validate(spec string) error {
	_, err := cron.ParseStandard(spec)
	return err
}

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

func (c *Cron) Remove(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if id, ok := c.entries[name]; ok {
		c.engine.Remove(id)
		delete(c.entries, name)
	}
}

func (c *Cron) Start() { c.engine.Start() }
func (c *Cron) Stop()  { c.engine.Stop() }
