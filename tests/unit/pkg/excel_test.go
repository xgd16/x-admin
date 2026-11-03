package pkg_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"x-admin/internal/pkg/excel"
)

func TestWriteAndRead(t *testing.T) {
	data, err := excel.Write("用户", []string{"用户名", "昵称"}, [][]string{{"alice", "Alice"}, {"bob", "Bob"}})
	require.NoError(t, err)
	require.NotEmpty(t, data)

	rows, err := excel.Read(bytes.NewReader(data))
	require.NoError(t, err)
	require.Len(t, rows, 3)
	assert.Equal(t, []string{"用户名", "昵称"}, rows[0])
	assert.Equal(t, []string{"alice", "Alice"}, rows[1])
	assert.Equal(t, []string{"bob", "Bob"}, rows[2])
}

func TestWriteDefaultSheetAndTrimsRows(t *testing.T) {
	data, err := excel.Write("", []string{"A"}, [][]string{{"1"}})
	require.NoError(t, err)
	rows, err := excel.Read(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"A"}, {"1"}}, rows)
}

func TestReadInvalidWorkbook(t *testing.T) {
	_, err := excel.Read(bytes.NewReader([]byte("not excel")))
	require.Error(t, err)
}
