package dnscheck

type stateChoice struct {
	names StateNames
}

func (choice stateChoice) found(records []string) bool {
	return len(records) > choice.noRecords()
}

func (choice stateChoice) missingUnless(found bool, state string) string {
	if found {
		return state
	}
	return choice.names.Missing()
}

func (choice stateChoice) okWhen(valid bool) string {
	if valid {
		return choice.names.Ok()
	}
	return choice.names.Wrong()
}

func (choice stateChoice) both(first bool, second bool) bool {
	return first && second
}

func (stateChoice) noRecords() int {
	return 0
}
