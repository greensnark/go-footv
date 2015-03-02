package tty

const (
	VT100AttrBold Attribute = iota << 16
	VT100AttrDim
	VT100AttrItalic
	VT100AttrUnderline
	VT100AttrBlink
	VT100AttrInverse
)

type AttrMode int

const (
	AttrModeNorm AttrMode = iota
	AttrMode38a
	AttrMode38b
	AttrMode48a
	AttrMode48b
)
