package startup

type InvalidSetting struct {
	Name   string
	Reason string
}

func (invalid InvalidSetting) Error() string {
	return "the environment variable " + invalid.Name + " is not valid: " + invalid.Reason
}
