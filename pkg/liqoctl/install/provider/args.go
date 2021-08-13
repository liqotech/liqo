package provider

import (
	"time"

	flag "github.com/spf13/pflag"
)

// CommonArguments encapsulates all the arguments common across install providers.
type CommonArguments struct {
	Version string
	Debug   bool
	Timeout time.Duration
}

// ValidateCommonArguments validates install common arguments. If the inputs are valid, it returns a *CommonArgument
// with all the parameters contents.
func ValidateCommonArguments(flags *flag.FlagSet) (*CommonArguments, error) {
	version, err := flags.GetString("version")
	if err != nil {
		return nil, err
	}
	debug, err := flags.GetBool("debug")
	if err != nil {
		return nil, err
	}
	timeout, err := flags.GetInt("timeout")
	if err != nil {
		return nil, err
	}
	return &CommonArguments{
		Version: version,
		Debug:   debug,
		Timeout: time.Duration(timeout) * time.Second,
	}, nil
}
