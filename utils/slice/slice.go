package slice

func Insert(arr []interface{}, item interface{}, pos int) []interface{} {
	if pos < 0 || pos > len(arr) {
		return arr
	}

	res := append(arr[:pos + 1], arr[pos:]...)
	res[pos] = item
	return res
}
