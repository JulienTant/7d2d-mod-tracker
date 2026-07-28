package main

import "fyne.io/fyne/v2"

type responsiveGridLayout struct {
	minCellWidth float32
	cellHeight   float32
	rowCount     int
}

func newResponsiveGridLayout(minCellWidth, cellHeight float32) *responsiveGridLayout {
	return &responsiveGridLayout{
		minCellWidth: minCellWidth,
		cellHeight:   cellHeight,
		rowCount:     1,
	}
}

func (g *responsiveGridLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var visible []fyne.CanvasObject
	for _, object := range objects {
		if object.Visible() {
			visible = append(visible, object)
		}
	}
	if len(visible) == 0 {
		g.rowCount = 1
		return
	}

	columns := int(size.Width / g.minCellWidth)
	if columns < 1 {
		columns = 1
	}
	if columns > len(visible) {
		columns = len(visible)
	}

	g.rowCount = (len(visible) + columns - 1) / columns
	for row, first := 0, 0; first < len(visible); row, first = row+1, first+columns {
		rowItems := min(columns, len(visible)-first)
		cellWidth := size.Width / float32(rowItems)
		for column := 0; column < rowItems; column++ {
			object := visible[first+column]
			object.Move(fyne.NewPos(float32(column)*cellWidth, float32(row)*g.cellHeight))
			object.Resize(fyne.NewSize(cellWidth, g.cellHeight))
		}
	}
}

func (g *responsiveGridLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(g.minCellWidth, float32(g.rowCount)*g.cellHeight)
}
