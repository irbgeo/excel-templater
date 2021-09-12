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
// TODO: adjustPageBreaks, adjustComments, adjustDataValidations, adjustProtectedCells
//
func (f *File) adjustHelper(sheet string, dir adjustDirection, num, offset int) error {
	ws, err := f.workSheetReader(sheet)
	if err != nil {
		return err
	}
	sheetID := f.getSheetID(sheet)
	if dir == rows {
		f.adjustRowDimensions(ws, num, offset)
	} else {
		f.adjustColDimensions(ws, num, offset)
	}
	f.adjustHyperlinks(ws, sheet, dir, num, offset)
	if err = f.adjustMergeCells(ws, dir, num, offset); err != nil {
		return err
	}
	if err = f.adjustAutoFilter(ws, dir, num, offset); err != nil {
		return err
	}
	if err = f.adjustCalcChain(dir, num, offset, sheetID); err != nil {
		return err
	}
	checkSheet(ws)
	_ = checkRow(ws)

	if ws.MergeCells != nil && len(ws.MergeCells.Cells) == 0 {
		ws.MergeCells = nil
	}

	return nil
}

// adjustColDimensions provides a function to update column dimensions when
// inserting or deleting rows or columns.
func (f *File) adjustColDimensions(ws *xlsxWorksheet, col, offset int) {
	for rowIdx := range ws.SheetData.Row {
		for colIdx, v := range ws.SheetData.Row[rowIdx].C {
			cellCol, cellRow, _ := CellNameToCoordinates(v.R)
			if col <= cellCol {
				if newCol := cellCol + offset; newCol > 0 {
					ws.SheetData.Row[rowIdx].C[colIdx].R, _ = CoordinatesToCellName(newCol, cellRow)
				}
			}
		}
	}
}

// adjustRowDimensions provides a function to update row dimensions when
// inserting or deleting rows or columns.
func (f *File) adjustRowDimensions(ws *xlsxWorksheet, row, offset int) {
	for i := range ws.SheetData.Row {
		r := &ws.SheetData.Row[i]
		if newRow := r.R + offset; r.R >= row && newRow > 0 {
			f.ajustSingleRowDimensions(r, newRow)
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
func (f *File) adjustHyperlinks(ws *xlsxWorksheet, sheet string, dir adjustDirection, num, offset int) {
	// short path
	if ws.Hyperlinks == nil || len(ws.Hyperlinks.Hyperlink) == 0 {
		return
	}

	// order is important
	if offset < 0 {
		for i := len(ws.Hyperlinks.Hyperlink) - 1; i >= 0; i-- {
			linkData := ws.Hyperlinks.Hyperlink[i]
			colNum, rowNum, _ := CellNameToCoordinates(linkData.Ref)

			if (dir == rows && num == rowNum) || (dir == columns && num == colNum) {
				f.deleteSheetRelationships(sheet, linkData.RID)
				if len(ws.Hyperlinks.Hyperlink) > 1 {
					ws.Hyperlinks.Hyperlink = append(ws.Hyperlinks.Hyperlink[:i],
						ws.Hyperlinks.Hyperlink[i+1:]...)
				} else {
					ws.Hyperlinks = nil
				}
			}
		}
	}
	if ws.Hyperlinks == nil {
		return
	}
	for i := range ws.Hyperlinks.Hyperlink {
		link := &ws.Hyperlinks.Hyperlink[i] // get reference
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
func (f *File) adjustAutoFilter(ws *xlsxWorksheet, dir adjustDirection, num, offset int) error {
	if ws.AutoFilter == nil {
		return nil
	}

	coordinates, err := f.areaRefToCoordinates(ws.AutoFilter.Ref)
	if err != nil {
		return err
	}
	x1, y1, x2, y2 := coordinates[0], coordinates[1], coordinates[2], coordinates[3]

	if (dir == rows && y1 == num && offset < 0) || (dir == columns && x1 == num && x2 == num) {
		ws.AutoFilter = nil
		for rowIdx := range ws.SheetData.Row {
			rowData := &ws.SheetData.Row[rowIdx]
			if rowData.R > y1 && rowData.R <= y2 {
				rowData.Hidden = false
			}
		}
		return nil
	}

	coordinates = f.adjustAutoFilterHelper(dir, coordinates, num, offset)
	x1, y1, x2, y2 = coordinates[0], coordinates[1], coordinates[2], coordinates[3]

	if ws.AutoFilter.Ref, err = f.coordinatesToAreaRef([]int{x1, y1, x2, y2}); err != nil {
		return err
	}
	return nil
}

// adjustAutoFilterHelper provides a function for adjusting auto filter to
// compare and calculate cell axis by the given adjust direction, operation
// axis and offset.
func (f *File) adjustAutoFilterHelper(dir adjustDirection, coordinates []int, num, offset int) []int {
	if dir == rows {
		if coordinates[1] >= num {
			coordinates[1] += offset
		}
		if coordinates[3] >= num {
			coordinates[3] += offset
		}
	} else {
		if coordinates[2] >= num {
			coordinates[2] += offset
		}
	}
	return coordinates
}

// areaRefToCoordinates provides a function to convert area reference to a
// pair of coordinates.
func (f *File) areaRefToCoordinates(ref string) ([]int, error) {
	rng := strings.Split(strings.Replace(ref, "$", "", -1), ":")
	if len(rng) < 2 {
		return nil, ErrParameterInvalid
	}

	return areaRangeToCoordinates(rng[0], rng[1])
}

// areaRangeToCoordinates provides a function to convert cell range to a
// pair of coordinates.
func areaRangeToCoordinates(firstCell, lastCell string) ([]int, error) {
	coordinates := make([]int, 4)
	var err error
	coordinates[0], coordinates[1], err = CellNameToCoordinates(firstCell)
	if err != nil {
		return coordinates, err
	}
	coordinates[2], coordinates[3], err = CellNameToCoordinates(lastCell)
	return coordinates, err
}

// sortCoordinates provides a function to correct the coordinate area, such
// correct C1:B3 to B1:C3.
func sortCoordinates(coordinates []int) error {
	if len(coordinates) != 4 {
		return ErrCoordinates
	}
	if coordinates[2] < coordinates[0] {
		coordinates[2], coordinates[0] = coordinates[0], coordinates[2]
	}
	if coordinates[3] < coordinates[1] {
		coordinates[3], coordinates[1] = coordinates[1], coordinates[3]
	}
	return nil
}

// coordinatesToAreaRef provides a function to convert a pair of coordinates
// to area reference.
func (f *File) coordinatesToAreaRef(coordinates []int) (string, error) {
	if len(coordinates) != 4 {
		return "", ErrCoordinates
	}
	firstCell, err := CoordinatesToCellName(coordinates[0], coordinates[1])
	if err != nil {
		return "", err
	}
	lastCell, err := CoordinatesToCellName(coordinates[2], coordinates[3])
	if err != nil {
		return "", err
	}
	return firstCell + ":" + lastCell, err
}

// adjustMergeCells provides a function to update merged cells when inserting
// or deleting rows or columns.
func (f *File) adjustMergeCells(ws *xlsxWorksheet, dir adjustDirection, num, offset int) error {
	if ws.MergeCells == nil {
		return nil
	}

	for i := 0; i < len(ws.MergeCells.Cells); i++ {
		areaData := ws.MergeCells.Cells[i]
		coordinates, err := f.areaRefToCoordinates(areaData.Ref)
		if err != nil {
			return err
		}
		x1, y1, x2, y2 := coordinates[0], coordinates[1], coordinates[2], coordinates[3]
		if dir == rows {
			if y1 == num && y2 == num && offset < 0 {
				f.deleteMergeCell(ws, i)
				i--
			}
			y1 = f.adjustMergeCellsHelper(y1, num, offset)
			y2 = f.adjustMergeCellsHelper(y2, num, offset)
		} else {
			if x1 == num && x2 == num && offset < 0 {
				f.deleteMergeCell(ws, i)
				i--
			}
			x1 = f.adjustMergeCellsHelper(x1, num, offset)
			x2 = f.adjustMergeCellsHelper(x2, num, offset)
		}
		if x1 == x2 && y1 == y2 {
			f.deleteMergeCell(ws, i)
			i--
		}
		if areaData.Ref, err = f.coordinatesToAreaRef([]int{x1, y1, x2, y2}); err != nil {
			return err
		}
	}
	return nil
}

// adjustMergeCellsHelper provides a function for adjusting merge cells to
// compare and calculate cell axis by the given pivot, operation axis and
// offset.
func (f *File) adjustMergeCellsHelper(pivot, num, offset int) int {
	if pivot >= num {
		pivot += offset
		if pivot < 1 {
			return 1
		}
		return pivot
	}
	return pivot
}

// deleteMergeCell provides a function to delete merged cell by given index.
func (f *File) deleteMergeCell(ws *xlsxWorksheet, idx int) {
	if len(ws.MergeCells.Cells) > idx {
		ws.MergeCells.Cells = append(ws.MergeCells.Cells[:idx], ws.MergeCells.Cells[idx+1:]...)
		ws.MergeCells.Count = len(ws.MergeCells.Cells)
	}
}

// adjustCalcChain provides a function to update the calculation chain when
// inserting or deleting rows or columns.
func (f *File) adjustCalcChain(dir adjustDirection, num, offset, sheetID int) error {
	if f.CalcChain == nil {
		return nil
	}
	for index, c := range f.CalcChain.C {
		if c.I != sheetID {
			continue
		}
		colNum, rowNum, err := CellNameToCoordinates(c.R)
		if err != nil {
			return err
		}
		if dir == rows && num <= rowNum {
			if newRow := rowNum + offset; newRow > 0 {
				f.CalcChain.C[index].R, _ = CoordinatesToCellName(colNum, newRow)
			}
		}
		if dir == columns && num <= colNum {
			if newCol := colNum + offset; newCol > 0 {
				f.CalcChain.C[index].R, _ = CoordinatesToCellName(newCol, rowNum)
			}
		}
	}
	return nil
}