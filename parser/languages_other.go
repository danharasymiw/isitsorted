package parser

import "math/big"

// init registers Russian, Arabic, Hindi, and Abkhaz language definitions.
// Each table covers nominative singular forms only — no grammatical agreement
// (case, gender, number) is attempted for this first pass.
func init() {
	languages = append(languages,
		russianLangDef(),
		arabicLangDef(),
		hindiLangDef(),
		abkhazLangDef(),
	)
}

func russianLangDef() langDef {
	return langDef{
		name: "russian",
		ones: map[string]int64{
			"ноль": 0, "нуль": 0,
			"один": 1, "одна": 1, "одно": 1,
			"два": 2, "две": 2,
			"три":          3,
			"четыре":       4,
			"пять":         5,
			"шесть":        6,
			"семь":         7,
			"восемь":       8,
			"девять":       9,
			"десять":       10,
			"одиннадцать":  11,
			"двенадцать":   12,
			"тринадцать":   13,
			"четырнадцать": 14,
			"пятнадцать":   15,
			"шестнадцать":  16,
			"семнадцать":   17,
			"восемнадцать": 18,
			"девятнадцать": 19,
		},
		tens: map[string]int64{
			"двадцать":    20,
			"тридцать":    30,
			"сорок":       40,
			"пятьдесят":   50,
			"шестьдесят":  60,
			"семьдесят":   70,
			"восемьдесят": 80,
			"девяносто":   90,
			// These compound hundreds behave like pre-multiplied tens: each
			// is a single word, so it is added rather than multiplied.
			"двести":    200,
			"триста":    300,
			"четыреста": 400,
			"пятьсот":   500,
			"шестьсот":  600,
			"семьсот":   700,
			"восемьсот": 800,
			"девятьсот": 900,
		},
		scales: map[string]*big.Int{
			"сто":        big.NewInt(100),
			"тысяча":     big.NewInt(1_000),
			"тысячи":     big.NewInt(1_000),
			"тысяч":      big.NewInt(1_000),
			"миллион":    big.NewInt(1_000_000),
			"миллиона":   big.NewInt(1_000_000),
			"миллионов":  big.NewInt(1_000_000),
			"миллиард":   big.NewInt(1_000_000_000),
			"миллиарда":  big.NewInt(1_000_000_000),
			"миллиардов": big.NewInt(1_000_000_000),
		},
		negative: []string{"минус"},
	}
}

func arabicLangDef() langDef {
	return langDef{
		name: "arabic",
		ones: map[string]int64{
			"صفر":    0,
			"واحد":   1,
			"أحد":    1,
			"اثنان":  2,
			"اثنين":  2,
			"اثنا":   2,
			"ثلاثة":  3,
			"ثلاث":   3,
			"أربعة":  4,
			"أربع":   4,
			"خمسة":   5,
			"خمس":    5,
			"ستة":    6,
			"ست":     6,
			"سبعة":   7,
			"سبع":    7,
			"ثمانية": 8,
			"ثماني":  8,
			"تسعة":   9,
			"تسع":    9,
			"عشرة":   10,
			"عشر":    10,
		},
		tens: map[string]int64{
			"عشرون":  20,
			"عشرين":  20,
			"ثلاثون": 30,
			"ثلاثين": 30,
			"أربعون": 40,
			"أربعين": 40,
			"خمسون":  50,
			"خمسين":  50,
			"ستون":   60,
			"ستين":   60,
			"سبعون":  70,
			"سبعين":  70,
			"ثمانون": 80,
			"ثمانين": 80,
			"تسعون":  90,
			"تسعين":  90,
		},
		scales: map[string]*big.Int{
			"مئة":     big.NewInt(100),
			"مائة":    big.NewInt(100),
			"ألف":     big.NewInt(1_000),
			"آلاف":    big.NewInt(1_000),
			"مليون":   big.NewInt(1_000_000),
			"ملايين":  big.NewInt(1_000_000),
			"مليار":   big.NewInt(1_000_000_000),
			"مليارات": big.NewInt(1_000_000_000),
		},
		negative: []string{"سالب"},
		// "و" is the Arabic conjunction "wa" ("and"), used to connect
		// number groups (e.g. "مئة و خمسة" = "a hundred and five").
		skip: []string{"و"},
	}
}

func abkhazLangDef() langDef {
	// Abkhaz uses a hybrid vigesimal-decimal system. Numbers 1-19 are
	// individual words; 20 (ҩажәа) is the vigesimal base. Hundreds and
	// thousands work multiplicatively via the shared engine.
	return langDef{
		name: "abkhaz",
		ones: map[string]int64{
			"акы":    1,
			"ҩба":    2,
			"хԥа":    3,
			"ԥшьба":  4,
			"хәба":   5,
			"фба":    6,
			"бжьба":  7,
			"ааба":   8,
			"жәба":   9,
			"жәаба":  10,
			"жәеиза": 11,
			"жәаҩа":  12,
			"жәаха":  13,
			"жәиԥшь": 14,
			"жәохә":  15,
			"жәаф":   16,
			"жәибжь": 17,
			"жәаа":   18,
			"зеижә":  19,
		},
		tens: map[string]int64{
			"ҩажәа": 20,
		},
		scales: map[string]*big.Int{
			"шәкы": big.NewInt(100),
			"зықы": big.NewInt(1_000),
		},
	}
}

func hindiLangDef() langDef {
	return langDef{
		name: "hindi",
		ones: map[string]int64{
			"शून्य":  0,
			"एक":     1,
			"दो":     2,
			"तीन":    3,
			"चार":    4,
			"पाँच":   5,
			"पांच":   5,
			"छह":     6,
			"छः":     6,
			"सात":    7,
			"आठ":     8,
			"नौ":     9,
			"दस":     10,
			"ग्यारह": 11,
			"बारह":   12,
			"तेरह":   13,
			"चौदह":   14,
			"पंद्रह": 15,
			"सोलह":   16,
			"सत्रह":  17,
			"अठारह":  18,
			"उन्नीस": 19,
		},
		tens: map[string]int64{
			"बीस":   20,
			"तीस":   30,
			"चालीस": 40,
			"पचास":  50,
			"साठ":   60,
			"सत्तर": 70,
			"अस्सी": 80,
			"नब्बे": 90,
		},
		scales: map[string]*big.Int{
			// Hindi groups by lakh (100,000) and crore (10,000,000) rather
			// than million/billion; both exceed the engine's 1,000
			// flush threshold, so they flush correctly without a
			// dedicated tenThousandBase-style mode.
			"सौ":    big.NewInt(100),
			"हज़ार": big.NewInt(1_000),
			"हजार":  big.NewInt(1_000),
			"लाख":   big.NewInt(100_000),
			"करोड़": big.NewInt(10_000_000),
			"करोड":  big.NewInt(10_000_000),
		},
		negative: []string{"ऋण"},
	}
}
