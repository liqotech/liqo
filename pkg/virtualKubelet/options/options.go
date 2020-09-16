package options

type OptionKey string
type OptionValue string

type ReadOnlyOption interface {
	Key() OptionKey
	Value() OptionValue
}

type Option interface {
	ReadOnlyOption

	SetValue(OptionValue)
}

func (k OptionKey) ToString() string {
	return string(k)
}

func (v OptionValue) ToString() string {
	return string(v)
}
