package vt

import (
	"bytes"
	"fmt"
)

// DebugDump returns a string with a debug dump of the tty content,
// matching the dumps produced in termrec's tests.
func (t *Tty) DebugDump() string {
	out := &bytes.Buffer{}
	fmt.Fprintf(out, ".-===[ %dx%d ]\n", t.Size.X, t.Size.Y)
	attr := Attribute(0x1010)

	for y := 0; y < t.Size.Y; y++ {
		fmt.Fprint(out, "| ")

		baseOffset := y * t.Size.X
		for x := 0; x < t.Size.X; x++ {
			c := t.Buf[baseOffset+x]
			if c.Attr != attr {
				attr = c.Attr
				fmt.Fprintf(out, "{%X}", attr)
			}
			if c.Ch >= ' ' && c.Ch < 127 {
				fmt.Fprintf(out, "%c", c.Ch)
			} else {
				fmt.Fprintf(out, "[%04X]", uint32(c.Ch))
			}
		}
		fmt.Fprintln(out, "")
	}
	fmt.Fprintf(out, "`-===[ cursor at %d,%d]\n", t.Cursor.X, t.Cursor.Y)
	return out.String()
}
