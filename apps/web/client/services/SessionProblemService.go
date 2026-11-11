// @Service
type SessionProblemService struct{}

func (problems *SessionProblemService) OfFailure(failure error) string {
	return problems.named(problems.statusOf(failure))
}

func (problems *SessionProblemService) statusOf(failure error) int {
	if failure == nil {
		return problems.noStatus()
	}
	return BehaviorErrorStatus(failure)
}

func (problems *SessionProblemService) named(status int) string {
	known := map[int]string{
		problems.noStatus(): "",
		401:                 "wrongCredentials",
		429:                 "rateLimited",
	}
	problem, found := known[status]
	return problems.foundOrUnavailable(problem, found)
}

func (problems *SessionProblemService) foundOrUnavailable(problem string, found bool) string {
	if found {
		return problem
	}
	return "unavailable"
}

func (problems *SessionProblemService) noStatus() int {
	return -1
}
