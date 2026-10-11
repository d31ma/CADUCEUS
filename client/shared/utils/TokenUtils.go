import "strings"

// TokenUtils checks identifiers by their characters and length. It replaces
// regular expressions with large repetition counts such as {1,200}: TinyGo
// compiles those into a deep program that overflowed the 64 KB stack of a
// page call and crashed the module ("null function").
type TokenUtils struct{}

// Fits is true when value has between shortest and longest characters, all
// of them in allowed.
func (TokenUtils) Fits(value string, allowed string, bounds TokenBounds) bool {
	return TokenUtils{}.fits(len(value), strings.Trim(value, allowed) == "", bounds)
}

// Identifier is the character set of opaque server identifiers.
func (TokenUtils) Identifier() string {
	return "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
}

func (TokenUtils) fits(length int, allowedOnly bool, bounds TokenBounds) bool {
	return allowedOnly && length >= bounds.Shortest && length <= bounds.Longest
}
