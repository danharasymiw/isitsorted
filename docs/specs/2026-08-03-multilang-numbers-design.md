# Multilingual Number Word Parsing

**Date:** 2026-08-03
**Status:** Approved

## Goal

Extend `parseValue` to recognize number words from ~13 languages beyond English, including multi-word phrases (e.g. "drei hundert zwanzig" → 320, "三百二十" → 320).

## Data Model

Each language is described by a `langDef` struct:

```go
type langDef struct {
    name     string
    ones     map[string]int64    // "drei" → 3
    tens     map[string]int64    // "dreißig" → 30
    scales   map[string]*big.Int // "tausend" → 1000
    negative []string            // negation prefixes: ["minus", "negativ"]
    skip     []string            // connector words to ignore: ["und", "et"]
    quirks   func([]string) (*big.Int, bool) // optional pre-engine hook
}
```

All language definitions are registered in a global slice. The engine tries each in order and returns the first clean match.

## File Structure

| File | Contents |
|---|---|
| `parse.go` | `parseValue` entry point, English logic, `tokenize`, `formatRat` — unchanged structure |
| `multilang.go` | `langDef` type, `tryLang`, `parseMultiLang`, language registry |
| `languages_latin.go` | German, French, Spanish, Portuguese, Italian, Dutch, Swedish |
| `languages_cjk.go` | Japanese, Chinese (Simplified), Korean; character-based tokenizer |
| `languages_other.go` | Russian, Arabic, Hindi |
| `multilang_test.go` | All multilingual test cases |

## Shared Engine

`tryLang(lang, tokens) (*big.Int, bool)` runs the same accumulator algorithm as `parseEnglish` today, but driven by `langDef` tables:

1. Strip negative prefix → set neg flag
2. Skip connector words (`lang.skip`)
3. For each token:
   - ones match → add to current group
   - tens match → add to current group
   - scale ≥ thousand → flush current into total, multiply, reset current
   - scale = hundred → multiply current only
   - no match → return `false` (language didn't match; try next)
4. Return `total + current`, negated if flag set

If `lang.quirks` is non-nil, it runs first and can short-circuit the generic engine.

`parseMultiLang(s string) (*big.Int, error)` tokenizes `s` (script-aware, see below), then iterates all registered languages via `tryLang`. Returns the first match, or an error if none match.

**Integration:** In `parseValue`, `parseMultiLang` is called after the existing English fallback. No change to call signatures or error messages.

## Tokenization

Detection is based on Unicode script:

- **CJK input** (any rune in U+3000–U+9FFF): tokenize character-by-character. Each Han/Kana/Hangul character is one token.
- **All other input**: space + hyphen split (current `tokenize` behavior).

**German fused forms** ("einundzwanzig"): before `tryLang`, a pre-processing step checks if a Latin-script token contains "und" and neither half is a scale word. If so, the token is split into [left, "und", right]. "und" is in `lang.skip`, so it is ignored by the engine.

## Language-Specific Quirks

### French
The 70–99 range doesn't follow regular tens:
- **70–79**: "soixante" (60) sits in the tens table; the additive engine naturally combines it with dix/onze/douze/... from the ones table. No special handling needed.
- **80–99**: "quatre-vingts" (4×20) and "quatre-vingt-dix" (4×20+10) are multiplicative — the additive engine can't handle them. The `quirks` hook intercepts any token stream that begins with ["quatre", "vingt"], computes 4×20=80, then lets the engine add any remaining ones tokens.

### German
No engine changes needed beyond the "und"-splitting pre-processor described above. "Eins" (standalone 1) and "ein" (1 in compounds) are both in the `ones` table.

### Japanese & Chinese (ten-thousand base)
These languages use 万 (10,000) as the primary grouping unit rather than 1,000. A config flag `tenThousandBase bool` on `langDef` switches the engine's flush logic:
- Normal: flush on scales ≥ 1,000
- Ten-thousand base: flush on 万 (10,000), 億 (10^8), 兆 (10^12)

Higher scale words (億, 兆) multiply the 万-accumulated subtotal.

### Korean
Two coexisting number systems share one `langDef`:
- **Sino-Korean**: 일(1) 이(2) 삼(3) ... 십(10) 백(100) 천(1,000) 만(10,000)
- **Native Korean**: 하나/한(1) 둘/두(2) 셋/세(3) ... 열(10); tops out at 99

Both systems' words are registered in the same `ones`/`tens` tables. Native Korean forms that have two variants (하나/한, 둘/두, etc.) are both included. If a scale word like 백 (hundred) appears, the Sino-Korean system handles it naturally; native Korean tokens are unambiguous below 100.

### Russian, Arabic, Hindi
Nominative singular forms only — no grammatical agreement for a first pass. This handles the common input pattern (someone typing a number word) without requiring full morphological analysis.

## Error Handling

- `tryLang` returns `(value, bool)` — `false` is "not a match," not an error. Every non-skip token must resolve; partial matches are rejected.
- If no language matches, `parseMultiLang` returns an error. `parseValue` surfaces this as the existing `cannot parse %q as a number` message.
- No change to error UX.

## Testing (`multilang_test.go`)

Table-driven tests covering:

| Category | Examples |
|---|---|
| Simple word per language | "eins"→1, "dos"→2, "삼"→3, "trois"→3, ... |
| Multi-word phrase per language | "drei hundert zwanzig"→320, "trois cent vingt"→320 |
| French quirks | "soixante-dix"→70, "quatre-vingts"→80, "quatre-vingt-dix"→90 |
| German fused forms | "einundzwanzig"→21, "dreiundvierzig"→43 |
| CJK phrases | "三百二十"→320 (Chinese/Japanese), "삼백이십"→320 (Korean) |
| Korean dual system | "하나"→1 and "일"→1 both succeed |
| Negative prefixes | "minus drei"→-3, "moins trois"→-3 |
| Rejection | "drei banana"→error, mixed-language phrase→error |
| Large numbers | "eine Million zweihundert"→1,000,200 |

Existing `parse_test.go` English tests are untouched.

## Languages in Scope (v1)

| Language | Script | Notes |
|---|---|---|
| German | Latin | Fused tens pre-processor |
| French | Latin | Quirks hook for 70–99 |
| Spanish | Latin | Pure table |
| Portuguese | Latin | Pure table |
| Italian | Latin | Pure table |
| Dutch | Latin | Pure table |
| Swedish | Latin | Pure table |
| Russian | Cyrillic | Nominative forms only |
| Japanese | CJK | 万-base, kanji + hiragana romaji |
| Chinese (Simplified) | CJK | 万-base, characters |
| Korean | Hangul | Sino + native systems |
| Arabic | Arabic | Nominative forms only |
| Hindi | Devanagari | Nominative forms only |
