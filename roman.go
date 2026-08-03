package main

import (
	"math/big"
	"strings"
)

var romanValues = map[rune]int64{
	'I': 1,
	'V': 5,
	'X': 10,
	'L': 50,
	'C': 100,
	'D': 500,
	'M': 1000,
}

func parseRoman(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}

	if strings.EqualFold(s, "nulla") {
		return new(big.Rat), true
	}

	s = strings.ToUpper(s)
	runes := []rune(s)

	var values []int64
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Check for combining overline (U+0305) on the next rune
		multiplier := int64(1)
		if i+1 < len(runes) && runes[i+1] == '̅' {
			multiplier = 1000
			i++
		}

		base, ok := romanValues[r]
		if !ok {
			return nil, false
		}
		values = append(values, base*multiplier)
	}

	if len(values) == 0 {
		return nil, false
	}

	total := int64(0)
	for i, v := range values {
		if i+1 < len(values) && v < values[i+1] {
			total -= v
		} else {
			total += v
		}
	}

	return new(big.Rat).SetInt64(total), true
}
