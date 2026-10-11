import "slices"

// ChoiceUtils answers the small yes-or-no questions of the settings
// sections: which of two texts a condition picks, and whether all or any of
// several conditions hold. Keeping them here keeps the services' methods
// free of inline branches.
type ChoiceUtils struct{}

// Text answers chosen when the condition holds, and other otherwise.
func (ChoiceUtils) Text(condition bool, chosen string, other string) string {
	return map[bool]string{true: chosen, false: other}[condition]
}

// All is true when every condition holds.
func (ChoiceUtils) All(conditions ...bool) bool {
	return !slices.Contains(conditions, false)
}

// Any is true when at least one condition holds.
func (ChoiceUtils) Any(conditions ...bool) bool {
	return slices.Contains(conditions, true)
}
