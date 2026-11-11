// @Service
type NotificationProblemService struct{}

func (problems *NotificationProblemService) OfFailure(failure error) string {
	return problems.named(problems.statusOf(failure))
}

func (problems *NotificationProblemService) SignedOut() error {
	return &BehaviorError{Code: "signed_out", Message: "No domain is signed in yet.", Status: problems.signedOutStatus()}
}

func (problems *NotificationProblemService) statusOf(failure error) int {
	if failure == nil {
		return problems.noStatus()
	}
	return BehaviorErrorStatus(failure)
}

func (problems *NotificationProblemService) named(status int) string {
	names := map[int]string{
		problems.noStatus():        "",
		problems.signedOutStatus(): "waiting",
		401:                        "session",
	}
	name, found := names[status]
	return problems.nameOrError(name, found)
}

func (problems *NotificationProblemService) nameOrError(name string, found bool) string {
	if found {
		return name
	}
	return "error"
}

func (problems *NotificationProblemService) noStatus() int {
	return -1
}

func (problems *NotificationProblemService) signedOutStatus() int {
	return -2
}
