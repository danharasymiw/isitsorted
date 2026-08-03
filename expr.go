package main

import (
	"fmt"
	"math/big"
	"unicode"
)


type exprParser struct {
	runes []rune
	pos   int
}

// parseExprString parses and evaluates an arithmetic expression.
// Supports +, -, *, /, ^ (integer exponents ≤ 100), parentheses,
// unary minus/plus, numeric literals, and named constants (pi, e, etc.).
func parseExprString(s string) (*big.Rat, error) {
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

// parseAddSub = parseMulDiv (('+' | '-') parseMulDiv)*
func (p *exprParser) parseAddSub() (*big.Rat, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return nil, err
	}
	for {
		ch, ok := p.peek()
		if !ok || (ch != '+' && ch != '-') {
			break
		}
		p.pos++
		right, err := p.parseMulDiv()
		if err != nil {
			return nil, err
		}
		res := new(big.Rat)
		if ch == '+' {
			res.Add(left, right)
		} else {
			res.Sub(left, right)
		}
		left = res
	}
	return left, nil
}

// parseMulDiv = parsePow (('*' | '/') parsePow)*
func (p *exprParser) parseMulDiv() (*big.Rat, error) {
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
		res := new(big.Rat)
		if ch == '*' {
			res.Mul(left, right)
		} else {
			if right.Sign() == 0 {
				return nil, fmt.Errorf("division by zero")
			}
			res.Quo(left, right)
		}
		left = res
	}
	return left, nil
}

// parsePow = parseUnary ('^' parseUnary)?  right-associative
func (p *exprParser) parsePow() (*big.Rat, error) {
	base, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	ch, ok := p.peek()
	if !ok || ch != '^' {
		return base, nil
	}
	p.pos++
	exp, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	if !exp.IsInt() {
		return nil, fmt.Errorf("exponent must be an integer, got %s", exp.RatString())
	}
	expNum := exp.Num()
	if !expNum.IsInt64() {
		return nil, fmt.Errorf("exponent must fit in int64")
	}
	return ratPow(base, expNum.Int64())
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
func (p *exprParser) parseUnary() (*big.Rat, error) {
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
		return new(big.Rat).Neg(v), nil
	}
	if ch == '+' {
		p.pos++
		return p.parseUnary()
	}
	return p.parsePrimary()
}

// parsePrimary = '(' parseAddSub ')' | atom
func (p *exprParser) parsePrimary() (*big.Rat, error) {
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
func (p *exprParser) parseAtom() (*big.Rat, error) {
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
		return v, nil
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
		return r, nil
	}

	return nil, fmt.Errorf("unexpected %q", ch)
}
