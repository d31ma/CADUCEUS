package mailauth

import "slices"

type verdict struct {
	outcomes Outcomes
}

func (judge verdict) decide(results Results) Results {
	results.Reject = judge.rejected(results)
	results.Suspicious = judge.suspicious(results)
	return results
}

func (judge verdict) rejected(results Results) bool {
	return results.Dmarc == judge.outcomes.Fail() && results.DmarcPolicy == judge.outcomes.Reject()
}

func (judge verdict) suspicious(results Results) bool {
	return !results.Reject && (results.Dmarc == judge.outcomes.Fail() || judge.spfFailed(results.Spf) || judge.unverified(results.Dkim))
}

func (judge verdict) spfFailed(spf string) bool {
	return spf == judge.outcomes.Fail() || spf == judge.outcomes.SoftFail()
}

func (verdict) unverified(dkim []DkimResult) bool {
	return len(dkim) > 0 && !slices.ContainsFunc(dkim, DkimResult.passed)
}
