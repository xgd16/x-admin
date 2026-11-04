package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTriggerValue(t *testing.T) {
	assert.Equal(t, "cron", triggerValue(nil))
	assert.Equal(t, "cron", triggerValue([]string{""}))
	assert.Equal(t, "manual", triggerValue([]string{"manual"}))
	assert.Equal(t, "cron", triggerValue([]string{"anything"}))
}
