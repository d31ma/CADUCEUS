// @Service
type SessionProblemService struct{}

func (problems *SessionProblemService) OfState(expected string, actual string) string {
	return problems.startAgainUnless(expected, actual)
}

func (problems *SessionProblemService) startAgainUnless(expected string, actual string) string {
	if expected != "" && expected == actual {
		return ""
	}
	return problems.startAgain()
}

func (problems *SessionProblemService) OfFailure(failure error) string {
	return problems.named(problems.statusOf(failure))
}

func (problems *SessionProblemService) OfSignInError(code string) string {
	known := map[string]string{
		"access_denied":           "cancelled",
		"login_required":          "cancelled",
		"temporarily_unavailable": problems.unreachable(),
		"invalid_request":         problems.startAgain(),
	}
	return problems.orUnreachable(known[code])
}

func (problems *SessionProblemService) Unavailable() string {
	return "unavailable"
}

func (problems *SessionProblemService) statusOf(failure error) int {
	if failure == nil {
		return problems.noStatus()
	}
	return BehaviorErrorStatus(failure)
}

func (problems *SessionProblemService) named(status int) string {
	known := map[int]string{
		problems.noStatus():  "",
		problems.forbidden(): "accountOff",
	}
	problem, found := known[status]
	return problems.foundOrUnreachable(problem, found)
}

func (problems *SessionProblemService) foundOrUnreachable(problem string, found bool) string {
	if found {
		return problem
	}
	return problems.unreachable()
}

func (problems *SessionProblemService) orUnreachable(problem string) string {
	if problem == "" {
		return problems.unreachable()
	}
	return problem
}

func (problems *SessionProblemService) startAgain() string {
	return "startAgain"
}

func (problems *SessionProblemService) unreachable() string {
	return "idpDown"
}

func (problems *SessionProblemService) noStatus() int {
	return -1
}

func (problems *SessionProblemService) forbidden() int {
	return 403
}
