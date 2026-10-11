// TextUtils holds the small text choices that several services make.
type TextUtils struct{}

// FirstOf answers the preferred text, or the fallback when it is empty.
func (TextUtils) FirstOf(preferred string, fallback string) string {
	return TextUtils{}.either(preferred, fallback)
}

func (TextUtils) either(preferred string, fallback string) string {
	if preferred != "" {
		return preferred
	}
	return fallback
}
