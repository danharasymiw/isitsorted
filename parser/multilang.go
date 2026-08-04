package parser

import (
	"fmt"
	"math/big"
	"strings"
)

// langDef describes the word-to-value tables and quirks needed to parse
// number words in a single language. The shared engine in tryLang drives
// off these tables using the same additive/multiplicative algorithm as
// parseEnglish.
type langDef struct {
	name            string
	ones            map[string]int64
	tens            map[string]int64
	scales          map[string]*big.Int
	negative        []string
	skip            []string
	tenThousandBase bool
	preprocess      func([]string) []string
	quirks          func([]string) (*big.Int, bool)
}

// languages is the global registry of language definitions, populated by
// init() functions in languages_latin.go, languages_cjk.go, and
// languages_other.go.
var languages []langDef

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

// cjkRuneInRange reports whether r falls in a CJK or Hangul script range,
// which signals that the input should be tokenized character-by-character
// rather than split on whitespace/hyphens.
func cjkRuneInRange(r rune) bool {
	return (r >= 0x3000 && r <= 0x9FFF) || (r >= 0xAC00 && r <= 0xD7AF)
}

// tokenizeMultiLang tokenizes s for multilingual number parsing. CJK/Hangul
// input is split character-by-character since those scripts don't use
// whitespace between number words; other scripts use the same space+hyphen
// splitting as the English tokenizer.
func tokenizeMultiLang(s string) []string {
	for _, r := range s {
		if cjkRuneInRange(r) {
			tokens := make([]string, 0, len(s))
			for _, ch := range s {
				if strings.TrimSpace(string(ch)) != "" {
					tokens = append(tokens, string(ch))
				}
			}
			return tokens
		}
	}
	return tokenize(s)
}

// tryLang attempts to parse tokens as a number phrase in lang. It returns
// (value, true) on a clean match, or (nil, false) if any token cannot be
// resolved, mirroring parseEnglish's algorithm but driven by lang's tables.
func tryLang(lang langDef, tokens []string) (*big.Int, bool) {
	if lang.quirks != nil {
		if v, ok := lang.quirks(tokens); ok {
			return v, true
		}
	}

	if lang.preprocess != nil {
		tokens = lang.preprocess(tokens)
	}

	if len(tokens) == 0 {
		return nil, false
	}

	neg := false
	for _, negWord := range lang.negative {
		if tokens[0] == negWord {
			neg = true
			tokens = tokens[1:]
			break
		}
	}
	if len(tokens) == 0 {
		return nil, false
	}

	scaleThreshold := big.NewInt(1000)
	if lang.tenThousandBase {
		scaleThreshold = big.NewInt(10000)
	}

	result := new(big.Int)
	current := new(big.Int)
	gotDigit := false

	for _, tok := range tokens {
		if containsToken(lang.skip, tok) {
			continue
		}
		if v, ok := lang.ones[tok]; ok {
			current.Add(current, big.NewInt(v))
			gotDigit = true
		} else if v, ok := lang.tens[tok]; ok {
			current.Add(current, big.NewInt(v))
			gotDigit = true
		} else if scale, ok := lang.scales[tok]; ok {
			if !gotDigit && scale.Cmp(scaleThreshold) < 0 {
				return nil, false
			}
			if scale.Cmp(scaleThreshold) < 0 {
				current.Mul(current, scale)
			} else {
				if current.Sign() == 0 {
					current.SetInt64(1)
				}
				current.Mul(current, scale)
				result.Add(result, current)
				current.SetInt64(0)
			}
			gotDigit = true
		} else {
			return nil, false
		}
	}

	result.Add(result, current)

	if !gotDigit {
		return nil, false
	}
	if neg {
		result.Neg(result)
	}
	return result, true
}

// containsToken reports whether tok appears in words.
func containsToken(words []string, tok string) bool {
	for _, w := range words {
		if w == tok {
			return true
		}
	}
	return false
}

// parseMultiLang tokenizes s in a script-aware manner and tries each
// registered language in turn, returning the first clean match.
func parseMultiLang(s string) (*big.Int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return nil, fmt.Errorf("empty input")
	}

	tokens := tokenizeMultiLang(s)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("no tokens in %q", s)
	}

	for _, lang := range languages {
		if v, ok := tryLang(lang, tokens); ok {
			return v, nil
		}
	}

	return nil, fmt.Errorf("cannot parse %q as a number in any known language", s)
}
