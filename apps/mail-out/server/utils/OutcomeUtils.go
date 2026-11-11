import (
	"errors"
	"fmt"

	"mesail.dev/packages/compose"
	"mesail.dev/packages/smtpclient"
)

type OutcomeUtils struct{}

func (OutcomeUtils) handOff() string { return "mail-in" }

func (OutcomeUtils) queue() string { return "queue" }

func (utils OutcomeUtils) FromSmtp(transport string, reply smtpclient.Reply, failure error) Outcome {
	refusal := smtpclient.DeliveryError{Reply: reply}
	errors.As(failure, &refusal)
	return Outcome{Status: utils.smtpStatus(failure, refusal.Permanent), Transport: transport, Code: refusal.Reply.Code, Enhanced: refusal.Reply.Enhanced, Detail: utils.detail(refusal.Reply.Text, failure)}
}

func (utils OutcomeUtils) FromHandOff(status int, failure error) Outcome {
	outcome, found := utils.handOffAnswers()[status]
	return utils.handOffOutcome(outcome, found, utils.handOffDetail(status, failure), failure)
}

func (utils OutcomeUtils) FromBuild(failure error) Outcome {
	return Outcome{Status: utils.buildStatus(failure), Transport: "build", Detail: failure.Error()}
}

func (utils OutcomeUtils) Expired() Outcome {
	return Outcome{Status: OutboundStateUtils{}.Expired(), Transport: utils.queue(), Detail: "expired"}
}

func (utils OutcomeUtils) Refused(reason string) Outcome {
	return Outcome{Status: OutboundStateUtils{}.Failed(), Transport: utils.queue(), Detail: reason}
}

func (utils OutcomeUtils) Suppressed() Outcome {
	return Outcome{Status: OutboundStateUtils{}.Failed(), Transport: utils.queue(), Detail: "suppressed"}
}

func (OutcomeUtils) smtpStatus(failure error, permanent bool) string {
	switch {
	case failure == nil:
		return OutboundStateUtils{}.Sent()
	case permanent:
		return OutboundStateUtils{}.Bounced()
	default:
		return OutboundStateUtils{}.Deferred()
	}
}

func (OutcomeUtils) detail(text string, failure error) string {
	if failure != nil {
		return failure.Error()
	}
	return text
}

func (utils OutcomeUtils) handOffAnswers() map[int]Outcome {
	sent := Outcome{Status: OutboundStateUtils{}.Sent(), Transport: utils.handOff(), Code: 250, Enhanced: "2.0.0", Detail: "accepted by mail-in"}
	rejected := Outcome{Status: OutboundStateUtils{}.Bounced(), Transport: utils.handOff(), Code: 554, Enhanced: "5.6.0", Detail: "refused by mail-in"}
	tooLarge := Outcome{Status: OutboundStateUtils{}.Bounced(), Transport: utils.handOff(), Code: 552, Enhanced: "5.3.4", Detail: "too large for mail-in"}
	unknown := Outcome{Status: OutboundStateUtils{}.Bounced(), Transport: utils.handOff(), Code: 550, Enhanced: "5.1.1", Detail: "unknown recipient at mail-in"}
	return map[int]Outcome{200: sent, 201: sent, 202: sent, 400: rejected, 404: unknown, 413: tooLarge, 422: rejected}
}

func (utils OutcomeUtils) handOffOutcome(outcome Outcome, found bool, detail string, failure error) Outcome {
	if !found || failure != nil {
		return Outcome{Status: OutboundStateUtils{}.Deferred(), Transport: utils.handOff(), Code: 451, Enhanced: "4.4.1", Detail: detail}
	}
	return outcome
}

func (OutcomeUtils) handOffDetail(status int, failure error) string {
	if failure != nil {
		return "mail-in unreachable: " + failure.Error()
	}
	return fmt.Sprintf("mail-in answered %d", status)
}

func (OutcomeUtils) buildStatus(failure error) string {
	unsafe := compose.UnsafeHeaderValue{}
	missing := compose.MissingField{}
	if errors.As(failure, &unsafe) || errors.As(failure, &missing) {
		return OutboundStateUtils{}.Failed()
	}
	return OutboundStateUtils{}.Deferred()
}
