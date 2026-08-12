package main

import (
	"math/big"
	"sorted/parser"
)

// infCmp compares two values where either may be infinite. It returns
// -1, 0, or +1 like big.Rat.Cmp. aInf/bInf are the Inf fields of
// each value (0 for finite); aRat/bRat are used when the corresponding
// inf field is 0.
func infCmp(aInf int8, aRat *big.Rat, bInf int8, bRat *big.Rat) int {
	switch {
	case aInf < 0 && bInf < 0, aInf > 0 && bInf > 0:
		return 0
	case aInf < 0, bInf > 0:
		return -1
	case aInf > 0, bInf < 0:
		return 1
	case aInf == 0 && bInf == 0:
		return aRat.Cmp(bRat)
	default:
		return aRat.Cmp(bRat)
	}
}

// Check reports whether list is sorted according to order ("asc" or "desc").
//
// Continuous ranges (parser.Value with Min/Max) use forall semantics: every
// point in the range must satisfy the ordering constraint. Discrete sets
// (parser.Value with Points) use exists semantics: at least one point must
// fit, and that point becomes the new reference for subsequent comparisons.
//
// Infinities compare as expected: -∞ < every finite value < +∞.
func Check(list []*parser.Value, order string) bool {
	if len(list) <= 1 {
		return true
	}

	asc := order != "desc"

	var prevInf int8
	var prevRat *big.Rat

	first := list[0]
	if first.Inf != 0 {
		prevInf = first.Inf
	} else if first.Points != nil {
		if asc {
			prevRat = first.Points[0]
		} else {
			prevRat = first.Points[len(first.Points)-1]
		}
	} else if asc {
		prevRat = first.Max
	} else {
		prevRat = first.Min
	}

	for i := 1; i < len(list); i++ {
		v := list[i]

		if v.Inf != 0 {
			cmp := infCmp(v.Inf, nil, prevInf, prevRat)
			if asc && cmp < 0 {
				return false
			}
			if !asc && cmp > 0 {
				return false
			}
			prevInf = v.Inf
			prevRat = nil
			continue
		}

		if v.Points != nil {
			if asc {
				found := false
				for _, p := range v.Points {
					if infCmp(0, p, prevInf, prevRat) >= 0 {
						prevInf = 0
						prevRat = p
						found = true
						break
					}
				}
				if !found {
					return false
				}
			} else {
				found := false
				for j := len(v.Points) - 1; j >= 0; j-- {
					if infCmp(0, v.Points[j], prevInf, prevRat) <= 0 {
						prevInf = 0
						prevRat = v.Points[j]
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
		} else {
			if asc {
				if infCmp(0, v.Min, prevInf, prevRat) < 0 {
					return false
				}
				prevInf = 0
				prevRat = v.Max
			} else {
				if infCmp(0, v.Max, prevInf, prevRat) > 0 {
					return false
				}
				prevInf = 0
				prevRat = v.Min
			}
		}
	}
	return true
}
