package transport

type RequestProblem struct {
	Status int
	Text   string
}

func (problem RequestProblem) Error() string {
	return problem.Text
}
