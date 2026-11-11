package reception

import "context"

type SkippedChecks struct{}

func (checks SkippedChecks) Check(_ context.Context, _ AuthInput) AuthVerdict {
	results := AuthResults{Spf: checks.none(), Dkim: checks.none(), Dmarc: checks.none(), DmarcPolicy: checks.none()}
	return AuthVerdict{Results: results}
}

func (SkippedChecks) none() string {
	return "none"
}
