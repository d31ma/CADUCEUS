package failures

import "mesail.dev/packages/apierrors"

type Problem struct {
	Status int
	Code   string
	Text   string
	Field  string
	Case   int
}

func (problem Problem) Error() string {
	return problem.Message()
}

func (problem Problem) Number() int {
	return problem.Case
}

func (problem Problem) Message() string {
	return apierrors.Message{}.Format(problem.Case, problem.Text)
}
