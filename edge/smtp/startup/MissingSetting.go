package startup

type MissingSetting struct {
	Name string
}

func (missing MissingSetting) Error() string {
	return "the environment variable " + missing.Name + " is required"
}
