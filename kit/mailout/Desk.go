package mailout

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/outbox"
)

type Desk struct {
	Gate    CallerGate
	Seen    SeenEvents
	Intake  Intake
	Run     QueueRun
	Relayer Relayer
	Now     func() time.Time
	Limit   int
	Failure error
}

func (Desk) serviceRole() string { return "Service" }

func (desk Desk) Receive(authorization string, body any) (events.Receipt, error) {
	return failureGuard[events.Receipt]{failure: desk.Failure}.run(func() (events.Receipt, error) {
		return desk.receive(authorization, body)
	})
}

func (desk Desk) Deliver() (QueueReport, error) {
	return failureGuard[QueueReport]{failure: desk.Failure}.run(desk.deliver)
}

func (desk Desk) Relay() (outbox.Report, error) {
	return failureGuard[outbox.Report]{failure: desk.Failure}.run(desk.relay)
}

func (desk Desk) Pending() ([]outbox.Document, error) {
	return failureGuard[[]outbox.Document]{failure: desk.Failure}.run(desk.pending)
}

func (desk Desk) DeadLetters() ([]outbox.DeadLetter, error) {
	return failureGuard[[]outbox.DeadLetter]{failure: desk.Failure}.run(desk.deadLetters)
}

func (desk Desk) Requeue(request RequeueRequest) (outbox.Report, error) {
	return failureGuard[outbox.Report]{failure: desk.Failure}.run(func() (outbox.Report, error) {
		return desk.Relayer.Requeue(request)
	})
}

func (desk Desk) receive(authorization string, body any) (events.Receipt, error) {
	envelope, err := desk.envelope(body)
	if err != nil {
		return events.Receipt{}, err
	}
	caller := desk.caller(authorization)
	err = desk.domainCheck(caller, envelope.DomainId)
	if err != nil {
		return events.Receipt{}, err
	}
	return events.NewReceiver(events.Services{}.MailOut(), desk.Seen, desk.Intake).Receive(caller, envelope)
}

func (desk Desk) envelope(body any) (events.Envelope, error) {
	envelope, err := desk.decode(body)
	if err != nil {
		return events.Envelope{}, events.InvalidEnvelope{Reason: err.Error()}
	}
	return envelope, nil
}

func (Desk) decode(body any) (events.Envelope, error) {
	envelope := events.Envelope{}
	raw, err := json.Marshal(body)
	if err != nil {
		return envelope, err
	}
	return envelope, json.Unmarshal(raw, &envelope)
}

func (desk Desk) caller(authorization string) oidc.Caller {
	caller, err := desk.Gate.Identify(authorization)
	if err != nil {
		return oidc.Caller{}
	}
	return caller
}

func (desk Desk) domainCheck(caller oidc.Caller, domainId string) error {
	if desk.mayActFor(caller, domainId) {
		return nil
	}
	return DomainMismatch{DomainId: domainId}
}

func (desk Desk) mayActFor(caller oidc.Caller, domainId string) bool {
	return !slices.Contains(caller.Roles, desk.serviceRole()) || strings.EqualFold(caller.DomainId, domainId)
}

func (desk Desk) deliver() (QueueReport, error) {
	return desk.Run.Run(desk.Now())
}

func (desk Desk) relay() (outbox.Report, error) {
	return desk.Relayer.RunOnce(desk.Limit)
}

func (desk Desk) pending() ([]outbox.Document, error) {
	return desk.Relayer.Pending(desk.Limit)
}

func (desk Desk) deadLetters() ([]outbox.DeadLetter, error) {
	return desk.Relayer.DeadLetters(desk.Limit)
}
