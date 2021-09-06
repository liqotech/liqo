package args

import (
	"fmt"
	"strconv"
)

// Percentage implements the flag.Value interface and allows to parse stringified percentages.
type Percentage struct {
	Val uint64
}

// String returns the stringified percentage.
func (p Percentage) String() string {
	return fmt.Sprintf("%v", p.Val)
}

// Set parses the provided string into the percentage.
func (p *Percentage) Set(str string) error {
	if str == "" {
		return nil
	}
	val, err := strconv.ParseUint(str, 10, 64)
	if err != nil {
		return err
	}
	p.Val = val

	if p.Val > 100 {
		return fmt.Errorf("invalid percentage value: %v. It has to be in range [0 - 100]", str)
	}

	return nil
}

// Type returns the percentage type.
func (p Percentage) Type() string {
	return "percentage"
}
