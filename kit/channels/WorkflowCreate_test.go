package channels

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

func TestChannelNamesFollowTheNameRules(t *testing.T) {
	cases := []struct {
		name     string
		accepted bool
	}{
		{name: "", accepted: false},
		{name: "General", accepted: false},
		{name: "two words", accepted: false},
		{name: strings.Repeat("a", 81), accepted: false},
		{name: strings.Repeat("a1-_", 20), accepted: true},
	}
	for _, check := range cases {
		t.Run(check.name, func(t *testing.T) {
			_, err := newWorld(t).workflow.Create("Bearer ana", CreateRequest{Name: check.name, Visibility: "public"})
			wantEqual(t, err == nil, check.accepted)
		})
	}
}

func TestARefusedNameNamesTheNameField(t *testing.T) {
	_, err := newWorld(t).workflow.Create("Bearer ana", CreateRequest{Name: "Bad Name", Visibility: "public"})
	wantField(t, err, "name")
}

func TestAnUnknownVisibilityIsRefused(t *testing.T) {
	_, err := newWorld(t).workflow.Create("Bearer ana", CreateRequest{Name: "general", Visibility: "secret"})
	wantField(t, err, "visibility")
}

func TestATopicLongerThanTwoHundredFiftyCharactersIsRefusedOnCreate(t *testing.T) {
	_, err := newWorld(t).workflow.Create("Bearer ana", CreateRequest{Name: "general", Visibility: "public", Topic: strings.Repeat("t", 251)})
	wantField(t, err, "topic")
}

func TestATopicOfTwoHundredFiftyCharactersIsAccepted(t *testing.T) {
	document, err := newWorld(t).workflow.Create("Bearer ana", CreateRequest{Name: "general", Visibility: "private", Topic: strings.Repeat("t", 250)})
	wantNoError(t, err)
	wantEqual(t, len(document.Topic), 250)
}

func TestANameIsUniqueInItsDomain(t *testing.T) {
	w := newWorld(t)
	w.create("Bearer ana", "general", "public")
	_, err := w.workflow.Create("Bearer bo", CreateRequest{Name: "general", Visibility: "private"})
	conflict := wantError[*Conflict](t, err)
	wantContains(t, conflict.Error(), "general")
}

func TestTheSameNameIsFreeInAnotherDomain(t *testing.T) {
	w := newWorld(t)
	w.create("Bearer ana", "general", "public")
	other := w.create("Bearer eve", "general", "public")
	wantEqual(t, other.DomainId, "example.org")
}

func TestCreateStoresTheChannelInTheCallersDomainWithItsWireIds(t *testing.T) {
	w := newWorld(t)
	document := w.create("Bearer ana", "general", "public")
	stored := w.stored("example.com", document)
	wantEqual(t, stored.Name, "general")
	wantEqual(t, stored.CreatedBy, "ana")
	wantEqual(t, stored.CreatedAt, "2026-11-10T12:00:00Z")
	wantEqual(t, document.ConversationId, "cnv_"+document.Id)
	wantEqual(t, stored.ConversationId, "cnv_"+document.Id)
}

func TestCreateJoinsTheCreator(t *testing.T) {
	w := newWorld(t)
	document := w.create("Bearer ana", "secret-plans", "private")
	membership := w.membership("example.com", document, "ana")
	wantEqual(t, membership.ChannelId, document.Id)
	wantEqual(t, membership.Left, false)
	wantEqual(t, w.listed("Bearer ana"), []string{"secret-plans"})
}

func TestCreatePublishesTheChannelAndTheCreatorJoiningToEveryConsumer(t *testing.T) {
	w := newWorld(t)
	w.create("Bearer ana", "general", "public")
	want := []string{"channel.created", "channel.member.added"}
	wantEqual(t, w.types("messaging"), want)
	wantEqual(t, w.types("realtime"), want)
	wantEqual(t, w.types("search"), want)
}

func TestTheCreatedEventCarriesTheWireIds(t *testing.T) {
	w := newWorld(t)
	document := w.create("Bearer ana", "general", "public")
	payload := events.ChannelCreated{}
	wantNoError(t, json.Unmarshal(w.pusher.PushedTo("search")[0].Payload, &payload))
	wantEqual(t, payload.ChannelId, "ch_"+document.Id)
	wantEqual(t, payload.ConversationId, "cnv_"+document.Id)
	wantEqual(t, payload.Name, "general")
}

func TestAnEventThatCannotBeDeliveredStaysInTheChannel(t *testing.T) {
	w := newWorld(t)
	w.pusher.FailFor("messaging", errors.New("messaging is down"))
	document := w.create("Bearer ana", "general", "public")
	wantEqual(t, w.stored("example.com", document).OutboxPending, 1)
	wantEqual(t, w.pusher.PushedTo("search")[0].Type, "channel.created")
	wantEqual(t, len(w.pusher.PushedTo("messaging")), 0)
}

func TestACallerOfAnInvalidDomainCannotCreateAChannel(t *testing.T) {
	_, err := newWorld(t).workflow.Create("Bearer broken", CreateRequest{Name: "general", Visibility: "public"})
	wantTrue(t, err != nil, "a channel was created in an invalid domain")
}

func TestEveryCallWithAnUnknownTokenIsUnauthenticated(t *testing.T) {
	w := newWorld(t)
	document := w.create("Bearer ana", "general", "public")
	_, listErr := w.workflow.List("Bearer nobody")
	_, createErr := w.workflow.Create("Bearer nobody", CreateRequest{Name: "other", Visibility: "public"})
	_, updateErr := w.update("Bearer nobody", document, UpdateRequest{Topic: text("x")})
	wantError[*Unauthenticated](t, listErr)
	wantError[*Unauthenticated](t, createErr)
	wantError[*Unauthenticated](t, updateErr)
	wantError[*Unauthenticated](t, w.join("Bearer nobody", document))
	wantError[*Unauthenticated](t, w.leave("Bearer nobody", document))
	wantError[*Unauthenticated](t, w.invite("Bearer nobody", document, "bo"))
}

func TestWithoutOutboxSettingsNoEventIsStagedAndTheCallIsUnavailable(t *testing.T) {
	w := newWorld(t)
	open := w.create("Bearer ana", "open", "public")
	left := w.create("Bearer ana", "left", "public")
	w.must(w.join("Bearer bo", left))
	w.must(w.leave("Bearer bo", left))
	w.must(w.join("Bearer cy", open))
	w.configure(`{"yon":{}}`)
	_, createErr := w.workflow.Create("Bearer ana", CreateRequest{Name: "newer", Visibility: "public"})
	_, archiveErr := w.update("Bearer ana", open, UpdateRequest{Archived: flag(true)})
	unavailable := wantError[*Unavailable](t, createErr)
	wantTrue(t, errors.Is(unavailable, outbox.MissingSettings{}), "the cause is not kept")
	wantError[*Unavailable](t, archiveErr)
	wantError[*Unavailable](t, w.join("Bearer bo", open))
	wantError[*Unavailable](t, w.join("Bearer bo", left))
	wantError[*Unavailable](t, w.leave("Bearer cy", open))
	_, runErr := w.desk.RunOnce()
	wantError[*Unavailable](t, runErr)
}
