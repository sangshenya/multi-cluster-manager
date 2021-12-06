package sliceutil

func ContainsString(slice []string, s string) bool {
	if len(slice) == 0 {
		return false
	}
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func RemoveString(slice []string, s string) (result []string) {
	if len(slice) == 0 {
		return
	}
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}