package parser

import (
	"fmt"
	"math/big"
	"strings"
	"unicode"
)

type exprParser struct {
	runes []rune
	pos   int
}

// parseExprString parses and evaluates an arithmetic expression.
// Supports +, -, *, /, ^ (integer exponents ≤ 100), parentheses,
// unary minus/plus, numeric literals, named constants (pi, e, etc.),
// and "±" (or "+/-") uncertainty ranges.
func parseExprString(s string) (*Value, error) {
	s = strings.ReplaceAll(s, "+/-", "±")
	p := &exprParser{runes: []rune(s)}
	v, err := p.parseAddSub()
	if err != nil {
		return nil, err
	}
	p.skipWS()
	if p.pos < len(p.runes) {
		return nil, fmt.Errorf("unexpected %q", string(p.runes[p.pos:]))
	}
	return v, nil
}

func (p *exprParser) skipWS() {
	for p.pos < len(p.runes) && unicode.IsSpace(p.runes[p.pos]) {
		p.pos++
	}
}

func (p *exprParser) peek() (rune, bool) {
	p.skipWS()
	if p.pos >= len(p.runes) {
		return 0, false
	}
	return p.runes[p.pos], true
}

// parseAddSub = parseMulDiv (('+' | '-' | '±') parseMulDiv)*
func (p *exprParser) parseAddSub() (*Value, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}
	for {
		ch, ok := p.peek()
		if !ok || (ch != '+' && ch != '-' && ch != '±') {
			break
		}
		p.pos++
		right, err := p.parseMulDiv()
		if err != nil {
			return nil, err
		}
		switch ch {
		case '+':
			left = valAdd(left, right)
		case '-':
			left = valSub(left, right)
		case '±':
			left = valPlusMinus(left, right)
		}
	}
	return left, nil
}

// parseMulDiv = parsePow (('*' | '/') parsePow)*
func (p *exprParser) parseMulDiv() (*Value, error) {
	left, err := p.parsePow()
	if err != nil {
		return nil, err
	}
	for {
		ch, ok := p.peek()
		if !ok || (ch != '*' && ch != '/') {
			break
		}
		p.pos++
		right, err := p.parsePow()
		if err != nil {
			return nil, err
		}
		if ch == '*' {
			left = valMul(left, right)
		} else {
			if right.Min.Sign() <= 0 && right.Max.Sign() >= 0 {
				return nil, fmt.Errorf("division by zero")
			}
			left, err = valDiv(left, right)
			if err != nil {
				return nil, err
			}
		}
	}
	return left, nil
}

// parsePow = parseUnary ('^' parsePow)?  right-associative
func (p *exprParser) parsePow() (*Value, error) {
	base, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	ch, ok := p.peek()
	if !ok || ch != '^' {
		return base, nil
	}
	p.pos++
	exp, err := p.parsePow()
	if err != nil {
		return nil, err
	}
	if !exp.IsPoint() {
		return nil, fmt.Errorf("exponent must be a point value")
	}
	if !exp.Min.IsInt() {
		return nil, fmt.Errorf("exponent must be an integer, got %s", exp.Min.RatString())
	}
	expNum := exp.Min.Num()
	if !expNum.IsInt64() {
		return nil, fmt.Errorf("exponent must fit in int64")
	}
	return valPow(base, expNum.Int64())
}

func ratPow(base *big.Rat, n int64) (*big.Rat, error) {
	if n == 0 {
		return new(big.Rat).SetInt64(1), nil
	}
	neg := n < 0
	if neg {
		n = -n
	}
	result := new(big.Rat).SetInt64(1)
	for i := int64(0); i < n; i++ {
		result.Mul(result, base)
	}
	if neg {
		if result.Sign() == 0 {
			return nil, fmt.Errorf("0^negative is undefined")
		}
		result.Inv(result)
	}
	return result, nil
}

// parseUnary = ('+' | '-') parseUnary | parsePrimary
func (p *exprParser) parseUnary() (*Value, error) {
	ch, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("unexpected end of expression")
	}
	if ch == '-' {
		p.pos++
		v, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return valNeg(v), nil
	}
	if ch == '+' {
		p.pos++
		return p.parseUnary()
	}
	return p.parsePrimary()
}

// parsePrimary = '(' parseAddSub ')' | atom
func (p *exprParser) parsePrimary() (*Value, error) {
	ch, ok := p.peek()
	if !ok {
		return nil, fmt.Errorf("unexpected end of expression")
	}
	if ch == '(' {
		p.pos++
		v, err := p.parseAddSub()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		if p.pos >= len(p.runes) || p.runes[p.pos] != ')' {
			return nil, fmt.Errorf("missing ')'")
		}
		p.pos++
		return v, nil
	}
	return p.parseAtom()
}

// parseAtom reads a numeric literal or named constant.
// Word-numbers (e.g. "forty-two") are not supported inside expressions —
// they must be the entire input and are handled by parseValue's fallback.
func (p *exprParser) parseAtom() (*Value, error) {
	p.skipWS()
	if p.pos >= len(p.runes) {
		return nil, fmt.Errorf("expected a number")
	}
	ch := p.runes[p.pos]

	// Alphabetic → named constant (pi, e, tau, phi, zeta(3), etc.)
	if unicode.IsLetter(ch) {
		start := p.pos
		for p.pos < len(p.runes) {
			r := p.runes[p.pos]
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '(' || r == ')' {
				if r == '(' {
					// consume through closing paren for names like "zeta(3)"
					for p.pos < len(p.runes) && p.runes[p.pos] != ')' {
						p.pos++
					}
					if p.pos < len(p.runes) {
						p.pos++
					}
					break
				}
				p.pos++
			} else {
				break
			}
		}
		name := string(p.runes[start:p.pos])
		v, ok := parseConstant(name)
		if !ok {
			return nil, fmt.Errorf("unknown constant %q", name)
		}
		return PointValue(v), nil
	}

	// Digit or '.' → numeric literal (int, decimal, or rational a/b)
	if unicode.IsDigit(ch) || ch == '.' {
		start := p.pos
		for p.pos < len(p.runes) {
			r := p.runes[p.pos]
			if unicode.IsDigit(r) || r == '.' || r == '/' {
				p.pos++
			} else {
				break
			}
		}
		s := string(p.runes[start:p.pos])
		r := new(big.Rat)
		if _, ok := r.SetString(s); !ok {
			return nil, fmt.Errorf("invalid number %q", s)
		}
		return PointValue(r), nil
	}

	return nil, fmt.Errorf("unexpected %q", ch)
}

const maxDiscretePoints = 1024

func discretePoints(v *Value) []*big.Rat {
	if v.Points != nil {
		return v.Points
	}
	if v.IsPoint() {
		return []*big.Rat{v.Min}
	}
	return nil
}

func valPlusMinus(a, b *Value) *Value {
	ap, bp := discretePoints(a), discretePoints(b)
	if ap != nil && bp != nil {
		var out []*big.Rat
		for _, x := range ap {
			for _, y := range bp {
				out = append(out,
					new(big.Rat).Sub(new(big.Rat).Set(x), y),
					new(big.Rat).Add(new(big.Rat).Set(x), y),
				)
			}
		}
		if len(out) <= maxDiscretePoints {
			return DiscreteValue(out)
		}
	}
	return &Value{
		Min: new(big.Rat).Sub(a.Min, b.Max),
		Max: new(big.Rat).Add(a.Max, b.Max),
	}
}

func valAdd(a, b *Value) *Value {
	ap, bp := discretePoints(a), discretePoints(b)
	if ap != nil && bp != nil {
		var out []*big.Rat
		for _, x := range ap {
			for _, y := range bp {
				out = append(out, new(big.Rat).Add(new(big.Rat).Set(x), y))
			}
		}
		if len(out) <= maxDiscretePoints {
			return DiscreteValue(out)
		}
	}
	return &Value{
		Min: new(big.Rat).Add(a.Min, b.Min),
		Max: new(big.Rat).Add(a.Max, b.Max),
	}
}

func valSub(a, b *Value) *Value {
	ap, bp := discretePoints(a), discretePoints(b)
	if ap != nil && bp != nil {
		var out []*big.Rat
		for _, x := range ap {
			for _, y := range bp {
				out = append(out, new(big.Rat).Sub(new(big.Rat).Set(x), y))
			}
		}
		if len(out) <= maxDiscretePoints {
			return DiscreteValue(out)
		}
	}
	return &Value{
		Min: new(big.Rat).Sub(a.Min, b.Max),
		Max: new(big.Rat).Sub(a.Max, b.Min),
	}
}

func valNeg(a *Value) *Value {
	if a.Points != nil {
		out := make([]*big.Rat, len(a.Points))
		for i, p := range a.Points {
			out[i] = new(big.Rat).Neg(p)
		}
		return DiscreteValue(out)
	}
	return &Value{
		Min: new(big.Rat).Neg(a.Max),
		Max: new(big.Rat).Neg(a.Min),
	}
}

func valMul(a, b *Value) *Value {
	ap, bp := discretePoints(a), discretePoints(b)
	if ap != nil && bp != nil {
		var out []*big.Rat
		for _, x := range ap {
			for _, y := range bp {
				out = append(out, new(big.Rat).Mul(new(big.Rat).Set(x), y))
			}
		}
		if len(out) <= maxDiscretePoints {
			return DiscreteValue(out)
		}
	}
	products := [4]*big.Rat{
		new(big.Rat).Mul(a.Min, b.Min),
		new(big.Rat).Mul(a.Min, b.Max),
		new(big.Rat).Mul(a.Max, b.Min),
		new(big.Rat).Mul(a.Max, b.Max),
	}
	lo, hi := products[0], products[0]
	for _, p := range products[1:] {
		if p.Cmp(lo) < 0 {
			lo = p
		}
		if p.Cmp(hi) > 0 {
			hi = p
		}
	}
	return &Value{Min: lo, Max: hi}
}

func valDiv(a, b *Value) (*Value, error) {
	ap, bp := discretePoints(a), discretePoints(b)
	if ap != nil && bp != nil {
		var out []*big.Rat
		for _, x := range ap {
			for _, y := range bp {
				if y.Sign() == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				out = append(out, new(big.Rat).Quo(new(big.Rat).Set(x), y))
			}
		}
		if len(out) <= maxDiscretePoints {
			return DiscreteValue(out), nil
		}
	}
	if b.Min.Sign() <= 0 && b.Max.Sign() >= 0 {
		return nil, fmt.Errorf("division by interval containing zero")
	}
	inv := &Value{
		Min: new(big.Rat).Inv(new(big.Rat).Set(b.Max)),
		Max: new(big.Rat).Inv(new(big.Rat).Set(b.Min)),
	}
	return valMul(a, inv), nil
}

func valPow(base *Value, n int64) (*Value, error) {
	if n == 0 {
		return PointValue(new(big.Rat).SetInt64(1)), nil
	}
	if base.Points != nil {
		neg := n < 0
		absN := n
		if neg {
			absN = -absN
		}
		var out []*big.Rat
		for _, p := range base.Points {
			r, err := ratPow(p, absN)
			if err != nil {
				return nil, err
			}
			if neg {
				if r.Sign() == 0 {
					return nil, fmt.Errorf("0^negative is undefined")
				}
				r.Inv(r)
			}
			out = append(out, r)
		}
		if len(out) <= maxDiscretePoints {
			return DiscreteValue(out), nil
		}
	}
	neg := n < 0
	if neg {
		n = -n
	}
	loP, err := ratPow(base.Min, n)
	if err != nil {
		return nil, err
	}
	hiP, err := ratPow(base.Max, n)
	if err != nil {
		return nil, err
	}

	var result *Value
	if n%2 == 0 && base.Min.Sign() < 0 && base.Max.Sign() > 0 {
		zero := new(big.Rat)
		hi := loP
		if hiP.Cmp(hi) > 0 {
			hi = hiP
		}
		result = &Value{Min: zero, Max: hi}
	} else {
		lo, hi := loP, hiP
		if lo.Cmp(hi) > 0 {
			lo, hi = hi, lo
		}
		result = &Value{Min: lo, Max: hi}
	}

	if neg {
		if result.Min.Sign() <= 0 && result.Max.Sign() >= 0 {
			return nil, fmt.Errorf("0^negative is undefined")
		}
		result = &Value{
			Min: new(big.Rat).Inv(result.Max),
			Max: new(big.Rat).Inv(result.Min),
		}
	}
	return result, nil
}
