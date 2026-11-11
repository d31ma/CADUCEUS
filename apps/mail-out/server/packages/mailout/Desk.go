package mailout

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"mesail.dev/packages/events"
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

func (desk Desk) Receive(authorization string, body any) (events.Receipt, error) {
	if desk.Failure != nil {
		return events.Receipt{}, desk.Failure
	}
	envelope := events.Envelope{}
	raw, err := json.Marshal(body)
	if err == nil {
		err = json.Unmarshal(raw, &envelope)
	}
	if err != nil {
		return events.Receipt{}, events.InvalidEnvelope{Reason: err.Error()}
	}
	caller, _ := desk.Gate.Identify(authorization)
	if slices.Contains(caller.Roles, "Service") && !strings.EqualFold(caller.DomainId, envelope.DomainId) {
		return events.Receipt{}, DomainMismatch{DomainId: envelope.DomainId}
	}
	return events.NewReceiver(events.Services{}.MailOut(), desk.Seen, desk.Intake).Receive(caller, envelope)
}

func (desk Desk) Deliver() (QueueReport, error) {
	if desk.Failure != nil {
		return QueueReport{}, desk.Failure
	}
	return desk.Run.Run(desk.Now())
}

func (desk Desk) Relay() (outbox.Report, error) {
	if desk.Failure != nil {
		return outbox.Report{}, desk.Failure
	}
	return desk.Relayer.RunOnce(desk.Limit)
}

func (desk Desk) Pending() ([]outbox.Document, error) {
	if desk.Failure != nil {
		return nil, desk.Failure
	}
	return desk.Relayer.Pending(desk.Limit)
}

func (desk Desk) DeadLetters() ([]outbox.DeadLetter, error) {
	if desk.Failure != nil {
		return nil, desk.Failure
	}
	return desk.Relayer.DeadLetters(desk.Limit)
}

func (desk Desk) Requeue(request RequeueRequest) (outbox.Report, error) {
	if desk.Failure != nil {
		return outbox.Report{}, desk.Failure
	}
	return desk.Relayer.Requeue(request)
}
