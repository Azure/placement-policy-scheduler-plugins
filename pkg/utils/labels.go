package utils

// HasMatchingLabels checks if the labels exist in the provided set
func HasMatchingLabels(l, wantLabels map[string]string) bool {
	if len(l) < len(wantLabels) {
		return false
	}

	for k, v := range wantLabels {
		if l[k] != v {
			return false
		}
	}
	return true
}
