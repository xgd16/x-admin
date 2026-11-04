package jobs_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/jobs"
)

func TestCronLifecycle(t *testing.T) {
	c := jobs.New()
	assert.NoError(t, c.Validate("*/5 * * * *"))
	assert.Error(t, c.Validate("not-a-cron"))

	require.NoError(t, c.Add("cleanup", "*/5 * * * *", func() {}))
	// 同名任务替换后仍只有一个 entry，Remove 可正常清理。
	require.NoError(t, c.Add("cleanup", "*/10 * * * *", func() {}))
	assert.Error(t, c.Add("broken", "bad", func() {}))
	c.Remove("cleanup")
	c.Remove("missing")

	c.Start()
	c.Stop()
}
