package parser

import (
	"fmt"
	"math/big"
	"strings"
)

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

func rat20(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic("bad constant: " + s)
	}
	return r
}

func normalizeEmoji(s string) string {
	hasEmoji := false
	for _, r := range s {
		if r == 0xFE0F || r == 0x20E3 || r == 0x1F51F || r == 0x1F522 {
			hasEmoji = true
			break
		}
	}
	if !hasEmoji {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch r {
		case 0xFE0F, 0x20E3:
		case 0x1F522:
			b.WriteString("1234")
		case 0x1F51F:
			b.WriteString("10")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Value represents a parsed value, which may be an exact point or a range
// (e.g. from "±" uncertainty notation).
type Value struct {
	Min *big.Rat
	Max *big.Rat
}

// PointValue wraps a single *big.Rat as an exact (zero-width) Value.
func PointValue(r *big.Rat) *Value {
	return &Value{Min: r, Max: r}
}

// IsPoint reports whether the value is an exact point (Min == Max).
func (v *Value) IsPoint() bool {
	return v.Min.Cmp(v.Max) == 0
}

func ParseValue(s string) (*Value, error) {
	s = strings.TrimSpace(s)
	s = normalizeEmoji(s)
	if s == "" {
		return nil, fmt.Errorf("empty value")
	}

	r := new(big.Rat)
	if _, ok := r.SetString(s); ok {
		return PointValue(r), nil
	}

	if n, ok := new(big.Int).SetString(s, 0); ok {
		return PointValue(new(big.Rat).SetInt(n)), nil
	}

	if f, ok := new(big.Float).SetPrec(256).SetString(s); ok {
		rat, _ := f.Rat(nil)
		return PointValue(rat), nil
	}

	if v, ok := parseConstant(s); ok {
		return PointValue(v), nil
	}

	if v, ok := parseRoman(s); ok {
		return PointValue(v), nil
	}

	if v, ok := parseBraille(s); ok {
		return PointValue(v), nil
	}

	if v, err := parseExprString(s); err == nil {
		return v, nil
	}

	if n, err := parseMultiLang(s); err == nil {
		return PointValue(new(big.Rat).SetInt(n)), nil
	}

	return nil, fmt.Errorf("cannot parse %q as a number", s)
}

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

func FormatRat(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String()
	}
	s := r.FloatString(10)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// FormatValue renders a Value for display: a plain number for point values,
// "center±delta" for symmetric ranges, or "[min..max]" for asymmetric ranges.
func FormatValue(v *Value) string {
	if v.IsPoint() {
		return FormatRat(v.Min)
	}
	sum := new(big.Rat).Add(v.Min, v.Max)
	center := new(big.Rat).Mul(sum, new(big.Rat).SetFrac64(1, 2))
	delta := new(big.Rat).Sub(v.Max, center)
	if delta.Sign() > 0 {
		return FormatRat(center) + "±" + FormatRat(delta)
	}
	return "[" + FormatRat(v.Min) + ".." + FormatRat(v.Max) + "]"
}
