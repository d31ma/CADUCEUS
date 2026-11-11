package outbox

import (
	"errors"
	"testing"

	"mesail.dev/packages/events"
)

type channelPayload struct {
	ChannelId string `json:"channelId"`
}

func TestPublishedOneDecodesTheStagedPayload(t *testing.T) {
	fields := stageAll(t, testSettings(), channelCreated("evt_1"))
	payload := channelPayload{}

	envelope, err := Published{}.One(fields, events.Types{}.ChannelCreated(), &payload)

	if err != nil || envelope.EventID != "evt_1" || payload.ChannelId != "chn_1" {
		t.Fatalf("envelope %+v, payload %+v, error %v", envelope, payload, err)
	}
}

func TestPublishedOneReportsWhenNothingWasStaged(t *testing.T) {
	fields := stageAll(t, testSettings(), channelCreated("evt_1"))

	_, err := Published{}.One(fields, events.Types{}.ChannelArchived(), &channelPayload{})

	failure := NotPublished{}
	if !errors.As(err, &failure) || failure.Count != 0 || err.Error() != "expected exactly one staged channel.archived event, found 0" {
		t.Fatalf("error %v", err)
	}
}

func TestPublishedOneReportsTwoStagedEvents(t *testing.T) {
	fields := stageAll(t, testSettings(), channelCreated("evt_1"), channelCreated("evt_2"))

	staged, stagedErr := Published{}.Staged(fields, events.Types{}.ChannelCreated())
	_, err := Published{}.One(fields, events.Types{}.ChannelCreated(), &channelPayload{})

	if stagedErr != nil || len(staged) != 2 || !errors.As(err, &NotPublished{}) {
		t.Fatalf("staged %+v, errors %v %v", staged, stagedErr, err)
	}
}

func TestPublishedOneReportsFieldsItCannotRead(t *testing.T) {
	_, err := Published{}.One(Fields{PendingEvents: []string{"not json"}}, events.Types{}.ChannelCreated(), &channelPayload{})

	if err == nil {
		t.Fatal("broken fields should be reported")
	}
}

func TestPublishedOneReportsAPayloadOfAnotherShape(t *testing.T) {
	fields := stageAll(t, testSettings(), channelCreated("evt_1"))
	payload := struct {
		Other string `json:"other"`
	}{}

	_, err := Published{}.One(fields, events.Types{}.ChannelCreated(), &payload)

	if err == nil {
		t.Fatal("an unknown payload field should be reported")
	}
}
