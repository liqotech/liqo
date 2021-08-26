package installutils

// GetInterfaceSlice casts a slice of string to a slice in interface{}.
func GetInterfaceSlice(in []string) []interface{} {
	out := make([]interface{}, len(in))
	for i := range in {
		out[i] = in[i]
	}
	return out
}
