package main

func check(list []int, order string) bool {
	for i := 1; i < len(list); i++ {
		if order == "desc" {
			if list[i] > list[i-1] {
				return false
			}
		} else {
			if list[i] < list[i-1] {
				return false
			}
		}
	}
	return true
}
