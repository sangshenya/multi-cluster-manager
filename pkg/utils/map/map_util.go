package _map

func ContainsMap(big, sub map[string]string) bool {
	for k, v := range sub {
		value, ok := big[k]
		if !ok || value != v {
			return false
		}
	}
	return true
}
