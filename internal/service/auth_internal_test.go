package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRefreshValueVariants(t *testing.T) {
	userID, version, sid, remember, err := parseRefreshValue("42:3:sid:1")
	assert.NoError(t, err)
	assert.Equal(t, uint64(42), userID)
	assert.Equal(t, int64(3), version)
	assert.Equal(t, "sid", sid)
	assert.True(t, remember)

	userID, version, sid, remember, err = parseRefreshValue("42:0:sid")
	assert.NoError(t, err)
	assert.Equal(t, uint64(42), userID)
	assert.Equal(t, int64(0), version)
	assert.Equal(t, "sid", sid)
	assert.False(t, remember)

	_, _, _, _, err = parseRefreshValue("bad")
	assert.Error(t, err)
	_, _, _, _, err = parseRefreshValue("42:not-a-number:sid:0")
	assert.Error(t, err)
}

func TestDeviceLabel(t *testing.T) {
	assert.Equal(t, "（移动端）", deviceLabel("Mozilla/5.0 (iPhone)"))
	assert.Equal(t, "（移动端）", deviceLabel("Android"))
	assert.Equal(t, "（macOS）", deviceLabel("Macintosh"))
	assert.Equal(t, "（Windows）", deviceLabel("Windows NT"))
	assert.Equal(t, "（Linux）", deviceLabel("X11; Linux"))
	assert.Empty(t, deviceLabel("curl/8"))
}
