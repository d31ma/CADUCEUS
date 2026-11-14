package realtime

import "strings"

type IdText struct{}

func (text IdText) Fits(value string, minimum int) bool {
	return len(value) >= minimum && len(value) <= SignalLimits{}.MaximumIdLength() && text.plain(value)
}

func (IdText) plain(value string) bool {
	return strings.Trim(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") == ""
}
