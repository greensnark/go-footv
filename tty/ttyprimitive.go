package tty

func clamp(x, min, max int) int {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}

func (t *Tty) clampCursor(c Pt) Pt {
	return Pt{
		X: clamp(c.X, 0, t.Size.X),
		Y: clamp(c.Y, 0, t.Size.Y),
	}
}

// Clamps cursor strictly within bounds
func (t *Tty) clampCursorStrict(c Pt) Pt {
	return Pt{
		X: clamp(c.X, 0, t.Size.X-1),
		Y: clamp(c.Y, 0, t.Size.Y-1),
	}
}

func (t *Tty) cursorMove(delta Pt) {
	t.Cursor = t.clampCursor(Pt{
		X: t.Cursor.X + delta.X,
		Y: t.Cursor.Y + delta.Y,
	})
}

func (t *Tty) backspace() {
	if t.Cursor.X > 0 {
		t.Cursor.X--
		if t.CursorMoved != nil {
			t.CursorMoved(t, t.Cursor)
		}
	}
}

func (t *Tty) IsTabStop(x int) bool { return (x & 7) == 0 }

func (t *Tty) posOffset(p Pt) int { return t.Size.Offset(p) }
func (t *Tty) maxOffset() int     { return t.Size.Area() }

func (t *Tty) Get(p Pt) AttrChar      { return t.Buf[t.posOffset(p)] }
func (t *Tty) Set(p Pt, ach AttrChar) { t.Buf[t.posOffset(p)] = ach }

func (t *Tty) tab() {
	z := t.DefaultAttrChar()
	for t.Cursor.X < t.Size.X {
		t.Set(t.Cursor, z)
		t.Cursor.X++
		if t.IsTabStop(t.Cursor.X) {
			break
		}
	}
}

func (t *Tty) linefeed() {
	t.carriageReturn()
	t.verticaltab()
}

func (t *Tty) upline() {
	t.Cursor.Y--
	if t.Cursor.Y == t.ScrollRange.Low-1 {
		t.Cursor.Y = t.ScrollRange.Low
		t.Scroll(-1)
	} else {
		if t.Cursor.Y < 0 {
			t.Cursor.Y = 0
		}
	}
}

func (t *Tty) verticaltab() {
	t.Cursor.Y++
	if t.Cursor.Y == t.ScrollRange.High {
		t.Scroll(t.ScrollRange.High - t.Cursor.Y + 1)
		t.Cursor.Y = t.ScrollRange.High - 1
	} else if t.Cursor.Y >= t.Size.Y {
		t.Cursor.Y = t.Size.Y - 1
	}
}

func (t *Tty) formfeed() {
	t.ClearScreen()
}

func (t *Tty) carriageReturn() {
	t.Cursor.X = 0
}

func (t *Tty) clampCursorX() {
	if t.Cursor.X >= t.Size.X {
		if t.AutoWrap {
			t.Cursor.X = 0
			t.verticaltab()
		} else {
			t.Cursor.X = t.Size.X - 1
		}
	}
}
