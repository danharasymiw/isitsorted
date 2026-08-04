package parser

import "math/big"

// japaneseOnes maps kanji digits 0-9 to their values.
var japaneseOnes = map[string]int64{
	"〇": 0, "一": 1, "二": 2, "三": 3, "四": 4,
	"五": 5, "六": 6, "七": 7, "八": 8, "九": 9,
}

// japaneseScales maps kanji place-value words to their magnitudes. 十/百/千
// are positional multipliers within a ten-thousand group; 万/億/兆 mark the
// ten-thousand-based group boundaries.
var japaneseScales = map[string]*big.Int{
	"十": big.NewInt(10),
	"百": big.NewInt(100),
	"千": big.NewInt(1000),
	"万": big.NewInt(10000),
	"億": big.NewInt(100000000),
	"兆": big.NewInt(1000000000000),
}

// chineseOnes maps simplified Chinese digits 0-9 to their values. 〇 and 零
// both mean zero; 二 and 两 both mean two.
var chineseOnes = map[string]int64{
	"〇": 0, "零": 0,
	"一": 1,
	"二": 2, "两": 2,
	"三": 3, "四": 4, "五": 5, "六": 6, "七": 7, "八": 8, "九": 9,
}

// chineseScales maps simplified Chinese place-value words to their
// magnitudes, mirroring japaneseScales.
var chineseScales = map[string]*big.Int{
	"十": big.NewInt(10),
	"百": big.NewInt(100),
	"千": big.NewInt(1000),
	"万": big.NewInt(10000),
	"亿": big.NewInt(100000000),
	"兆": big.NewInt(1000000000000),
}

// koreanSinoOnes maps Sino-Korean digits 0-9 to their values.
var koreanSinoOnes = map[string]int64{
	"영": 0, "일": 1, "이": 2, "삼": 3, "사": 4,
	"오": 5, "육": 6, "칠": 7, "팔": 8, "구": 9,
}

// koreanSinoScales maps Sino-Korean place-value words to their magnitudes,
// following the same 十/百/千 vs 万/億/兆 split as Japanese and Chinese.
var koreanSinoScales = map[string]*big.Int{
	"십": big.NewInt(10),
	"백": big.NewInt(100),
	"천": big.NewInt(1000),
	"만": big.NewInt(10000),
	"억": big.NewInt(100000000),
	"조": big.NewInt(1000000000000),
}

// koreanNativeOnes maps native Korean number words 1-10 to their values.
// Several values have two interchangeable forms (하나/한, 둘/두, 셋/세,
// 넷/네). Native Korean compounding beyond 10 (e.g. 스물다섯 = 25) is out of
// scope for v1, so 열 (10) is treated as a plain value rather than a scale —
// that also sidesteps tryLang's generic-engine rule that rejects a
// standalone scale word with no preceding digit.
var koreanNativeOnes = map[string]int64{
	"하나": 1, "한": 1,
	"둘": 2, "두": 2,
	"셋": 3, "세": 3,
	"넷": 4, "네": 4,
	"다섯": 5,
	"여섯": 6,
	"일곱": 7,
	"여덟": 8,
	"아홉": 9,
	"열":  10,
}

// koreanNativeCompoundWords lists the native Korean number words that span
// two Hangul syllable blocks. Single-syllable words (둘, 셋, 넷, 열, 한, 두,
// 세, 네) are already whole tokens after character-by-character
// tokenization and need no joining.
var koreanNativeCompoundWords = []string{"하나", "다섯", "여섯", "일곱", "여덟", "아홉"}

// cjkFlushThreshold is the magnitude at which a CJK/Korean scale word
// (万/亿/億/만/兆/조 and up) flushes the current ten-thousand group into the
// running total, rather than multiplying a single positional digit within
// the group (as 十/百/千/십/백/천 do).
var cjkFlushThreshold = big.NewInt(10000)

// parseCJKPositional parses tokens as a positional CJK number, e.g.
// 三百二十 = 3×100 + 2×10 = 320. tryLang's generic engine multiplies its
// entire running "current" value by every sub-thousand scale it meets,
// which is correct for Western languages that use at most one such scale
// per group (e.g. German "dreihundert...") but wrong for CJK numbers, which
// routinely combine several positional scales (千/百/十) within a single
// ten-thousand group — each of those must multiply only the single digit
// that precedes it, not the whole group accumulated so far. This is used as
// a langDef.quirks hook to bypass the generic engine entirely for Japanese,
// Chinese, and Sino-Korean.
func parseCJKPositional(tokens []string, ones map[string]int64, scales map[string]*big.Int) (*big.Int, bool) {
	if len(tokens) == 0 {
		return nil, false
	}

	const noPendingDigit = -1
	pendingDigit := int64(noPendingDigit)
	group := new(big.Int)
	total := new(big.Int)

	commitPendingDigit := func() {
		if pendingDigit != noPendingDigit {
			group.Add(group, big.NewInt(pendingDigit))
			pendingDigit = noPendingDigit
		}
	}

	for _, tok := range tokens {
		if v, ok := ones[tok]; ok {
			if pendingDigit != noPendingDigit {
				return nil, false
			}
			pendingDigit = v
			continue
		}

		scale, ok := scales[tok]
		if !ok {
			return nil, false
		}

		if scale.Cmp(cjkFlushThreshold) >= 0 {
			commitPendingDigit()
			if group.Sign() == 0 {
				group.SetInt64(1)
			}
			group.Mul(group, scale)
			total.Add(total, group)
			group = new(big.Int)
			continue
		}

		digit := pendingDigit
		if digit == noPendingDigit {
			digit = 1
		}
		group.Add(group, new(big.Int).Mul(big.NewInt(digit), scale))
		pendingDigit = noPendingDigit
	}

	commitPendingDigit()
	total.Add(total, group)
	return total, true
}

// joinKoreanNativeWords re-joins character tokens that together spell a
// two-syllable native Korean number word (see koreanNativeCompoundWords).
// This runs as a langDef.preprocess hook: tokenizeMultiLang splits Hangul
// input character-by-character, which is correct for Sino-Korean but would
// otherwise split native words like 하나 into ["하", "나"].
func joinKoreanNativeWords(tokens []string) []string {
	joined := make([]string, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		if i+1 < len(tokens) && containsToken(koreanNativeCompoundWords, tokens[i]+tokens[i+1]) {
			joined = append(joined, tokens[i]+tokens[i+1])
			i++
			continue
		}
		joined = append(joined, tokens[i])
	}
	return joined
}

func init() {
	languages = append(languages,
		langDef{
			name:            "japanese",
			ones:            japaneseOnes,
			scales:          japaneseScales,
			tenThousandBase: true,
			quirks: func(tokens []string) (*big.Int, bool) {
				return parseCJKPositional(tokens, japaneseOnes, japaneseScales)
			},
		},
		langDef{
			name:            "chinese",
			ones:            chineseOnes,
			scales:          chineseScales,
			tenThousandBase: true,
			quirks: func(tokens []string) (*big.Int, bool) {
				return parseCJKPositional(tokens, chineseOnes, chineseScales)
			},
		},
		langDef{
			name:            "korean-sino",
			ones:            koreanSinoOnes,
			scales:          koreanSinoScales,
			tenThousandBase: true,
			quirks: func(tokens []string) (*big.Int, bool) {
				return parseCJKPositional(tokens, koreanSinoOnes, koreanSinoScales)
			},
		},
		langDef{
			name:       "korean-native",
			ones:       koreanNativeOnes,
			preprocess: joinKoreanNativeWords,
		},
	)
}
