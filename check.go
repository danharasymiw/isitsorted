package main

import "math/big"

func check(list []*big.Rat, order string) bool {
	for i := 1; i < len(list); i++ {
		cmp := list[i].Cmp(list[i-1])
		if order == "desc" {
			if cmp > 0 {
				return false
			}
		} else {
			if cmp < 0 {
				return false
			}
		}
	}
	return true
}
