package excelize

import (
	"strings"
)

type adjustDirection bool

const (
	columns adjustDirection = false
	rows    adjustDirection = true
)

// adjustHelper provides a function to adjust rows and columns dimensions,
// hyperlinks, merged cells and auto filter when inserting or deleting rows or
// columns.
//
// sheet: Worksheet name that we're editing
// column: Index number of the column we're inserting/deleting before
// row: Index number of the row we're inserting/deleting before
// offset: Number of rows/column to insert/delete negative values indicate deletion
//
// TODO: adjustCalcChain, adjustPageBreaks, adjustComments,
// adjustDataValidations, adjustProtectedCells
//
func (f *File) adjustHelper(sheet string, dir adjustDirection, num, offset int) {
	xlsx := f.workSheetReader(sheet)

	if dir == rows {
		f.adjustRowDimensions(xlsx, num, offset)
	} else {
		f.adjustColDimensions(xlsx, num, offset)
	}
	f.adjustHyperlinks(xlsx, sheet, dir, num, offset)
	f.adjustMergeCells(xlsx, dir, num, offset)
	f.adjustAutoFilter(xlsx, dir, num, offset)

	checkSheet(xlsx)
	checkRow(xlsx)
}

// adjustColDimensions provides a function to update column dimensions when
// inserting or deleting rows or columns.
func (f *File) adjustColDimensions(xlsx *xlsxWorksheet, col, offset int) {
	for rowIdx := range xlsx.SheetData.Row {
		for colIdx, v := range xlsx.SheetData.Row[rowIdx].C {
			cellCol, cellRow, _ := CellNameToCoordinates(v.R)
			if col <= cellCol {
				if newCol := cellCol + offset; newCol > 0 {
					xlsx.SheetData.Row[rowIdx].C[colIdx].R, _ = CoordinatesToCellName(newCol, cellRow)
				}
			}
		}
	}
}

// adjustRowDimensions provides a function to update row dimensions when
// inserting or deleting rows or columns.
func (f *File) adjustRowDimensions(xlsx *xlsxWorksheet, row, offset int) {
	for i, r := range xlsx.SheetData.Row {
		if newRow := r.R + offset; r.R >= row && newRow > 0 {
			f.ajustSingleRowDimensions(&xlsx.SheetData.Row[i], newRow)
		}
	}
}

// ajustSingleRowDimensions provides a function to ajust single row dimensions.
func (f *File) ajustSingleRowDimensions(r *xlsxRow, num int) {
	r.R = num
	for i, col := range r.C {
		colName, _, _ := SplitCellName(col.R)
		r.C[i].R, _ = JoinCellName(colName, num)
	}
}

// adjustHyperlinks provides a function to update hyperlinks when inserting or
// deleting rows or columns.
func (f *File) adjustHyperlinks(xlsx *xlsxWorksheet, sheet string, dir adjustDirection, num, offset int) {
	// short path
	if xlsx.Hyperlinks == nil || len(xlsx.Hyperlinks.Hyperlink) == 0 {
		return
	}

	// order is important
	if offset < 0 {
		for rowIdx, linkData := range xlsx.Hyperlinks.Hyperlink {
			colNum, rowNum, _ := CellNameToCoordinates(linkData.Ref)

			if (dir == rows && num == rowNum) || (dir == columns && num == colNum) {
				f.deleteSheetRelationships(sheet, linkData.RID)
				if len(xlsx.Hyperlinks.Hyperlink) > 1 {
					xlsx.Hyperlinks.Hyperlink = append(xlsx.Hyperlinks.Hyperlink[:rowIdx],
						xlsx.Hyperlinks.Hyperlink[rowIdx+1:]...)
				} else {
					xlsx.Hyperlinks = nil
				}
			}
		}
	}

	if xlsx.Hyperlinks == nil {
		return
	}

	for i := range xlsx.Hyperlinks.Hyperlink {
		link := &xlsx.Hyperlinks.Hyperlink[i] // get reference
		colNum, rowNum, _ := CellNameToCoordinates(link.Ref)

		if dir == rows {
			if rowNum >= num {
				link.Ref, _ = CoordinatesToCellName(colNum, rowNum+offset)
			}
		} else {
			if colNum >= num {
				link.Ref, _ = CoordinatesToCellName(colNum+offset, rowNum)
			}
		}
	}
}

// adjustAutoFilter provides a function to update the auto filter when
// inserting or deleting rows or columns.
func (f *File) adjustAutoFilter(xlsx *xlsxWorksheet, dir adjustDirection, num, offset int) {
	if xlsx.AutoFilter == nil {
		return
	}

	rng := strings.Split(xlsx.AutoFilter.Ref, ":")
	firstCell := rng[0]
	lastCell := rng[1]

	firstCol, firstRow, err := CellNameToCoordinates(firstCell)
	if err != nil {
		panic(err)
	}

	lastCol, lastRow, err := CellNameToCoordinates(lastCell)
	if err != nil {
		panic(err)
	}

	if (dir == rows && firstRow == num && offset < 0) || (dir == columns && firstCol == num && lastCol == num) {
		xlsx.AutoFilter = nil
		for rowIdx := range xlsx.SheetData.Row {
			rowData := &xlsx.SheetData.Row[rowIdx]
			if rowData.R > firstRow && rowData.R <= lastRow {
				rowData.Hidden = false
			}
		}
		return
	}

	if dir == rows {
		if firstRow >= num {
			firstCell, _ = CoordinatesToCellName(firstCol, firstRow+offset)
		}
		if lastRow >= num {
			lastCell, _ = CoordinatesToCellName(lastCol, lastRow+offset)
		}
	} else {
		if lastCol >= num {
			lastCell, _ = CoordinatesToCellName(lastCol+offset, lastRow)
		}
	}

	xlsx.AutoFilter.Ref = firstCell + ":" + lastCell
}

// adjustMergeCells provides a function to update merged cells when inserting
// or deleting rows or columns.
func (f *File) adjustMergeCells(xlsx *xlsxWorksheet, dir adjustDirection, num, offset int) {
	if xlsx.MergeCells == nil {
		return
	}

	for i, areaData := range xlsx.MergeCells.Cells {
		rng := strings.Split(areaData.Ref, ":")
		firstCell := rng[0]
		lastCell := rng[1]

		firstCol, firstRow, err := CellNameToCoordinates(firstCell)
		if err != nil {
			panic(err)
		}

		lastCol, lastRow, err := CellNameToCoordinates(lastCell)
		if err != nil {
			panic(err)
		}

		adjust := func(v int) int {
			if v >= num {
				v += offset
				if v < 1 {
					return 1
				}
				return v
			}
			return v
		}

		if dir == rows {
			firstRow = adjust(firstRow)
			lastRow = adjust(lastRow)
		} else {
			firstCol = adjust(firstCol)
			lastCol = adjust(lastCol)
		}

		if firstCol == lastCol && firstRow == lastRow {
			if len(xlsx.MergeCells.Cells) > 1 {
				xlsx.MergeCells.Cells = append(xlsx.MergeCells.Cells[:i], xlsx.MergeCells.Cells[i+1:]...)
				xlsx.MergeCells.Count = len(xlsx.MergeCells.Cells)
			} else {
				xlsx.MergeCells = nil
			}
		}

		if firstCell, err = CoordinatesToCellName(firstCol, firstRow); err != nil {
			panic(err)
		}

		if lastCell, err = CoordinatesToCellName(lastCol, lastRow); err != nil {
			panic(err)
		}

		areaData.Ref = firstCell + ":" + lastCell
	}
}
