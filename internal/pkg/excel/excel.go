package excel

import (
	"bytes"
	"fmt"
	"io"

	"github.com/xuri/excelize/v2"
)

// Write builds a single-sheet xlsx with a styled header row.
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

// Read returns the first sheet's rows, trimming fully-empty trailing rows.
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

func isEmpty(row []string) bool {
	for _, cell := range row {
		if cell != "" {
			return false
		}
	}
	return true
}
