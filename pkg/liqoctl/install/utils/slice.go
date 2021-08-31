package installutils

// GetInterfaceSlice casts a slice of string to a slice in interface{}.
func GetInterfaceSlice(in []string) []interface{} {
	out := make([]interface{}, len(in))
	for i := range in {
		out[i] = in[i]
	}
	return out
}

// GetInterfaceMap casts a map of [string]string to a map of [string]interface{}.
func GetInterfaceMap(in map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
