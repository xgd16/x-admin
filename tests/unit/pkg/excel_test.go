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

func TestWriteInvalidSheetName(t *testing.T) {
	_, err := excel.Write("bad/name", []string{"A"}, nil)
	require.Error(t, err)
}

func TestReadTrimsTrailingEmptyRows(t *testing.T) {
	data, err := excel.Write("", []string{"A", "B"}, [][]string{{"1", "2"}, {"", ""}})
	require.NoError(t, err)
	rows, err := excel.Read(bytes.NewReader(data))
	require.NoError(t, err)
	require.Len(t, rows, 2) // 末尾全空行被裁剪
	assert.Equal(t, []string{"1", "2"}, rows[1])
}

func TestStreamBookWriteAndRead(t *testing.T) {
	book, err := excel.NewStreamBook("用户列表:beta", []string{"A", "B"})
	require.NoError(t, err)
	defer book.Close()
	require.NoError(t, book.WriteRow([]any{1, "x"}))
	require.NoError(t, book.WriteRow([]any{2, "y"}))
	assert.Equal(t, int64(2), book.Rows())

	var buf bytes.Buffer
	require.NoError(t, book.SaveTo(&buf))
	rows, err := excel.Read(&buf)
	require.NoError(t, err)
	// 非法字符的工作表名被清洗，且默认的空 Sheet1 已移除。
	assert.Equal(t, [][]string{{"A", "B"}, {"1", "x"}, {"2", "y"}}, rows)
}
