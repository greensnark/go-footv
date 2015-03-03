package tty

import (
	"bytes"
	"fmt"
)

func (t *Tty) InBounds(p Pt) bool {
	return p.X >= 0 && p.X < t.Size.X && p.Y >= 0 && p.Y < t.Size.Y
}

// TextAtN returns a string of length runes at the given position. If
// the position is outside the terminal, returns the empty string. If
// the requested length is too large, returns the maximum available.
//
// The returned string may contain invalid UTF-8, including the zero
// byte.
func (t *Tty) TextAtN(at Pt, length int) string {
	if !t.InBounds(at) {
		return ""
	}

	out := bytes.Buffer{}
	offset := t.posOffset(at)
	maxOffset := t.Size.Area()
	end := offset + length
	if end > maxOffset {
		end = maxOffset
	}
	for _, c := range t.Buf[offset:end] {
		out.WriteRune(c.Ch)
	}
	return out.String()
}

func (t *Tty) WriteString(content string) {
	t.Write([]byte(content))
}

func (t *Tty) Write(content []byte) {
	for _, b := range content {
		t.consumeByte(b)
	}
}

func (t *Tty) ClearRegion(start, length int) {
	zero := t.DefaultAttrChar()
	region := t.Buf[start : length+start]
	for i := range region {
		region[i] = zero
	}
}

func (t *Tty) Resize(newsize Pt) {
	if newsize == t.Size {
		return
	}
	oldsize := t.Size
	t.debug(fmt.Sprintf("Resize from %s -> %s",
		oldsize.String(), newsize.String()))

	oldbuf := t.Buf

	t.Size = newsize
	t.Buf = t.allocBuf(t.Size)
	t.ClearRegion(0, newsize.Area())

	var oldOffset, newOffset int
	copysize := PointMin(oldsize, newsize)
	for y := 0; y < copysize.Y; y++ {
		copy(t.Buf[newOffset:newOffset+copysize.X],
			oldbuf[oldOffset:oldOffset+copysize.X])
		oldOffset += oldsize.X
		newOffset += newsize.X
	}
	t.ScrollRange = Range{Low: 0, High: newsize.Y}
	t.Cursor.X = clamp(t.Cursor.X, 0, newsize.X)
	t.Cursor.Y = clamp(t.Cursor.Y, 0, newsize.Y-1)
}

func (t *Tty) ClearScreen() {
	t.Cursor = Pt{}
	t.ClearRegion(0, t.Size.Area())
}

func (t *Tty) Reset() {
	t.Cursor = Pt{}
	t.Attr = 0x1010
	t.CursorVisible = true
	t.AutoWrap = true
	t.Kpad = false
	t.ScrollRange = Range{0, t.Size.Y}
	t.savedCursor = Pt{}
	t.csetShift = 0
	t.csetSelect = 1 << 1
	t.utfCount = 0
	t.ClearRegion(0, t.bufSize())
	t.changeState(VTNorm)
	t.clearParState()
}
