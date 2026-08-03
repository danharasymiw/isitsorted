package main

import "math/big"

// splitFusedCompound splits tokens like German "einundzwanzig" or Dutch
// "eenentwintig" into [left, infix, right] whenever infix appears strictly
// inside the token (not at the start or end). Tokens without an internal
// match pass through unchanged, as do tokens that isWholeWord already
// recognizes outright — this guards scale/tens words that happen to contain
// infix as a substring, such as German "hundert" (contains "und") or Dutch
// "duizend"/"zeventig"/"negentig" (contain "en").
//
// The split point is the LAST occurrence of infix, not the first. This
// matters for Dutch: "een" (one) itself contains "en", so a first-occurrence
// search on "eenentwintig" would wrongly split after just "e" instead of
// after "een". Compounds only fuse a single tens/ones pair, so the final
// "en"/"und" in the token is always the real connector.
func splitFusedCompound(tokens []string, infix string, isWholeWord func(string) bool) []string {
	result := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if isWholeWord(tok) {
			result = append(result, tok)
			continue
		}
		idx := lastIndex(tok, infix)
		if idx > 0 && idx+len(infix) < len(tok) {
			left := tok[:idx]
			right := tok[idx+len(infix):]
			if isWholeWord(left) && isWholeWord(right) {
				result = append(result, left, infix, right)
				continue
			}
		}
		result = append(result, tok)
	}
	return result
}

// wholeWordChecker returns a predicate reporting whether tok is already a
// recognized ones, tens, or scale word in the given tables.
func wholeWordChecker(ones map[string]int64, tens map[string]int64, scales map[string]*big.Int) func(string) bool {
	return func(tok string) bool {
		if _, ok := ones[tok]; ok {
			return true
		}
		if _, ok := tens[tok]; ok {
			return true
		}
		if _, ok := scales[tok]; ok {
			return true
		}
		return false
	}
}

// lastIndex returns the index of the last occurrence of substr in s, or -1
// if substr is not present.
func lastIndex(s, substr string) int {
	found := -1
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			found = i
		}
	}
	return found
}

// frenchQuatreVingtQuirk builds the quirks hook that intercepts French
// "quatre-vingt(s)" (80) and "quatre-vingt-dix" (90) style phrases, which
// are multiplicative and can't be handled by the additive/multiplicative
// engine in tryLang. It only fires when the pattern starts at token zero;
// any prefix is left for the generic engine to reject.
func frenchQuatreVingtQuirk(ones map[string]int64) func([]string) (*big.Int, bool) {
	return func(tokens []string) (*big.Int, bool) {
		if len(tokens) < 2 || tokens[0] != "quatre" || (tokens[1] != "vingt" && tokens[1] != "vingts") {
			return nil, false
		}

		total := int64(80)
		for _, tok := range tokens[2:] {
			if tok == "et" {
				continue
			}
			v, ok := ones[tok]
			if !ok {
				return nil, false
			}
			total += v
		}
		return big.NewInt(total), true
	}
}

func init() {
	germanOnes := map[string]int64{
		// "eine" is the feminine form of "ein", required before
		// feminine scale nouns like "Million"/"Milliarde" (e.g.
		// "eine million").
		"null": 0, "eins": 1, "ein": 1, "eine": 1, "zwei": 2, "drei": 3, "vier": 4,
		"fünf": 5, "sechs": 6, "sieben": 7, "acht": 8, "neun": 9,
		"zehn": 10, "elf": 11, "zwölf": 12, "dreizehn": 13, "vierzehn": 14,
		"fünfzehn": 15, "sechzehn": 16, "siebzehn": 17, "achtzehn": 18, "neunzehn": 19,
	}
	germanTens := map[string]int64{
		"zwanzig": 20, "dreißig": 30, "dreissig": 30, "vierzig": 40,
		"fünfzig": 50, "sechzig": 60, "siebzig": 70, "achtzig": 80, "neunzig": 90,
		"zweihundert": 200, "dreihundert": 300, "vierhundert": 400,
		"fünfhundert": 500, "sechshundert": 600, "siebenhundert": 700,
		"achthundert": 800, "neunhundert": 900,
	}
	germanScales := map[string]*big.Int{
		"hundert":   big.NewInt(100),
		"tausend":   big.NewInt(1000),
		"million":   big.NewInt(1000000),
		"milliarde": big.NewInt(1000000000),
		"billion":   big.NewInt(1000000000000),
		"billiarde": new(big.Int).Exp(big.NewInt(10), big.NewInt(15), nil),
	}
	germanIsWholeWord := wholeWordChecker(germanOnes, germanTens, germanScales)
	languages = append(languages, langDef{
		name:     "german",
		ones:     germanOnes,
		tens:     germanTens,
		scales:   germanScales,
		negative: []string{"minus", "negativ"},
		skip:     []string{"und"},
		preprocess: func(tokens []string) []string {
			return splitFusedCompound(tokens, "und", germanIsWholeWord)
		},
	})

	frenchOnes := map[string]int64{
		"zéro": 0, "zero": 0, "un": 1, "une": 1, "deux": 2, "trois": 3, "quatre": 4,
		"cinq": 5, "six": 6, "sept": 7, "huit": 8, "neuf": 9, "dix": 10,
		"onze": 11, "douze": 12, "treize": 13, "quatorze": 14, "quinze": 15, "seize": 16,
	}
	languages = append(languages, langDef{
		name: "french",
		ones: frenchOnes,
		tens: map[string]int64{
			"vingt": 20, "trente": 30, "quarante": 40, "cinquante": 50, "soixante": 60,
		},
		scales: map[string]*big.Int{
			"cent":     big.NewInt(100),
			"mille":    big.NewInt(1000),
			"million":  big.NewInt(1000000),
			"milliard": big.NewInt(1000000000),
		},
		negative: []string{"moins"},
		skip:     []string{"et"},
		quirks:   frenchQuatreVingtQuirk(frenchOnes),
	})

	languages = append(languages, langDef{
		name: "spanish",
		ones: map[string]int64{
			"cero": 0, "uno": 1, "una": 1, "dos": 2, "tres": 3, "cuatro": 4,
			"cinco": 5, "seis": 6, "siete": 7, "ocho": 8, "nueve": 9,
			"diez": 10, "once": 11, "doce": 12, "trece": 13, "catorce": 14, "quince": 15,
			"dieciséis": 16, "dieciseis": 16, "diecisiete": 17, "dieciocho": 18, "diecinueve": 19,
			"veintiuno": 21, "veintiún": 21, "veintiun": 21, "veintidós": 22, "veintidos": 22,
			"veintitrés": 23, "veintitres": 23, "veinticuatro": 24, "veinticinco": 25,
			"veintiséis": 26, "veintiseis": 26, "veintisiete": 27, "veintiocho": 28, "veintinueve": 29,
			"cien": 100,
		},
		tens: map[string]int64{
			"veinte": 20, "treinta": 30, "cuarenta": 40, "cincuenta": 50,
			"sesenta": 60, "setenta": 70, "ochenta": 80, "noventa": 90,
		},
		scales: map[string]*big.Int{
			"cien":   big.NewInt(100),
			"ciento": big.NewInt(100),
			"mil":    big.NewInt(1000),
			"millón": big.NewInt(1000000),
			"millon": big.NewInt(1000000),
		},
		negative: []string{"menos"},
		skip:     []string{"y"},
	})

	languages = append(languages, langDef{
		name: "portuguese",
		ones: map[string]int64{
			"zero": 0, "um": 1, "uma": 1, "dois": 2, "duas": 2, "três": 3, "tres": 3,
			"quatro": 4, "cinco": 5, "seis": 6, "sete": 7, "oito": 8, "nove": 9,
			"dez": 10, "onze": 11, "doze": 12, "treze": 13, "catorze": 14, "quatorze": 14,
			"quinze": 15, "dezesseis": 16, "dezasseis": 16, "dezessete": 17, "dezassete": 17,
			"dezoito": 18, "dezenove": 19, "dezanove": 19,
		},
		tens: map[string]int64{
			"vinte": 20, "trinta": 30, "quarenta": 40, "cinquenta": 50, "cinqüenta": 50,
			"sessenta": 60, "setenta": 70, "oitenta": 80, "noventa": 90,
		},
		scales: map[string]*big.Int{
			"cem":    big.NewInt(100),
			"cento":  big.NewInt(100),
			"mil":    big.NewInt(1000),
			"milhão": big.NewInt(1000000),
			"milhao": big.NewInt(1000000),
			"bilhão": big.NewInt(1000000000),
			"bilhao": big.NewInt(1000000000),
		},
		negative: []string{"menos"},
		skip:     []string{"e"},
	})

	languages = append(languages, langDef{
		name: "italian",
		ones: map[string]int64{
			"zero": 0, "uno": 1, "una": 1, "due": 2, "tre": 3, "quattro": 4,
			"cinque": 5, "sei": 6, "sette": 7, "otto": 8, "nove": 9,
			"dieci": 10, "undici": 11, "dodici": 12, "tredici": 13, "quattordici": 14,
			"quindici": 15, "sedici": 16, "diciassette": 17, "diciotto": 18, "diciannove": 19,
		},
		tens: map[string]int64{
			"venti": 20, "trenta": 30, "quaranta": 40, "cinquanta": 50,
			"sessanta": 60, "settanta": 70, "ottanta": 80, "novanta": 90,
		},
		scales: map[string]*big.Int{
			"cento":    big.NewInt(100),
			"mila":     big.NewInt(1000),
			"mille":    big.NewInt(1000),
			"milione":  big.NewInt(1000000),
			"miliardo": big.NewInt(1000000000),
		},
		negative: []string{"meno"},
	})

	dutchOnes := map[string]int64{
		"nul": 0, "een": 1, "één": 1, "twee": 2, "drie": 3, "vier": 4,
		"vijf": 5, "zes": 6, "zeven": 7, "acht": 8, "negen": 9,
		"tien": 10, "elf": 11, "twaalf": 12, "dertien": 13, "veertien": 14,
		"vijftien": 15, "zestien": 16, "zeventien": 17, "achttien": 18, "negentien": 19,
	}
	dutchTens := map[string]int64{
		"twintig": 20, "dertig": 30, "veertig": 40, "vijftig": 50,
		"zestig": 60, "zeventig": 70, "tachtig": 80, "negentig": 90,
	}
	dutchScales := map[string]*big.Int{
		"honderd": big.NewInt(100),
		"duizend": big.NewInt(1000),
		"miljoen": big.NewInt(1000000),
		"miljard": big.NewInt(1000000000),
	}
	dutchIsWholeWord := wholeWordChecker(dutchOnes, dutchTens, dutchScales)
	languages = append(languages, langDef{
		name:     "dutch",
		ones:     dutchOnes,
		tens:     dutchTens,
		scales:   dutchScales,
		negative: []string{"min"},
		skip:     []string{"en"},
		preprocess: func(tokens []string) []string {
			return splitFusedCompound(tokens, "en", dutchIsWholeWord)
		},
	})

	languages = append(languages, langDef{
		name: "swedish",
		ones: map[string]int64{
			"noll": 0, "en": 1, "ett": 1, "två": 2, "tre": 3, "fyra": 4,
			"fem": 5, "sex": 6, "sju": 7, "åtta": 8, "nio": 9,
			"tio": 10, "elva": 11, "tolv": 12, "tretton": 13, "fjorton": 14,
			"femton": 15, "sexton": 16, "sjutton": 17, "arton": 18, "nitton": 19,
		},
		tens: map[string]int64{
			"tjugo": 20, "trettio": 30, "fyrtio": 40, "fyrti": 40,
			"femtio": 50, "femti": 50, "sextio": 60, "sexti": 60,
			"sjuttio": 70, "sjutti": 70, "åttio": 80, "åtti": 80,
			"nittio": 90, "nitti": 90,
		},
		scales: map[string]*big.Int{
			"hundra":  big.NewInt(100),
			"tusen":   big.NewInt(1000),
			"miljon":  big.NewInt(1000000),
			"miljard": big.NewInt(1000000000),
		},
		negative: []string{"minus"},
	})
}
