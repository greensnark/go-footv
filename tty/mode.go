package tty

type VTMode int

const (
	VTNorm VTMode = iota
	VTEsc
	VTGetPars
	VTSquare
	VTQues
	VTSetG0
	VTSetG1
	VTPercent
	VTOsc
)

func (v VTMode) Code() string {
	switch v {
	case VTSquare:
		return "["
	case VTQues:
		return "[?"
	case VTPercent:
		return "%"
	default:
		return ""
	}
}
