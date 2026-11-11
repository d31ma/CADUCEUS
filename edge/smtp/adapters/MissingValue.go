package adapters

type MissingValue struct {
	Name string
}

func (missing MissingValue) Error() string {
	return "the answer has no " + missing.Name
}
