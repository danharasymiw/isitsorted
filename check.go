package main

import (
	"math/big"
	"sorted/parser"
)

func check(list []*parser.Value, order string) bool {
	if len(list) <= 1 {
		return true
	}

	asc := order != "desc"

	var prev *big.Rat
	first := list[0]
	if first.Points != nil {
		if asc {
			prev = first.Points[0]
		} else {
			prev = first.Points[len(first.Points)-1]
		}
	} else if asc {
		prev = first.Max
	} else {
		prev = first.Min
	}

	for i := 1; i < len(list); i++ {
		v := list[i]
		if v.Points != nil {
			if asc {
				found := false
				for _, p := range v.Points {
					if p.Cmp(prev) >= 0 {
						prev = p
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
					if v.Points[j].Cmp(prev) <= 0 {
						prev = v.Points[j]
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
				if v.Min.Cmp(prev) < 0 {
					return false
				}
				prev = v.Max
			} else {
				if v.Max.Cmp(prev) > 0 {
					return false
				}
				prev = v.Min
			}
		}
	}
	return true
}
