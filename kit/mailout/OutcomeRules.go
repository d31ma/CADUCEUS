package mailout

import (
	"errors"
	"fmt"
	"slices"

	"mesail.dev/packages/compose"
	"mesail.dev/packages/smtpclient"
)

type OutcomeRules struct{}

func (OutcomeRules) handOff() string { return "mail-in" }

func (OutcomeRules) queue() string { return "queue" }

func (rules OutcomeRules) FromSmtp(transport string, reply smtpclient.Reply, err error) Outcome {
	refusal := smtpclient.DeliveryError{Reply: reply}
	errors.As(err, &refusal)
	outcome := Outcome{Status: States{}.Sent(), Transport: transport, Code: refusal.Reply.Code, Enhanced: refusal.Reply.Enhanced, Detail: refusal.Reply.Text}
	if err == nil {
		return outcome
	}
	outcome.Status, outcome.Detail = States{}.Deferred(), err.Error()
	if refusal.Permanent {
		outcome.Status = States{}.Bounced()
	}
	return outcome
}

func (rules OutcomeRules) FromHandOff(status int, err error) Outcome {
	outcome, found := rules.handOffAnswers()[status]
	if found && err == nil {
		return outcome
	}
	detail := fmt.Sprintf("mail-in answered %d", status)
	if err != nil {
		detail = "mail-in unreachable: " + err.Error()
	}
	return Outcome{Status: States{}.Deferred(), Transport: rules.handOff(), Code: 451, Enhanced: "4.4.1", Detail: detail}
}

func (rules OutcomeRules) FromBuild(err error) Outcome {
	unsafe, missing := compose.UnsafeHeaderValue{}, compose.MissingField{}
	status := States{}.Deferred()
	if errors.As(err, &unsafe) || errors.As(err, &missing) {
		status = States{}.Failed()
	}
	return Outcome{Status: status, Transport: "build", Detail: err.Error()}
}

func (rules OutcomeRules) Expired() Outcome {
	return Outcome{Status: States{}.Expired(), Transport: rules.queue(), Detail: "expired"}
}

func (rules OutcomeRules) Suppressed() Outcome {
	return Outcome{Status: States{}.Failed(), Transport: rules.queue(), Detail: "suppressed"}
}

func (rules OutcomeRules) Refused(reason string) Outcome {
	return Outcome{Status: States{}.Failed(), Transport: rules.queue(), Detail: reason}
}

func (rules OutcomeRules) Unavailable(err error) Outcome {
	return Outcome{Status: States{}.Deferred(), Transport: rules.queue(), Detail: err.Error()}
}

func (OutcomeRules) MailboxGone(enhanced string) bool {
	return slices.Contains([]string{"5.1.0", "5.1.1", "5.1.10"}, enhanced)
}

func (rules OutcomeRules) handOffAnswers() map[int]Outcome {
	sent := Outcome{Status: States{}.Sent(), Transport: rules.handOff(), Code: 250, Enhanced: "2.0.0", Detail: "accepted by mail-in"}
	rejected := Outcome{Status: States{}.Bounced(), Transport: rules.handOff(), Code: 554, Enhanced: "5.6.0", Detail: "refused by mail-in"}
	tooLarge := Outcome{Status: States{}.Bounced(), Transport: rules.handOff(), Code: 552, Enhanced: "5.3.4", Detail: "too large for mail-in"}
	unknown := Outcome{Status: States{}.Bounced(), Transport: rules.handOff(), Code: 550, Enhanced: "5.1.1", Detail: "unknown recipient at mail-in"}
	return map[int]Outcome{200: sent, 201: sent, 202: sent, 400: rejected, 404: unknown, 413: tooLarge, 422: rejected}
}
