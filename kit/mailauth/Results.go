package mailauth

import "slices"

type Results struct {
	Spf         string
	SpfDomain   string
	Dkim        []DkimResult
	Dmarc       string
	DmarcPolicy string
	FromDomain  string
	Reject      bool
	Suspicious  bool
}

func (results Results) DkimSummary() string {
	return results.summary(slices.IndexFunc(results.Dkim, DkimResult.passed))
}

func (results Results) HeaderField(authServId string) string {
	return authenticationResults{}.field(results, authServId)
}

func (results Results) summary(passIndex int) string {
	switch {
	case passIndex >= 0:
		return Outcomes{}.Pass()
	case len(results.Dkim) > 0:
		return results.Dkim[0].Result
	default:
		return Outcomes{}.None()
	}
}
