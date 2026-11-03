package pkg_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"x-admin/internal/jobs"
)

func TestCronScheduler(t *testing.T) {
	c := jobs.New()
	require.NoError(t, c.Validate("0 3 * * *"))
	require.Error(t, c.Validate("not a cron"))

	require.NoError(t, c.Add("job:1", "0 3 * * *", func() {}))
	require.NoError(t, c.Add("job:1", "0 4 * * *", func() {}))
	require.Error(t, c.Add("job:2", "bad", func() {}))

	c.Remove("missing")
	c.Remove("job:1")
	c.Start()
	c.Stop()
}
