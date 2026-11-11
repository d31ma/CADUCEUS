// @Service
type AdminProblemService struct{}

func (problems *AdminProblemService) Of(failure error) string {
	return problems.named(problems.statusOf(failure))
}

func (problems *AdminProblemService) statusOf(failure error) int {
	if failure == nil {
		return problems.noError()
	}
	return BehaviorErrorStatus(failure)
}

func (problems *AdminProblemService) named(status int) string {
	name, known := problems.names()[status]
	if !known {
		return "unavailable"
	}
	return name
}

func (problems *AdminProblemService) names() map[int]string {
	return map[int]string{
		problems.noError(): "",
		400:                problems.invalid(),
		401:                "signedOut",
		403:                "forbidden",
		404:                "notFound",
		409:                "conflict",
		429:                "rateLimited",
		422:                problems.invalid(),
	}
}

func (problems *AdminProblemService) invalid() string {
	return "invalid"
}

func (problems *AdminProblemService) noError() int {
	return -1
}
