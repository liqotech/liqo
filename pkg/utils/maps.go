package utils

// MergeMaps merges two maps.
func MergeMaps(m1, m2 map[string]string) map[string]string {
	if m1 == nil {
		return m2
	}
	for k, v := range m2 {
		m1[k] = v
	}
	return m1
}

// SubMaps removes elements of m2 from m1.
func SubMaps(m1, m2 map[string]string) map[string]string {
	for k := range m2 {
		delete(m1, k)
	}
	return m1
}
