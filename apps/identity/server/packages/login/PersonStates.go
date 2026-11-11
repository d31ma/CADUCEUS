package login

type PersonStates struct{}

func (PersonStates) Disabled() string { return "disabled" }
func (PersonStates) Deleted() string  { return "deleted" }
