package inbound

import (
	"errors"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
)

func deliveryScene(t *testing.T) (*Scene, string) {
	t.Helper()
	scene := NewScene(t)
	records := []core.DeliveryRecord{
		{Recipient: "bo@example.com", MemberId: "bo", Channel: "chat", Role: "to", Status: "delivered"},
		{Recipient: "carol@outside.test", Channel: "email", Role: "to", Status: "queued"},
		{Recipient: "dan@outside.test", Channel: "email", Role: "bcc", Status: "queued"},
	}
	message := core.Message{DomainId: "example.com", Month: "202611", Deliveries: core.DeliveryCodec{}.Encode(records)}
	message.Id, _ = scene.memory.Messages.Insert(message)
	return scene, core.MessageIds{}.Wire(message.Month, message.Id)
}

func deliveries(t *testing.T, scene *Scene) []core.DeliveryRecord {
	t.Helper()
	records, err := core.DeliveryCodec{}.Decode(scene.Messages()[0].Deliveries)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return records
}

func TestDeliveryChangedMovesOnlyForward(t *testing.T) {
	scene, messageId := deliveryScene(t)
	changed := events.Types{}.EmailDeliveryChanged()
	scene.Must(changed, "example.com", events.EmailDeliveryChanged{MessageId: messageId, Rcpt: "Carol@Outside.test", Status: "deferred", Detail: "try later"})
	scene.Must(changed, "example.com", events.EmailDeliveryChanged{MessageId: messageId, Rcpt: "carol@outside.test", Status: "bounced", Detail: "550 no such user"})
	scene.Must(changed, "example.com", events.EmailDeliveryChanged{MessageId: messageId, Rcpt: "carol@outside.test", Status: "deferred", Detail: "late retry"})
	scene.Must(changed, "example.com", events.EmailDeliveryChanged{MessageId: messageId, Rcpt: "carol@outside.test", Status: "queued", Detail: ""})
	records := deliveries(t, scene)
	if records[1].Status != "bounced" || records[1].Detail != "550 no such user" {
		t.Fatalf("carol moved back: %+v", records[1])
	}
	if records[0].Status != "delivered" || records[2].Status != "queued" {
		t.Fatalf("other rows changed: %+v", records)
	}
}

func TestDeliveryChangedUpdatesABccRowAndIgnoresUnknownStatuses(t *testing.T) {
	scene, messageId := deliveryScene(t)
	changed := events.Types{}.EmailDeliveryChanged()
	scene.Must(changed, "example.com", events.EmailDeliveryChanged{MessageId: messageId, Rcpt: "dan@outside.test", Status: "sent"})
	scene.Must(changed, "example.com", events.EmailDeliveryChanged{MessageId: messageId, Rcpt: "dan@outside.test", Status: "lost"})
	scene.Must(changed, "example.com", events.EmailDeliveryChanged{MessageId: messageId, Rcpt: "bo@example.com", Status: "sent"})
	records := deliveries(t, scene)
	if records[2].Status != "sent" || records[0].Status != "delivered" {
		t.Fatalf("got %+v", records)
	}
}

func TestDeliveryChangedForAMissingMessageDoesNothing(t *testing.T) {
	scene, messageId := deliveryScene(t)
	scene.Must(events.Types{}.EmailDeliveryChanged(), "example.org", events.EmailDeliveryChanged{MessageId: messageId, Rcpt: "carol@outside.test", Status: "sent"})
	scene.Must(events.Types{}.EmailDeliveryChanged(), "example.com", events.EmailDeliveryChanged{MessageId: "msg_202611999999999999", Rcpt: "carol@outside.test", Status: "sent"})
	if deliveries(t, scene)[1].Status != "queued" {
		t.Fatalf("a message changed")
	}
}

func TestDeliveryChangedWithABadMessageIdIsRefused(t *testing.T) {
	scene, _ := deliveryScene(t)
	err := scene.Send(events.Types{}.EmailDeliveryChanged(), "example.com", events.EmailDeliveryChanged{MessageId: "cnv_1", Rcpt: "carol@outside.test", Status: "sent"})
	if !errors.As(err, new(*core.NotFound)) {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestDeliveryChangedWithBrokenStoredRowsIsAnError(t *testing.T) {
	scene := NewScene(t)
	scene.memory.Messages.Insert(core.Message{DomainId: "example.com", Month: "202611", Deliveries: []string{"{"}})
	err := scene.Send(events.Types{}.EmailDeliveryChanged(), "example.com", events.EmailDeliveryChanged{MessageId: "msg_202611000000000001", Rcpt: "carol@outside.test", Status: "sent"})
	if err == nil {
		t.Fatalf("want an error")
	}
}

func TestDeliveryChangedReportsStoreFailures(t *testing.T) {
	assertDeliveryChangedFails(t, 1)
	assertDeliveryChangedFails(t, 2)
}

func assertDeliveryChangedFails(t *testing.T, call int) {
	t.Helper()
	scene, messageId := deliveryScene(t)
	scene.FailAt(call)
	err := scene.Send(events.Types{}.EmailDeliveryChanged(), "example.com", events.EmailDeliveryChanged{MessageId: messageId, Rcpt: "carol@outside.test", Status: "sent"})
	if !errors.As(err, new(*core.Unavailable)) {
		t.Fatalf("call %d: want Unavailable, got %v", call, err)
	}
}
