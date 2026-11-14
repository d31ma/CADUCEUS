package outbox

type attemptGroups struct {
	byKind map[string][]pushAttempt
}

func newAttemptGroups(attempts []pushAttempt) attemptGroups {
	byKind := map[string][]pushAttempt{}
	for _, attempt := range attempts {
		kind := attempt.kind()
		byKind[kind] = append(byKind[kind], attempt)
	}
	return attemptGroups{byKind: byKind}
}

func (groups attemptGroups) of(kind string) []pushAttempt {
	return groups.byKind[kind]
}

func (groups attemptGroups) count(kind string) int {
	return len(groups.byKind[kind])
}

func (groups attemptGroups) names(kind string) []string {
	names := []string{}
	for _, attempt := range groups.byKind[kind] {
		names = append(names, attempt.subscriber)
	}
	return names
}

func (groups attemptGroups) remaining() []string {
	kinds := attemptKind{}
	return append(groups.names(kinds.retry()), groups.names(kinds.skipped())...)
}

func (groups attemptGroups) lastError() string {
	failed := groups.of(attemptKind{}.retry())
	return failed[len(failed)-1].err.Error()
}
