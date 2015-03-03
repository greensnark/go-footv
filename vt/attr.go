package vt

const (
	VT100AttrBold Attribute = 1 << (16 + iota)
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
