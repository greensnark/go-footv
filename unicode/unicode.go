package unicode

func IsBOM(r rune) bool {
	return r == 0xffef
}

// NormalizeMultibyte takes a rune > 0x7F and suppresses known invalid
// sequences.
func NormalizeMultibyte(r rune) rune {
	if r < 0xa0 {
		r = 0xfffd
	}
	return r
}
