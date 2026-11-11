package mailauth

type DkimResult struct {
	Result   string
	Domain   string
	Selector string
}

func (result DkimResult) passed() bool {
	return result.Result == Outcomes{}.Pass()
}

func (result DkimResult) failed() bool {
	return !result.passed()
}
