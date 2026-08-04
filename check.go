package main

import "sorted/parser"

func check(list []*parser.Value, order string) bool {
	for i := 1; i < len(list); i++ {
		if order == "desc" {
			if list[i].Max.Cmp(list[i-1].Min) > 0 {
				return false
			}
		} else {
			if list[i].Min.Cmp(list[i-1].Max) < 0 {
				return false
			}
		}
	}
	return true
}
