// stream.go 基于 excelize 的 StreamWriter 实现逐行流式写入，
// 内存占用与总行数无关，供异步导出等大数据量场景使用。
package excel

import (
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"
)

// maxSheetRows 是单个工作表可容纳的数据行数（扣除表头行）。
const maxSheetRows = excelize.TotalRows - 1

// StreamBook 是一个逐行写入的工作簿。
// 单个工作表写满后自动追加新工作表继续写入（表头随之重复）。
type StreamBook struct {
	f       *excelize.File
	name    string               // 基础工作表名（已清洗）
	headers []string
	styleID int                  // 表头样式 ID，工作簿级共享
	sheetNo int                  // 已创建的工作表数
	sheet   string               // 当前工作表名
	sw      *excelize.StreamWriter // 当前工作表的流式写
	row     int                  // 当前工作表下一个待写行号（1 基）
	rows    int64                // 累计写入的数据行数（不含表头）
}

// NewStreamBook 创建工作簿并写入第一张工作表的表头。
// name 作为工作表名，超过 31 字符或包含非法字符时会被清洗。
func NewStreamBook(name string, headers []string) (*StreamBook, error) {
	f := excelize.NewFile()
	styleID, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "0B1C2C"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"EAF2FA"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("excel: %w", err)
	}
	b := &StreamBook{f: f, name: sanitizeSheetName(name), headers: headers, styleID: styleID}
	if err := b.addSheet(); err != nil {
		f.Close()
		return nil, err
	}
	return b, nil
}

// addSheet 追加一张工作表并写入表头。
func (b *StreamBook) addSheet() error {
	b.sheetNo++
	sheet := b.name
	if b.sheetNo > 1 {
		sheet = fmt.Sprintf("%s-%d", b.name, b.sheetNo)
	}
	if b.sheetNo == 1 {
		// NewFile 自带空的 Sheet1：沿用或删除，保证工作簿里只有业务工作表。
		if sheet != "Sheet1" {
			if _, err := b.f.NewSheet(sheet); err != nil {
				return fmt.Errorf("excel: %w", err)
			}
			if err := b.f.DeleteSheet("Sheet1"); err != nil {
				return fmt.Errorf("excel: %w", err)
			}
		}
	} else if _, err := b.f.NewSheet(sheet); err != nil {
		return fmt.Errorf("excel: %w", err)
	}
	sw, err := b.f.NewStreamWriter(sheet)
	if err != nil {
		return fmt.Errorf("excel: %w", err)
	}
	if len(b.headers) > 0 {
		// SetColWidth 必须在首次 SetRow 之前调用。
		if err := sw.SetColWidth(1, len(b.headers), 18); err != nil {
			return fmt.Errorf("excel: %w", err)
		}
		cells := make([]any, len(b.headers))
		for i, h := range b.headers {
			cells[i] = excelize.Cell{StyleID: b.styleID, Value: h}
		}
		if err := sw.SetRow("A1", cells); err != nil {
			return fmt.Errorf("excel: %w", err)
		}
	}
	b.sheet = sheet
	b.sw = sw
	b.row = 2
	return nil
}

// Rows 返回累计写入的数据行数（不含表头）。
func (b *StreamBook) Rows() int64 { return b.rows }

// WriteRow 追加一行数据；当前工作表写满时自动换新工作表。
func (b *StreamBook) WriteRow(cells []any) error {
	if b.row > maxSheetRows+1 {
		if err := b.sw.Flush(); err != nil {
			return fmt.Errorf("excel: %w", err)
		}
		if err := b.addSheet(); err != nil {
			return err
		}
	}
	cell, err := excelize.CoordinatesToCellName(1, b.row)
	if err != nil {
		return fmt.Errorf("excel: %w", err)
	}
	if err := b.sw.SetRow(cell, cells); err != nil {
		return fmt.Errorf("excel: %w", err)
	}
	b.row++
	b.rows++
	return nil
}

// SaveTo 刷新缓冲并把工作簿写入 w。调用后本对象不可再写。
func (b *StreamBook) SaveTo(w io.Writer) error {
	if err := b.sw.Flush(); err != nil {
		return fmt.Errorf("excel: %w", err)
	}
	if err := b.f.Write(w); err != nil {
		return fmt.Errorf("excel: %w", err)
	}
	return nil
}

// Close 释放工作簿占用的临时资源。
func (b *StreamBook) Close() error { return b.f.Close() }

// sanitizeSheetName 清洗工作表名：去除 Excel 不允许的字符并限制在 31 字符内。
func sanitizeSheetName(name string) string {
	out := make([]rune, 0, 31)
	for _, r := range name {
		switch r {
		case ':', '\\', '/', '?', '*', '[', ']', '\'', '<', '>':
			continue
		}
		out = append(out, r)
		if len(out) == 28 { // 预留 "-999" 后缀的空间
			break
		}
	}
	if len(out) == 0 {
		return "Sheet1"
	}
	return string(out)
}
