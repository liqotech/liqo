package args

import (
	"strings"
)

// StringList implements the flag.Value interface and allows to parse stringified lists
// in the form: "val1,val2".
type StringList struct {
	StringList []string
}

// String returns the stringified list.
func (sl StringList) String() string {
	if sl.StringList == nil {
		return ""
	}
	return strings.Join(sl.StringList, ",")
}

// Set parses the provided string into the []string list.
func (sl *StringList) Set(str string) error {
	if sl.StringList == nil {
		sl.StringList = []string{}
	}
	if str == "" {
		return nil
	}
	chunks := strings.Split(str, ",")
	for i := range chunks {
		chunk := chunks[i]
		sl.StringList = append(sl.StringList, chunk)
	}
	return nil
}

// Type returns the stringList type.
func (sl StringList) Type() string {
	return "stringList"
}
