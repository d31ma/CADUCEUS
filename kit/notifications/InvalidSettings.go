package notifications

type InvalidSettings struct {
	Field  string
	Reason string
}

func (failure InvalidSettings) Error() string {
	return "the notification settings are not valid: " + failure.Field + " " + failure.Reason
}
