package main

import (
	"fmt"
	"math/big"
	"strings"
)

var ones = map[string]int64{
	"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4,
	"five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9,
	"ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13,
	"fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17,
	"eighteen": 18, "nineteen": 19,
}

var tens = map[string]int64{
	"twenty": 20, "thirty": 30, "forty": 40, "fifty": 50,
	"sixty": 60, "seventy": 70, "eighty": 80, "ninety": 90,
}

// scales maps scale words to their multiplier. Ordered largest-first for
// the parsing algorithm but stored in a flat map for O(1) lookup.
var scales = map[string]*big.Int{
	"hundred":      big.NewInt(100),
	"thousand":     big.NewInt(1_000),
	"million":      big.NewInt(1_000_000),
	"billion":      big.NewInt(1_000_000_000),
	"trillion":     new(big.Int).SetUint64(1_000_000_000_000),
	"quadrillion":  new(big.Int).SetUint64(1_000_000_000_000_000),
	"quintillion":  new(big.Int).SetUint64(1_000_000_000_000_000_000),
}

// scaleThreshold is the boundary between "hundred" (which only multiplies
// the current group) and the higher scales (which flush the group into
// the running total).
var scaleThreshold = big.NewInt(1000)

// constants maps math symbol names (and their Unicode glyphs) to
// high-precision rational approximations.
var constants = map[string]*big.Rat{
	"pi": rat20("3.14159265358979323846"),
	"π":  rat20("3.14159265358979323846"),

	"e": rat20("2.71828182845904523536"),

	"tau": rat20("6.28318530717958647692"),
	"τ":   rat20("6.28318530717958647692"),

	"phi": rat20("1.61803398874989484820"),
	"φ":   rat20("1.61803398874989484820"),

	"zeta(3)": rat20("1.20205690315959428540"),
	"ζ(3)":    rat20("1.20205690315959428540"),
}

// rat20 is a helper that builds a *big.Rat from a decimal string at init time.
func rat20(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic("bad constant: " + s)
	}
	return r
}

// parseValue converts a string to *big.Rat. It accepts plain integers,
// decimal floats, rationals (e.g. "3/4"), math constants (pi, e, tau, phi),
// and English number words (e.g. "forty-two", "three hundred thousand").
func parseValue(s string) (*big.Rat, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty value")
	}

	// Fast path: plain number (int, decimal, rational like 1/3).
	r := new(big.Rat)
	if _, ok := r.SetString(s); ok {
		return r, nil
	}

	// Prefixed integer literals: 0b (binary), 0x (hex), 0o (octal).
	if n, ok := new(big.Int).SetString(s, 0); ok {
		return new(big.Rat).SetInt(n), nil
	}

	// Scientific notation (e.g. 2.5e-3, 1E10).
	if f, ok := new(big.Float).SetPrec(256).SetString(s); ok {
		rat, _ := f.Rat(nil)
		return rat, nil
	}

	// Math constants with optional negative/minus prefix ("negative pi", "-pi").
	if v, ok := parseConstant(s); ok {
		return v, nil
	}

	// Arithmetic expression (handles operators, parens, constants).
	if v, err := parseExprString(s); err == nil {
		return v, nil
	}

	// Try English number words.
	n, err := parseEnglish(s)
	if err == nil {
		return new(big.Rat).SetInt(n), nil
	}

	// Try other languages.
	if n, err := parseMultiLang(s); err == nil {
		return new(big.Rat).SetInt(n), nil
	}

	return nil, fmt.Errorf("cannot parse %q as a number", s)
}

// parseConstant checks whether s is a math constant name, optionally
// preceded by "-", "negative", or "minus".
func parseConstant(s string) (*big.Rat, bool) {
	low := strings.ToLower(strings.TrimSpace(s))
	neg := false

	if strings.HasPrefix(low, "-") {
		neg = true
		low = strings.TrimSpace(low[1:])
	} else if after, ok := strings.CutPrefix(low, "negative "); ok {
		neg = true
		low = strings.TrimSpace(after)
	} else if after, ok := strings.CutPrefix(low, "minus "); ok {
		neg = true
		low = strings.TrimSpace(after)
	}

	c, ok := constants[low]
	if !ok {
		return nil, false
	}
	v := new(big.Rat).Set(c)
	if neg {
		v.Neg(v)
	}
	return v, true
}

// parseEnglish parses an English number phrase like "negative three hundred
// forty-two thousand five hundred sixty-seven" into a *big.Int.
func parseEnglish(s string) (*big.Int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return nil, fmt.Errorf("empty input")
	}

	// Tokenise on spaces and hyphens.
	tokens := tokenize(s)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no tokens in %q", s)
	}

	neg := false
	if tokens[0] == "negative" || tokens[0] == "minus" {
		neg = true
		tokens = tokens[1:]
		if len(tokens) == 0 {
			return nil, fmt.Errorf("expected number after negative/minus")
		}
	}

	// Special-case bare "zero".
	if len(tokens) == 1 && tokens[0] == "zero" {
		return big.NewInt(0), nil
	}

	result := new(big.Int)  // running total
	current := new(big.Int) // current group accumulator
	gotDigit := false

	for _, tok := range tokens {
		if tok == "and" {
			continue
		}
		if v, ok := ones[tok]; ok {
			current.Add(current, big.NewInt(v))
			gotDigit = true
		} else if v, ok := tens[tok]; ok {
			current.Add(current, big.NewInt(v))
			gotDigit = true
		} else if scale, ok := scales[tok]; ok {
			if !gotDigit && scale.Cmp(scaleThreshold) < 0 {
				return nil, fmt.Errorf("unexpected %q without preceding digit", tok)
			}
			if scale.Cmp(scaleThreshold) < 0 {
				// "hundred" — multiply the current group only.
				current.Mul(current, scale)
			} else {
				// thousand+ — flush into result.
				if current.Sign() == 0 {
					current.SetInt64(1) // "thousand" alone means 1000
				}
				current.Mul(current, scale)
				result.Add(result, current)
				current.SetInt64(0)
			}
			gotDigit = true
		} else {
			return nil, fmt.Errorf("unrecognised word %q", tok)
		}
	}

	result.Add(result, current)

	if !gotDigit {
		return nil, fmt.Errorf("no numeric words found")
	}
	if neg {
		result.Neg(result)
	}
	return result, nil
}

func tokenize(s string) []string {
	var tokens []string
	for _, part := range strings.Fields(s) {
		for _, sub := range strings.Split(part, "-") {
			sub = strings.TrimSpace(sub)
			if sub != "" {
				tokens = append(tokens, sub)
			}
		}
	}
	return tokens
}

// formatRat returns a clean display string for a *big.Rat.
func formatRat(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String()
	}
	s := r.FloatString(10)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
