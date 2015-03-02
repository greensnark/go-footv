// Package tty provides a virtual terminal. Based on
// https://github.com/kilobyte/termrec
package tty

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"github.com/greensnark/go-footv/cset"
	"github.com/greensnark/go-footv/unicode"
)

type Attribute uint32

type AttrChar struct {
	Attr Attribute
	Ch   rune
}

// Tty represents a terminal state, corresponding to the vt100
// definition in termrec.
type Tty struct {
	Size          Pt // aka sx, sy in termrec
	Cursor        Pt // aka cx, cy
	CursorVisible bool
	Attr          Attribute // aka attr
	ScrollRange   Range     // aka s1, s2

	Debug     bool
	State     VTMode     // aka state
	Buf       []AttrChar // aka scr
	Resizable bool       // aka opt_allow_resize
	AutoWrap  bool       // aka opt_auto_wrap
	Kpad      bool       // aka opt_kpad
	UTF8      bool       // aka utf

	csetSelect  int  // aka G
	csetShift   uint // aka curG in termrec
	utfChar     rune // aka utf_char
	utfCount    int  // aka utf_count
	savedCursor Pt   // aka save_cx, save_cy
	stateProc   func(byte)
	stateTok    []byte

	CursorMoved func(*Tty, Pt)
	CharWritten func(*Tty, Pt, AttrChar)
	Cleared     func(*Tty, Pt, int)
	Scrolled    func(*Tty, int)
	Resized     func(tty *Tty, oldSz, newSz Pt)
	Flushed     func(*Tty)
}

func defaultTtySize() Pt { return Pt{X: 80, Y: 24} }

func New() *Tty {
	tty := &Tty{
		Size:       defaultTtySize(),
		UTF8:       true,
		csetSelect: 1 << 1,
		stateTok:   make([]byte, 1, 10),
	}
	tty.init()
	return tty
}

func (t *Tty) InDECCset() bool {
	return (t.csetSelect & (1 << t.csetShift)) != 0
}

func (t *Tty) bufSize() int              { return t.Size.Area() }
func (t *Tty) allocBuf(sz Pt) []AttrChar { return make([]AttrChar, sz.Area()) }

func (t *Tty) init() {
	t.Buf = t.allocBuf(t.Size)
	t.Reset()
}

func (t *Tty) DefaultAttrChar() AttrChar {
	return AttrChar{Attr: t.Attr, Ch: ' '}
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
	t.CursorVisible = true
	t.ScrollRange = Range{0, t.Size.Y}
	t.savedCursor = Pt{}
	t.csetShift = 0
	t.csetSelect = 1 << 1
	t.ClearRegion(0, t.bufSize())
	t.changeState(VTNorm)
	t.clearParState()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (t *Tty) Scroll(scrolledLines int) {
	scrollRegionSize := t.ScrollRange.Span()
	absScrolledLines := abs(scrolledLines)

	preservedLines := scrollRegionSize - absScrolledLines
	if preservedLines <= 0 {
		t.ClearRegion(t.ScrollRange.Low*t.Size.X, scrollRegionSize*t.Size.X)
		return
	}

	preservedCharacters := preservedLines * t.Size.X
	if scrolledLines < 0 {
		targetOffset := t.posOffset(Pt{0, t.ScrollRange.Low - scrolledLines})
		sourceOffset := t.posOffset(Pt{0, t.ScrollRange.Low})
		copy(t.Buf[targetOffset:preservedCharacters+targetOffset],
			t.Buf[sourceOffset:preservedCharacters+sourceOffset])
		t.ClearRegion(t.posOffset(Pt{0, t.ScrollRange.Low}),
			absScrolledLines*t.Size.X)
	} else {
		targetOffset := t.posOffset(Pt{0, t.ScrollRange.Low})
		sourceOffset := t.posOffset(Pt{0, t.ScrollRange.Low + scrolledLines})
		copy(t.Buf[targetOffset:targetOffset+preservedCharacters],
			t.Buf[sourceOffset:sourceOffset+preservedCharacters])
		t.ClearRegion(t.posOffset(Pt{0, t.ScrollRange.High - scrolledLines}),
			scrolledLines*t.Size.X)
	}
}

func (t *Tty) Write(content []byte) {
	for _, b := range content {
		t.consumeByte(b)
	}
}

func (t *Tty) consumeByte(b byte) {
	if t.anyStateConsume(b) {
		return
	}
	t.stateProc(b)
}

// anyStateConsume attempts to consume a single byte, and returns true
// if it did. If false, the byte must be processed by the state proc.
func (t *Tty) anyStateConsume(b byte) bool {
	switch b {
	case 0, 127: // no-ops
	case 7:
		if t.State == VTOsc {
			t.changeState(VTNorm)
		}
	case 8:
		t.backspace()
	case 9:
		t.tab()
	case 10:
		t.linefeed()
	case 11:
		t.verticaltab()
	case 12:
		t.formfeed()
	case 13:
		t.carriageReturn()
	case 14:
		t.csetShift = 1
	case 15:
		t.csetShift = 0
	case 24:
	case 26:
		t.changeState(VTNorm)
	case 27:
		t.changeState(VTEsc)
	default:
		return false
	}
	return true
}

func (t *Tty) changeState(newState VTMode) {
	if t.State == VTGetPars {
		t.clearParState()
	}
	t.State = newState
	t.stateProc = t.currentStateProc(t.State)
}

func (t *Tty) currentStateProc(state VTMode) func(byte) {
	switch t.State {
	case VTNorm:
		return t.consumeNorm
	case VTEsc:
		return t.consumeEsc
	case VTSquare:
		return t.consumeEscSquare
	case VTPercent:
		return t.consumeEscPercent
	case VTGetPars:
		return t.consumeEscGetPars
	case VTQues:
		return t.consumeEscQues
	case VTSetG0:
		return t.consumeSetG0
	case VTSetG1:
		return t.consumeSetG1
	}
	return t.consumeNorm
}

func (t *Tty) consumeNorm(b byte) {
	if t.UTF8 {
		t.consumeUTF8(b)
	} else {
		t.consumeCp437(b)
	}
}

func (t *Tty) consumeUTF8(b byte) {
	if b > 0x7f {
		if t.utfCount > 0 && (b&0xc0) == 0x80 {
			t.utfChar = (t.utfChar << 6) | (rune(b) & 0x3f)
			t.utfCount--
			if t.utfCount <= 0 {
				t.applyRune(unicode.NormalizeMultibyte(t.utfChar))
			}
		} else {
			set := func(c int, ch byte) {
				t.utfCount = c
				t.utfChar = rune(ch)
			}
			if (b & 0xe0) == 0xc0 {
				set(1, b&0x1f)
			} else if (b & 0xf0) == 0xe0 {
				set(2, b&0xf)
			} else if (b & 0xf8) == 0xf0 {
				set(3, b&0x7)
			} else if (b & 0xfc) == 0xf8 {
				set(4, b&0x3)
			} else if (b & 0xfe) == 0xfc {
				set(5, b&0x1)
			} else {
				set(0, 0)
			}
		}
	} else {
		t.utfCount = 0
		t.applyRune(rune(b))
	}
}

func (t *Tty) consumeCp437(b byte) {
	t.applyRune(cset.Cp437[b])
}

func (t *Tty) applyRune(c rune) {
	if unicode.IsBOM(c) {
		return
	}
	if c > 31 {
		if c < 128 && t.InDECCset() {
			c = cset.VT100[c]
		}
		t.clampCursorX()
		t.Buf[t.posOffset(t.Cursor)] = AttrChar{
			Attr: t.Attr,
			Ch:   c,
		}
		t.Cursor.X++
	}
}

func (t *Tty) consumeSetG0(b byte) {
	t.setCharset(0, b)
	t.changeState(VTNorm)
}

func (t *Tty) consumeSetG1(b byte) {
	t.setCharset(1, b)
	t.changeState(VTNorm)
}

func (t *Tty) setCharset(g uint, b byte) {
	switch b {
	case '0':
		t.csetSelect |= 1 << g
	case 'B', 'U':
		t.csetSelect &= ^(1 << g)
	}
}

func (t *Tty) consumeEsc(b byte) {
	switch b {
	case '[':
		t.changeState(VTSquare)
		t.clearParState()
	case ']':
		t.changeState(VTOsc)
	case '(':
		t.changeState(VTSetG0)
	case ')':
		t.changeState(VTSetG1)
	case '%':
		t.changeState(VTPercent)
	case '7':
		t.savedCursor = t.Cursor
		t.changeState(VTNorm)
	case '8':
		t.Cursor = t.savedCursor
		t.changeState(VTNorm)
	case 'D':
		t.changeState(VTNorm)
		t.verticaltab()
	case 'E':
		t.changeState(VTNorm)
		t.linefeed()
	case 'M':
		t.changeState(VTNorm)
		t.upline()
	case '=': // application keypad mode
		t.changeState(VTNorm)
		t.Kpad = true
	case '>': // numeric keypad mode
		t.changeState(VTNorm)
		t.Kpad = false
	default:
		t.err(b)
	}
}

func (t *Tty) consumeEscPercent(b byte) {
	switch b {
	case '@': // turn off UTF-8
		t.UTF8 = false
	case '8', 'G':
		t.UTF8 = true
	}
	t.changeState(VTNorm)
}

func (t *Tty) consumeEscSquare(b byte) {
	if b == '?' {
		t.changeState(VTQues)
		return
	}
	t.changeState(VTGetPars)
	t.consumeByte(b)
}

func minMove(b byte, min int) int {
	if int(b) < min {
		return min
	}
	return int(b)
}

func (t *Tty) applyParameterByte(b byte) bool {
	switch b {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		i := len(t.stateTok) - 1
		t.stateTok[i] = t.stateTok[i]*10 + (b - '0')
		return true
	case ';':
		if t.isStateFull() {
			t.err(b)
			return true
		}
		t.stateTok = append(t.stateTok, 0)
		return true
	}
	return false
}

func (t *Tty) consumeEscGetPars(b byte) {
	if t.applyParameterByte(b) {
		return
	}
	switch b {
	case 'm':
		t.applyParAttrs(t.stateTok)
	case 'D':
		t.cursorMove(Pt{X: -minMove(t.stateTok[0], 1)})
	case 'C', 'a':
		t.cursorMove(Pt{X: minMove(t.stateTok[0], 1)})
	case 'A':
		t.cursorMove(Pt{Y: -minMove(t.stateTok[0], 1)})
	case 'B':
		t.cursorMove(Pt{Y: minMove(t.stateTok[0], 1)})
	case 'r': // set scrolling region
		scrollMin := minMove(t.stateTok[0], 1)
		scrollMax := t.Size.Y
		if len(t.stateTok) > 1 && t.stateTok[1] > 0 {
			scrollMax = int(t.stateTok[1])
		}
		if scrollMax <= t.Size.Y && scrollMin < scrollMax {
			t.ScrollRange = Range{Low: scrollMin, High: scrollMax}
			t.Cursor = Pt{Y: scrollMin}
		}
	case 'J': // clear screen
		switch t.stateTok[0] {
		case 0: // from cursor
			offset := t.posOffset(t.Cursor)
			t.ClearRegion(offset, t.maxOffset()-offset)
		case 1: // to cursor
			t.ClearRegion(0, t.posOffset(t.Cursor))
		case 2: // full screen
			t.ClearRegion(0, t.maxOffset())
		}
	case 'K': // clear line
		switch t.stateTok[0] {
		case 0: // from cursor
			t.ClearRegion(t.posOffset(t.Cursor), t.Size.X-t.Cursor.X)
		case 1: // to cursor
			t.ClearRegion(t.posOffset(Pt{Y: t.Cursor.Y}), t.Cursor.X)
		case 2:
			t.ClearRegion(t.posOffset(Pt{Y: t.Cursor.Y}), t.Size.X)
		}
	case 'L': // insert line
		if t.InScrollingRegion() {
			t.scrollExcursion(func() {
				t.Scroll(-minMove(t.stateTok[0], 1))
			})
		}
	case 'M': // delete line
		if t.InScrollingRegion() {
			t.scrollExcursion(func() {
				t.Scroll(minMove(t.stateTok[0], 1))
			})
		}
	case 'X': // erase to the right
		eraseSize := minMove(t.stateTok[0], 1)
		if eraseSize+t.Cursor.X > t.Size.X {
			eraseSize = t.Size.X - t.Cursor.X
		}
		t.ClearRegion(t.posOffset(t.Cursor), eraseSize)
	case 'f', 'H': // move cursor
		t.Cursor = t.clampCursorStrict(Pt{
			X: t.stateN(1) - 1,
			Y: t.stateN(0) - 1,
		})
	case 'G', '`': // move cursor horizontally
		t.Cursor.X = clamp(int(t.stateTok[0])-1, 0, t.Size.X-1)
	case 'd':
		t.Cursor.Y = clamp(int(t.stateTok[0])-1, 0, t.Size.Y-1)
	case 'c': // power on defaults
		t.Reset()
	case 't':
		switch t.stateTok[0] {
		case 8: // \e[8;<h>;<w>t -> resize window
			if !t.Resizable {
				break
			}
			t.Resize(Pt{
				X: t.stateNDef(2, t.Size.X),
				Y: t.stateNDef(1, t.Size.Y),
			})
		}
	}
	t.changeState(VTNorm)
}

func (t *Tty) consumeEscQues(b byte) {
	if t.applyParameterByte(b) {
		return
	}
	switch b {
	case 'h': // set options
		t.applyParOptions(t.stateTok, true)
	case 'l': // unset options
		t.applyParOptions(t.stateTok, false)
	}
	t.changeState(VTNorm)
}

func (t *Tty) stateN(index int) int {
	return t.stateNDef(index, 0)
}

func (t *Tty) stateNDef(index, defval int) int {
	if index < len(t.stateTok) {
		return int(t.stateTok[index])
	}
	return defval
}

func (t *Tty) scrollExcursion(action func()) {
	scrollMin := t.ScrollRange.Low
	t.ScrollRange.Low = t.Cursor.Y
	defer func() { t.ScrollRange.Low = scrollMin }()
	action()
}

func (t *Tty) InScrollingRegion() bool {
	return t.Cursor.Y >= t.ScrollRange.Low && t.Cursor.Y < t.ScrollRange.High
}

func (t *Tty) applyParOptions(attrs []byte, set bool) {
	for _, attr := range attrs {
		switch attr {
		case 7:
			t.AutoWrap = set
		case 26:
			t.CursorVisible = set
		}
	}
}

func (t *Tty) applyParAttrs(attrs []byte) {
	mode := AttrModeNorm
	for _, attr := range attrs {
		mode = t.applyParAttr(mode, attr)
	}
}

func (t *Tty) applyParAttr(mode AttrMode, attr byte) AttrMode {
	switch mode {
	case AttrMode38a:
		if attr != 5 {
			return AttrModeNorm
		}
		return AttrMode38b
	case AttrMode38b:
		if attr == 16 {
			t.Attr &= ^Attribute(0xff)
		} else {
			t.Attr = (t.Attr & ^Attribute(0xff)) | (Attribute(attr) & 0xff)
		}
		return AttrModeNorm
	case AttrMode48a:
		if attr != 5 {
			return AttrModeNorm
		}
		return AttrMode48b
	case AttrMode48b:
		if attr == 16 {
			t.Attr &= ^Attribute(0xff00)
		} else {
			t.Attr = (t.Attr & ^Attribute(0xff00)) | (Attribute(attr) << 8)
		}
		return AttrModeNorm
	}

	switch attr {
	case 0:
		t.Attr = 0x1010
	case 1:
		t.Attr |= VT100AttrBold
		t.Attr &= ^VT100AttrDim
	case 2:
		t.Attr |= VT100AttrDim
		t.Attr &= ^VT100AttrBold
	case 3:
		t.Attr |= VT100AttrItalic
	case 4:
		t.Attr |= VT100AttrUnderline
	case 5:
		t.Attr |= VT100AttrBlink
	case 7:
		t.Attr |= VT100AttrInverse
	case 21, 22:
		t.Attr &= ^(VT100AttrBold | VT100AttrDim)
	case 23:
		t.Attr &= ^VT100AttrItalic
	case 24:
		t.Attr &= ^VT100AttrUnderline
	case 25:
		t.Attr &= ^VT100AttrBlink
	case 27:
		t.Attr &= ^VT100AttrInverse
	case 30, 31, 32, 33, 34, 35, 36, 37:
		t.Attr = (t.Attr & ^Attribute(0xff)) | (Attribute(attr) - 30)
	case 38:
		// Other subcommands, none of which we support:
		// * 2: RGB
		// * 3: CMY
		// * 4: CMYK
		return AttrMode38a
	case 39:
		t.Attr = (t.Attr & ^Attribute(0xff)) | 0x10
	case 40, 41, 42, 43, 44, 45, 46, 47:
		t.Attr = (t.Attr & ^Attribute(0xff00)) | ((Attribute(attr) - 40) << 8)
	case 48:
		return AttrMode48a
	case 49:
		t.Attr = (t.Attr & ^Attribute(0xff00)) | 0x1000
	}
	return AttrModeNorm
}

func (t *Tty) isStateFull() bool {
	return len(t.stateTok) == cap(t.stateTok)
}

func (t *Tty) clearParState() {
	t.stateTok = t.stateTok[0:1]
	t.stateTok[0] = 0
}

func (t *Tty) debug(msg string) {
	if t.Debug {
		fmt.Fprintln(os.Stderr, msg)
	}
}

func (t *Tty) err(b byte) {
	if t.Debug {
		t.debugErr(b)
	}
	t.changeState(VTNorm)
}

func (t *Tty) escState() string {
	buf := bytes.Buffer{}
	for i, c := range t.stateTok {
		if i > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(strconv.FormatUint(uint64(c), 10))
	}
	return buf.String()
}

func (t *Tty) debugErr(b byte) {
	switch t.State {
	case VTNorm:
		fmt.Fprintf(os.Stderr, "Unknown code 0x%02x\n", b)
	case VTEsc:
		fmt.Fprintf(os.Stderr, "Unknown code ESC %c\n", rune(b))
	case VTSquare, VTQues, VTPercent:
		fmt.Fprintf(os.Stderr, "Unknown code ESC %s %s\n",
			t.State.Code(), t.escState())
	default:
		fmt.Fprintln(os.Stderr, "Bad state for VT")
	}
}
