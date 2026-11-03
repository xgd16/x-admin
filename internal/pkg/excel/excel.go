// Package excel 封装单工作表的 xlsx 读写，用于用户导入导出。
package excel

import (
	"bytes"
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"
)

// Write 生成单工作表 xlsx，第一行是带样式的表头。
func Write(sheet string, headers []string, rows [][]string) ([]byte, error) {
	if sheet == "" {
		sheet = "Sheet1"
	}
	f := excelize.NewFile()
	defer f.Close()
	idx, err := f.NewSheet(sheet)
	if err != nil {
		return nil, err
	}
	f.SetActiveSheet(idx)
	if sheet != "Sheet1" {
		if err := f.DeleteSheet("Sheet1"); err != nil {
			return nil, err
		}
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}
	// 表头样式：加粗、浅蓝底、水平垂直居中。
	style, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "0B1C2C"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"EAF2FA"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	lastCol, _ := excelize.CoordinatesToCellName(len(headers), 1)
	if err := f.SetCellStyle(sheet, "A1", lastCol, style); err != nil {
		return nil, err
	}
	for r, row := range rows {
		for c, value := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			if err := f.SetCellValue(sheet, cell, value); err != nil {
				return nil, err
			}
		}
	}
	if len(headers) > 0 {
		_ = f.SetColWidth(sheet, "A", lastCol, 18)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Read 读取第一个工作表的全部行，并去掉末尾的空行。
func Read(r io.Reader) ([][]string, error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return nil, fmt.Errorf("excel: %w", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("excel: empty workbook")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, err
	}
	for len(rows) > 0 && isEmpty(rows[len(rows)-1]) {
		rows = rows[:len(rows)-1]
	}
	return rows, nil
}

// isEmpty 判断一行是否所有单元格都为空。
func isEmpty(row []string) bool {
	for _, cell := range row {
		if cell != "" {
			return false
		}
	}
	return true
}
