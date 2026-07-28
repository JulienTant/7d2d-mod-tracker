package main

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

func TestResponsiveGridFillsAvailableWidth(t *testing.T) {
	grid := newResponsiveGridLayout(400, 200)
	objects := []fyne.CanvasObject{
		canvas.NewRectangle(nil),
		canvas.NewRectangle(nil),
		canvas.NewRectangle(nil),
	}

	grid.Layout(objects, fyne.NewSize(1000, 400))

	if objects[0].Size().Width != 500 || objects[1].Size().Width != 500 {
		t.Fatalf("first row widths are %v and %v, want 500", objects[0].Size().Width, objects[1].Size().Width)
	}
	if objects[2].Size().Width != 1000 {
		t.Fatalf("last row width is %v, want 1000", objects[2].Size().Width)
	}
	if objects[2].Position() != (fyne.Position{X: 0, Y: 200}) {
		t.Fatalf("last item position is %v", objects[2].Position())
	}
}

func TestResponsiveGridUsesOneColumnBelowMinimumWidth(t *testing.T) {
	grid := newResponsiveGridLayout(400, 200)
	objects := []fyne.CanvasObject{
		canvas.NewRectangle(nil),
		canvas.NewRectangle(nil),
	}

	grid.Layout(objects, fyne.NewSize(350, 400))

	if objects[0].Size().Width != 350 || objects[1].Position().Y != 200 {
		t.Fatalf("objects were not arranged as one full-width column")
	}
}
