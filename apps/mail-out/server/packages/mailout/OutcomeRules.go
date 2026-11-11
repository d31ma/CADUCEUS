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

func (OutcomeRules) unknownMailbox() string { return "5.1.1" }

func (rules OutcomeRules) FromSmtp(transport string, reply smtpclient.Reply, err error) Outcome {
	refusal := smtpclient.DeliveryError{Reply: reply}
	errors.As(err, &refusal)
	sent := Outcome{Status: States{}.Sent(), Transport: transport, Code: refusal.Reply.Code, Enhanced: refusal.Reply.Enhanced, Detail: refusal.Reply.Text}
	failed := sent
	failed.Status, failed.Detail = rules.smtpFailureStatus(refusal.Permanent), rules.errorText(err)
	return rules.pick(err == nil, sent, failed)
}

func (rules OutcomeRules) FromHandOff(status int, err error) Outcome {
	answer, found := rules.handOffAnswers()[status]
	unanswered := Outcome{Status: States{}.Deferred(), Transport: rules.handOff(), Code: 451, Enhanced: "4.4.1", Detail: rules.handOffDetail(status, err)}
	return rules.pick(rules.answered(found, err), answer, unanswered)
}

func (rules OutcomeRules) FromBuild(err error) Outcome {
	return Outcome{Status: rules.buildStatus(rules.unfixable(err)), Transport: "build", Detail: err.Error()}
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

func (rules OutcomeRules) MailboxGone(enhanced string) bool {
	return slices.Contains([]string{"5.1.0", rules.unknownMailbox(), "5.1.10"}, enhanced)
}

func (OutcomeRules) pick(chosen bool, first Outcome, second Outcome) Outcome {
	if chosen {
		return first
	}
	return second
}

func (OutcomeRules) smtpFailureStatus(permanent bool) string {
	if permanent {
		return States{}.Bounced()
	}
	return States{}.Deferred()
}

func (OutcomeRules) errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (OutcomeRules) answered(found bool, err error) bool {
	return found && err == nil
}

func (rules OutcomeRules) handOffDetail(status int, err error) string {
	if err != nil {
		return rules.handOff() + " unreachable: " + err.Error()
	}
	return fmt.Sprintf("%s answered %d", rules.handOff(), status)
}

func (OutcomeRules) unfixable(err error) bool {
	unsafe, missing := compose.UnsafeHeaderValue{}, compose.MissingField{}
	return errors.As(err, &unsafe) || errors.As(err, &missing)
}

func (OutcomeRules) buildStatus(unfixable bool) string {
	if unfixable {
		return States{}.Failed()
	}
	return States{}.Deferred()
}

func (rules OutcomeRules) handOffAnswers() map[int]Outcome {
	sent := Outcome{Status: States{}.Sent(), Transport: rules.handOff(), Code: 250, Enhanced: "2.0.0", Detail: "accepted by mail-in"}
	rejected := Outcome{Status: States{}.Bounced(), Transport: rules.handOff(), Code: 554, Enhanced: "5.6.0", Detail: "refused by mail-in"}
	tooLarge := Outcome{Status: States{}.Bounced(), Transport: rules.handOff(), Code: 552, Enhanced: "5.3.4", Detail: "too large for mail-in"}
	unknown := Outcome{Status: States{}.Bounced(), Transport: rules.handOff(), Code: 550, Enhanced: rules.unknownMailbox(), Detail: "unknown recipient at mail-in"}
	return map[int]Outcome{200: sent, 201: sent, 202: sent, 400: rejected, 404: unknown, 413: tooLarge, 422: rejected}
}
