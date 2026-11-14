package startup

type InvalidAddress struct {
	Name  string
	Value string
}

func (problem InvalidAddress) Error() string {
	return problem.Name + " holds " + problem.Value + ", which is not an IP address"
}
