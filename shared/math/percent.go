package math

// IsGEPercentLimit returns true if the percentage of number val from total
// is bigger or equal to limitPercent
func IsGEPercentLimit(val, total int, limitPercent int) bool {
	if total == 0 {
		return val > 0
	}

	percent := val * 100 / total
	return percent >= limitPercent
}
