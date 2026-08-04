package parser

import (
	"math/big"
	"strings"
	"unicode"
)

// parseSet recognizes finite set notation such as "{1, 3, 7}" and
// set-builder notation such as "{x | x ∈ [9..11]}". Braces disambiguate it
// from parseInterval, which only handles "[" and "(" delimited ranges.
func parseSet(s string) (*Value, bool) {
	if len(s) < 3 || s[0] != '{' || s[len(s)-1] != '}' {
		return nil, false
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return nil, false
	}

	if v, ok := trySetBuilder(inner); ok {
		return v, true
	}

	return tryFiniteSet(inner)
}

// trySetBuilder handles notation of the form "x | x ∈ [9..11]" or
// "n : n in [1..5]", delegating the actual range parsing to parseInterval.
func trySetBuilder(inner string) (*Value, bool) {
	sepIdx := strings.IndexAny(inner, "|:")
	if sepIdx < 0 {
		return nil, false
	}

	varName := strings.TrimSpace(inner[:sepIdx])
	if len([]rune(varName)) != 1 || !unicode.IsLetter([]rune(varName)[0]) {
		return nil, false
	}

	rest := strings.TrimSpace(inner[sepIdx+1:])

	// Strip variable name prefix (e.g. "x ∈ ..." or "x in ...")
	if strings.HasPrefix(rest, varName) {
		rest = strings.TrimSpace(rest[len(varName):])
	}

	// Strip ∈ or "in"
	if strings.HasPrefix(rest, "∈") {
		rest = strings.TrimSpace(strings.TrimPrefix(rest, "∈"))
	} else if after, ok := strings.CutPrefix(rest, "in "); ok {
		rest = strings.TrimSpace(after)
	} else if after, ok := strings.CutPrefix(rest, "in\t"); ok {
		rest = strings.TrimSpace(after)
	} else {
		return nil, false
	}

	v, ok := parseInterval(rest)
	if !ok {
		return nil, false
	}
	return v, true
}

func tryFiniteSet(inner string) (*Value, bool) {
	parts := SplitBracketAware(inner, ',')
	if len(parts) == 0 {
		return nil, false
	}

	values := make([]*Value, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		v, err := ParseValue(part)
		if err != nil {
			return nil, false
		}
		values = append(values, v)
	}

	var allPoints []*big.Rat
	for _, v := range values {
		if v.Points != nil {
			allPoints = append(allPoints, v.Points...)
		} else if v.IsPoint() {
			allPoints = append(allPoints, v.Min)
		} else {
			allPoints = nil
			break
		}
	}

	if allPoints != nil {
		return DiscreteValue(allPoints), true
	}

	minR, maxR := values[0].Min, values[0].Max
	for _, v := range values[1:] {
		if v.Min.Cmp(minR) < 0 {
			minR = v.Min
		}
		if v.Max.Cmp(maxR) > 0 {
			maxR = v.Max
		}
	}
	return &Value{Min: minR, Max: maxR}, true
}

// SplitBracketAware splits s on sep, ignoring occurrences of sep that are
// nested inside brackets ({}, [], or ()). It is exported for reuse by the
// handler's expression tokenizer.
func SplitBracketAware(s string, sep rune) []string {
	var parts []string
	depth := 0
	start := 0
	for i, ch := range s {
		switch ch {
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			depth--
		case sep:
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}
