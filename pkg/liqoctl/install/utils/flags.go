package installutils

import (
	"fmt"

	flag "github.com/spf13/pflag"
)

// PrefixedName returns the paramenter name with the providerPrefix.
func PrefixedName(providerPrefix, name string) string {
	return fmt.Sprintf("%v.%v", providerPrefix, name)
}

// CheckStringFlagIsSet checks that a string flag is set and returns its value.
func CheckStringFlagIsSet(flags *flag.FlagSet, providerPrefix, name string) (string, error) {
	value, err := flags.GetString(PrefixedName(providerPrefix, name))
	if err != nil {
		return "", err
	}
	if value == "" {
		err := fmt.Errorf("--%v.%v not provided", providerPrefix, name)
		return "", err
	}
	return value, nil
}
