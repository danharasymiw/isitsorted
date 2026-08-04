package parser

import (
	"fmt"
	"math/big"
	"strings"
)

// parseInterval recognizes interval notation such as "[9..11]", "(9..11)",
// "[9, 11]", and mixed bracket combinations like "[9..11)". Whether a bracket
// is open or closed does not affect the resulting bound values.
func parseInterval(s string) (*Value, bool) {
	if len(s) < 4 {
		return nil, false
	}

	open := s[0]
	close := s[len(s)-1]
	if (open != '[' && open != '(') || (close != ']' && close != ')') {
		return nil, false
	}

	inner := s[1 : len(s)-1]

	var loPart, hiPart string
	if idx := strings.Index(inner, ".."); idx >= 0 {
		loPart = strings.TrimSpace(inner[:idx])
		hiPart = strings.TrimSpace(inner[idx+2:])
	} else if idx := strings.IndexRune(inner, ','); idx >= 0 {
		loPart = strings.TrimSpace(inner[:idx])
		hiPart = strings.TrimSpace(inner[idx+1:])
	} else {
		return nil, false
	}

	lo, err := parseNumberLiteral(loPart)
	if err != nil {
		return nil, false
	}
	hi, err := parseNumberLiteral(hiPart)
	if err != nil {
		return nil, false
	}

	if lo.Cmp(hi) > 0 {
		return nil, false
	}
	return &Value{Min: lo, Max: hi}, true
}

// parseNumberLiteral parses a plain numeric literal (integer, fraction, or
// decimal) as used for interval bounds. It does not handle constants,
// expressions, or other ParseValue extensions.
func parseNumberLiteral(s string) (*big.Rat, error) {
	r := new(big.Rat)
	if _, ok := r.SetString(s); ok {
		return r, nil
	}
	if f, ok := new(big.Float).SetPrec(256).SetString(s); ok {
		rat, _ := f.Rat(nil)
		return rat, nil
	}
	return nil, fmt.Errorf("not a number: %q", s)
}
