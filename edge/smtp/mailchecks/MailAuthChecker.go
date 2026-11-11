package mailchecks

import (
	"context"

	"mesail.dev/edge/smtp/reception"
	"mesail.dev/packages/mailauth"
)

type MailAuthChecker struct {
	checker  Checker
	hostname string
}

func NewMailAuthChecker(checker Checker, hostname string) MailAuthChecker {
	return MailAuthChecker{checker: checker, hostname: hostname}
}

func (auth MailAuthChecker) Check(ctx context.Context, input reception.AuthInput) (verdict reception.AuthVerdict) {
	defer auth.skipOnPanic(ctx, &verdict)
	results := auth.checker.Check(ctx, mailauth.Input{RemoteIp: input.RemoteIp, Helo: input.Helo, MailFrom: input.MailFrom, Raw: input.Raw, ReceivedAt: input.ReceivedAt})
	return auth.verdict(results)
}

func (auth MailAuthChecker) verdict(results mailauth.Results) reception.AuthVerdict {
	return reception.AuthVerdict{
		Results: reception.AuthResults{
			Spf:         results.Spf,
			Dkim:        results.DkimSummary(),
			Dmarc:       results.Dmarc,
			DmarcPolicy: results.DmarcPolicy,
			Suspicious:  results.Suspicious,
		},
		Reject:      results.Reject,
		HeaderField: results.HeaderField(auth.hostname),
	}
}

func (MailAuthChecker) skipOnPanic(ctx context.Context, verdict *reception.AuthVerdict) {
	if recover() != nil {
		*verdict = reception.SkippedChecks{}.Check(ctx, reception.AuthInput{})
	}
}
