import "net/http"

// PreferenceProblem is why a preference update was refused: a 400 answer, or
// a 403 when it names a domain or account the user is not assigned.
type PreferenceProblem struct {
	Message   string
	Forbidden bool
}

// Answer answers the refusal.
func (problem *PreferenceProblem) Answer() *http.Response {
	if problem.Forbidden {
		return Answer{}.Forbidden(problem.Message)
	}
	return Answer{}.BadRequest(problem.Message)
}
