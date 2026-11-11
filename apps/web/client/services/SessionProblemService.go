import "strings"

// @Service
type SessionProblemService struct{}

func (problems *SessionProblemService) OfSignIn(failure error, login string) string {
	return problems.namedSignIn(problems.statusOf(failure), problems.forbiddenProblem(login))
}

func (problems *SessionProblemService) OfLogin(login string) string {
	ending := problems.lastCharacter(strings.TrimSpace(login))
	return problems.incompleteWhen(problems.incompleteEndings()[ending])
}

func (problems *SessionProblemService) OfSwitch(failure error) string {
	known := map[int]string{
		problems.noStatus(): "",
		401:                 "ended",
		403:                 "ended",
		404:                 "notMember",
	}
	problem, found := known[problems.statusOf(failure)]
	return problems.foundOrUnavailable(problem, found)
}

func (problems *SessionProblemService) OfDefaultDomain(failure error) string {
	known := map[int]string{
		problems.noStatus(): "",
		404:                 "notMember",
	}
	problem, found := known[problems.statusOf(failure)]
	return problems.foundOrUnavailable(problem, found)
}

func (problems *SessionProblemService) OfAccountChange(failure error, refused string) string {
	return problems.namedChange(problems.statusOf(failure), refused)
}

func (problems *SessionProblemService) namedChange(status int, refused string) string {
	known := map[int]string{
		problems.noStatus(): "",
		400:                 refused,
		401:                 "wrongPassword",
		429:                 "rateLimited",
	}
	problem, found := known[status]
	return problems.foundOrUnavailable(problem, found)
}

func (problems *SessionProblemService) namedSignIn(status int, forbidden string) string {
	known := map[int]string{
		problems.noStatus(): "",
		401:                 "wrongCredentials",
		403:                 forbidden,
		429:                 "rateLimited",
	}
	problem, found := known[status]
	return problems.foundOrUnavailable(problem, found)
}

func (problems *SessionProblemService) forbiddenProblem(login string) string {
	if strings.Contains(login, problems.at()) {
		return "notAMember"
	}
	return "noDomain"
}

func (problems *SessionProblemService) incompleteWhen(incomplete bool) string {
	if incomplete {
		return "invalidLogin"
	}
	return ""
}

func (problems *SessionProblemService) lastCharacter(text string) string {
	padded := problems.space() + text
	return padded[len(padded)-1:]
}

func (problems *SessionProblemService) incompleteEndings() map[string]bool {
	return map[string]bool{problems.space(): true, problems.at(): true}
}

func (problems *SessionProblemService) statusOf(failure error) int {
	if failure == nil {
		return problems.noStatus()
	}
	return BehaviorErrorStatus(failure)
}

func (problems *SessionProblemService) foundOrUnavailable(problem string, found bool) string {
	if found {
		return problem
	}
	return "unavailable"
}

func (problems *SessionProblemService) space() string {
	return " "
}

func (problems *SessionProblemService) at() string {
	return "@"
}

func (problems *SessionProblemService) noStatus() int {
	return -1
}
