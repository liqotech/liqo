package installutils

import (
	"fmt"

	flag "github.com/spf13/pflag"
)

// CheckStringFlagIsSet checks that a string flag is set and returns its value.
func CheckStringFlagIsSet(flags *flag.FlagSet, name string) (string, error) {
	value, err := flags.GetString(name)
	if err != nil {
		return "", err
	}
	if value == "" {
		err := fmt.Errorf("--%v not provided", name)
		return "", err
	}
	return value, nil
}
