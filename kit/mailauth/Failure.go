package mailauth

import "errors"

type failure struct {
	result string
}

func (problem failure) Error() string {
	return problem.result
}

func (failure) outcome(err error, success string) string {
	if err == nil {
		return success
	}
	found := failure{result: Outcomes{}.PermError()}
	errors.As(err, &found)
	return found.result
}
