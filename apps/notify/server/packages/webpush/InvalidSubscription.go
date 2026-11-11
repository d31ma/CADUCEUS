package webpush

type InvalidSubscription struct {
	Field string
}

func (invalid InvalidSubscription) Error() string {
	return "the push subscription has an invalid " + invalid.Field + " value"
}
