package startup

type MissingSetting struct {
	Name string
}

func (problem MissingSetting) Error() string {
	return problem.Name + " must be set"
}
