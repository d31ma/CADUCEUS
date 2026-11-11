package events

import (
	"errors"
	"reflect"
	"testing"
)

func TestCatalogNamesTheProducerOfEveryType(t *testing.T) {
	producer, err := Catalog{}.ProducerOf(Types{}.EmailReceived())
	if err != nil || producer != "mail-in" {
		t.Fatalf("got %q, %v", producer, err)
	}
}

func TestCatalogRefusesAnUnknownType(t *testing.T) {
	_, err := Catalog{}.ProducerOf("unknown.type")
	var unknown UnknownEventType
	if !errors.As(err, &unknown) || unknown.Type != "unknown.type" {
		t.Fatalf("got %v", err)
	}
}

func TestUnknownEventTypeExplainsItself(t *testing.T) {
	want := "the event type x.y is not in the Mesail event catalog"
	got := UnknownEventType{Type: "x.y"}.Error()
	if got != want {
		t.Fatalf("got %q", got)
	}
}

func TestCatalogListsTheConsumersOfAType(t *testing.T) {
	got := Catalog{}.ConsumersOf(Types{}.MessageCreated())
	want := []string{"realtime", "notify", "search"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestCatalogListsNoConsumersOfAnUnknownType(t *testing.T) {
	got := Catalog{}.ConsumersOf("unknown.type")
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestCatalogKnowsEveryEventType(t *testing.T) {
	types := Types{}
	names := []string{
		types.MemberUpserted(), types.ChannelCreated(), types.ChannelMemberAdded(),
		types.ChannelMemberRemoved(), types.ChannelArchived(), types.MessageCreated(),
		types.MessageEdited(), types.MessageDeleted(), types.ReactionChanged(),
		types.ReadPositionMoved(), types.EmailSendRequested(), types.EmailDeliveryChanged(),
		types.EmailReceived(), types.PresenceChanged(),
	}
	for _, name := range names {
		if !(Catalog{}).Known(name) {
			t.Fatalf("%s is not in the catalog", name)
		}
	}
}

func TestCatalogDoesNotKnowAMadeUpType(t *testing.T) {
	if (Catalog{}).Known("made.up") {
		t.Fatal("made.up should be unknown")
	}
}

func TestServicesNameEveryMesailService(t *testing.T) {
	services := Services{}
	got := []string{services.Identity(), services.Directory(), services.Channels(), services.Messaging(),
		services.Attachments(), services.MailIn(), services.MailOut(), services.Search(), services.Realtime(), services.Notify()}
	want := []string{"identity", "directory", "channels", "messaging", "attachments", "mail-in", "mail-out", "search", "realtime", "notify"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}
