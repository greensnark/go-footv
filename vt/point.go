package vt

import "fmt"

type Pt struct {
	X, Y int
}

func (p Pt) Area() int      { return p.X * p.Y }
func (p Pt) String() string { return fmt.Sprintf("(%d,%d)", p.X, p.Y) }

// Offset(p) returns the offset of p in a canvas of the given size.
func (size Pt) Offset(p Pt) int {
	return p.Y*size.X + p.X
}

// LineOffset(line) returns the offset of the first column of line in
// a canvas of the given size.
func (size Pt) LineOffset(line int) int {
	return line * size.X
}

func ptZero() Pt {
	return Pt{}
}

func intMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// PointMin returns the int minimum of the X and Y coordinates of the
// points a and b. In other words, if a and b are treated as rectangle
// sizes, this returns the size of the intersection of the two
// rectangles.
func PointMin(a, b Pt) Pt {
	return Pt{X: intMin(a.X, b.X), Y: intMin(a.Y, b.Y)}
}

type Range struct {
	Low, High int
}

func (p Range) Span() int { return p.High - p.Low }
