package parser

import (
	"math/big"
	"strings"
)

var brailleDigits = map[rune]byte{
	'⠁': '1', // U+2801
	'⠃': '2', // U+2803
	'⠉': '3', // U+2809
	'⠙': '4', // U+2819
	'⠑': '5', // U+2811
	'⠋': '6', // U+280B
	'⠛': '7', // U+281B
	'⠓': '8', // U+2813
	'⠊': '9', // U+280A
	'⠚': '0', // U+281A
}

const (
	brailleNumIndicator = '⠼' // U+283C
	brailleHyphen       = '⠤' // U+2824
	brailleDecimal      = '⠲' // U+2832
)

func hasBraille(s string) bool {
	for _, r := range s {
		if r >= 0x2800 && r <= 0x283F {
			return true
		}
	}
	return false
}

func parseBraille(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" || !hasBraille(s) {
		return nil, false
	}

	runes := []rune(s)
	pos := 0

	neg := false
	if runes[pos] == brailleHyphen {
		neg = true
		pos++
	}

	if pos < len(runes) && runes[pos] == brailleNumIndicator {
		pos++
	}

	if pos >= len(runes) {
		return nil, false
	}

	var b strings.Builder
	hasDecimal := false
	gotDigit := false

	for pos < len(runes) {
		r := runes[pos]
		if d, ok := brailleDigits[r]; ok {
			b.WriteByte(d)
			gotDigit = true
		} else if r == brailleDecimal {
			if hasDecimal {
				return nil, false
			}
			hasDecimal = true
			b.WriteByte('.')
		} else {
			return nil, false
		}
		pos++
	}

	if !gotDigit {
		return nil, false
	}

	numStr := b.String()
	if neg {
		numStr = "-" + numStr
	}

	r := new(big.Rat)
	if _, ok := r.SetString(numStr); ok {
		return r, true
	}

	f, ok := new(big.Float).SetPrec(256).SetString(numStr)
	if !ok {
		return nil, false
	}
	rat, _ := f.Rat(nil)
	return rat, true
}
