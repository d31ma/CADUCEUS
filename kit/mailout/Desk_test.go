package mailout

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

func equal(t *testing.T, want any, got any) {
	t.Helper()
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("\nwant %#v\n got %#v", want, got)
	}
}

func TestHostedRecipientIsHandedToMailInSignedAndThreaded(t *testing.T) {
	w := newWorld(t, func(w *world) { w.answers = []int{503, 201} })
	envelope := w.event("example.com", "Alice <alice@example.com>", "eve@example.org")
	envelope.Payload.To = []string{"eve@example.org", "Bob <bob@partner.test>"}
	envelope.Payload.Cc = []string{"carol@example.org"}

	receipt, err := w.send(envelope)
	equal(t, nil, err)
	equal(t, events.Receipt{EventID: envelope.EventId, Status: "applied"}, receipt)
	first, err := w.desk.Deliver()
	equal(t, nil, err)
	equal(t, QueueReport{Picked: 1, Deferred: 1}, first)
	equal(t, QueueReport{Picked: 1, Sent: 1}, w.run(time.Now().Add(2*time.Minute)))

	item := w.item("example.com", envelope.EventId)
	raw := w.raw(item)
	equal(t, "sent", item.State)
	equal(t, "example.com/outbound-"+item.Id, item.RawBlobKey)
	equal(t, InboundEnvelope{
		EnvelopeFrom: "alice@example.com", Recipients: []string{"eve@example.org"}, RawBlobKey: item.RawBlobKey, Size: int64(len(raw)),
		RemoteAddress: "internal", Helo: "mail-out", Tls: true,
		Headers: InboundHeaders{
			MessageId: envelope.Payload.MessageId + "@example.com", InReplyTo: "parent@example.org", References: []string{"root@example.org", "parent@example.org"},
			Subject: "Lunch on Friday", From: MailboxAddress{Name: "Alice", Address: "alice@example.com"},
			To: []MailboxAddress{{Address: "eve@example.org"}, {Name: "Bob", Address: "bob@partner.test"}}, Cc: []MailboxAddress{{Address: "carol@example.org"}},
		},
		AuthResults: AuthResults{DmarcPolicy: "none", Handoff: true},
	}, w.handOffs[1])
	equal(t, 0, len(w.relay.Messages()))
	equal(t, true, strings.HasPrefix(raw, "DKIM-Signature:"))
	equal(t, true, strings.Contains(header(raw, "DKIM-Signature"), "d=example.com;"))
	equal(t, true, strings.Contains(header(raw, "DKIM-Signature"), "s=mesail;"))
	equal(t, "<parent@example.org>", header(raw, "In-Reply-To"))
	equal(t, "<root@example.org> <parent@example.org>", header(raw, "References"))
	equal(t, "<"+envelope.Payload.MessageId+"@example.com>", header(raw, "Message-ID"))
	equal(t, true, strings.Contains(header(raw, "To"), "bob@partner.test"))
	equal(t, true, strings.Contains(header(raw, "Cc"), "carol@example.org"))
	equal(t, "", header(raw, "Bcc"))
	equal(t, "", header(raw, "Authentication-Results"))
	equal(t, true, strings.Contains(raw, "see you **soon**."))
	equal(t, []string{
		"mail-out example.com eve@example.org queued ",
		"mail-out example.com eve@example.org deferred mail-in answered 503",
		"mail-out example.com eve@example.org sent accepted by mail-in",
	}, w.changes())
	equal(t, []string{
		"push example.com",
		"directory example.com example.com", "source messaging example.com", "directory example.org mesail-operators", "mail-in example.org", "push example.com",
		"directory example.com example.com", "source messaging example.com", "directory example.org mesail-operators", "mail-in example.org", "push example.com",
	}, w.recorded())
	equal(t, 0, item.OutboxPending)
}

func TestBccCopyGoesToTheRelayWithoutTheBccAddressInHeaders(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "alice@example.com", "dave@partner.test")
	envelope.Payload.Role, envelope.Payload.To = "bcc", []string{"eve@example.org"}
	w.mustSend(envelope)

	equal(t, QueueReport{Picked: 1, Sent: 1}, w.run(time.Now()))
	message := w.relay.Messages()[0]
	headers, _, _ := strings.Cut(string(message.Data), "\r\n\r\n")
	equal(t, []string{"dave@partner.test"}, message.Recipients)
	equal(t, "alice@example.com", message.From)
	equal(t, true, strings.Contains(header(string(message.Data), "To"), "eve@example.org"))
	equal(t, false, strings.Contains(headers, "dave@partner.test"))
	equal(t, 0, len(w.handOffs))
}

func TestMessagesWithOnlyBccRecipientsShowTheSender(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "Alice <alice@example.com>", "dave@partner.test")
	envelope.Payload.Role, envelope.Payload.To = "bcc", []string{}
	w.mustSend(envelope)

	equal(t, QueueReport{Picked: 1, Sent: 1}, w.run(time.Now()))
	equal(t, true, strings.Contains(header(string(w.relay.Messages()[0].Data), "To"), "alice@example.com"))
}

func TestTemporaryReplyDefersWithBackoff(t *testing.T) {
	w := newWorld(t, func(w *world) {
		w.rejected, w.rejectCode, w.rejectText = "bob@partner.test", 451, "4.3.0 try again later"
	})
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	now := time.Now()
	w.mustSend(envelope)

	equal(t, QueueReport{Picked: 1, Deferred: 1}, w.run(now))
	item := w.item("example.com", envelope.EventId)
	equal(t, Clock{}.Format(now.Add(time.Minute)), item.NextAttemptAt)
	equal(t, QueueReport{}, w.run(now.Add(30*time.Second)))
	equal(t, QueueReport{Picked: 1, Deferred: 1}, w.run(now.Add(61*time.Second)))
	later := w.item("example.com", envelope.EventId)
	equal(t, Clock{}.Format(now.Add(61*time.Second).Add(2*time.Minute)), later.NextAttemptAt)
	equal(t, 2, later.Attempts)
	attempts, err := w.desk.Run.Delivery.Outcomes.Attempts.ForItem("example.com", item.Id)
	equal(t, nil, err)
	equal(t, 2, len(attempts))
	equal(t, "4.3.0", attempts[0].Enhanced)
	equal(t, 451, attempts[0].Code)
}

func TestUnknownMailboxBouncesAndIsSuppressedOnlyInTheSendersDomain(t *testing.T) {
	w := newWorld(t, func(w *world) {
		w.rejected, w.rejectCode, w.rejectText = "gone@partner.test", 550, "5.1.1 no such user"
	})
	suppressions := w.desk.Run.Delivery.Suppressions
	bounced := w.event("example.com", "alice@example.com", "gone@partner.test")
	again := w.event("example.com", "alice@example.com", "gone@partner.test")
	other := w.event("example.org", "eve@example.org", "gone@partner.test")
	now := time.Now()

	w.mustSend(bounced)
	equal(t, QueueReport{Picked: 1, Bounced: 1}, w.run(now))
	inSender, _ := suppressions.Contains("example.com", "GONE@partner.test")
	inOther, _ := suppressions.Contains("example.org", "gone@partner.test")
	equal(t, true, inSender)
	equal(t, false, inOther)
	w.rejecting = false
	w.mustSend(again)
	w.mustSend(other)
	equal(t, QueueReport{Picked: 2, Sent: 1, Failed: 1}, w.run(now.Add(time.Second)))
	equal(t, "suppressed", w.item("example.com", again.EventId).Detail)
	equal(t, "sent", w.item("example.org", other.EventId).State)
	equal(t, "eve@example.org", w.relay.Messages()[0].From)
}

func TestItemExpiresAfterTheMaxAge(t *testing.T) {
	w := newWorld(t, func(w *world) {
		w.rejected, w.rejectCode, w.rejectText = "bob@partner.test", 421, "4.4.2 busy"
		w.settings.MaxAgeMinutes = 10
	})
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	now := time.Now()
	w.mustSend(envelope)

	equal(t, QueueReport{Picked: 1, Deferred: 1}, w.run(now))
	equal(t, QueueReport{Picked: 1, Expired: 1}, w.run(now.Add(11*time.Minute)))
	statuses := w.changes()
	equal(t, "expired", w.item("example.com", envelope.EventId).State)
	equal(t, "mail-out example.com bob@partner.test failed expired", statuses[len(statuses)-1])
}

func TestADeferralPastTheMaxAgeExpires(t *testing.T) {
	w := newWorld(t, func(w *world) {
		w.rejected, w.rejectCode, w.rejectText = "bob@partner.test", 421, "4.4.2 busy"
		w.settings.MaxAgeMinutes = 1
		w.settings.FirstDelaySeconds = 120
	})
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	w.mustSend(envelope)

	equal(t, QueueReport{Picked: 1, Expired: 1}, w.run(time.Now()))
}

func TestRepeatedEventAndIdempotencyKeyMakeOneItem(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	sameKey := w.event("example.com", "alice@example.com", "bob@partner.test")
	sameKey.Payload = envelope.Payload

	first, _ := w.send(envelope)
	repeated, repeatedErr := w.send(envelope)
	other, otherErr := w.send(sameKey)

	equal(t, "applied", first.Status)
	equal(t, nil, repeatedErr)
	equal(t, events.Receipt{EventID: envelope.EventId, Status: "duplicate"}, repeated)
	equal(t, nil, otherErr)
	equal(t, "applied", other.Status)
	equal(t, QueueReport{Picked: 1, Sent: 1}, w.run(time.Now()))
	equal(t, 1, len(w.relay.Messages()))
	equal(t, 2, len(w.changes()))
}

func TestDirectDeliveryUsesTheMxHost(t *testing.T) {
	w := newWorld(t, func(w *world) {
		w.settings.DirectEnabled, w.settings.DirectMxHost, w.settings.DirectPort, w.settings.RelayAddress = true, "127.0.0.1", w.relay.Port(), "relay.invalid:25"
	})
	w.mustSend(w.event("example.com", "alice@example.com", "bob@partner.test"))

	equal(t, QueueReport{Picked: 1, Sent: 1}, w.run(time.Now()))
	equal(t, []string{"bob@partner.test"}, w.relay.Messages()[0].Recipients)
}

func TestAMissingDkimKeyDefersTheItem(t *testing.T) {
	w := newWorld(t, func(w *world) {
		w.domains["example.com"] = Domain{DomainId: "example.com", State: "active", DkimSelector: "missing"}
	})
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	w.mustSend(envelope)

	equal(t, QueueReport{Picked: 1, Deferred: 1}, w.run(time.Now()))
	equal(t, 0, len(w.relay.Messages()))
	equal(t, false, strings.Contains(w.item("example.com", envelope.EventId).Detail, "PRIVATE"))
}

func TestAHeaderLineBreakFailsTheItem(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	envelope.Payload.Subject = "Hello\r\nBcc: mallory@partner.test"
	w.mustSend(envelope)

	equal(t, QueueReport{Picked: 1, Failed: 1}, w.run(time.Now()))
	equal(t, 0, len(w.relay.Messages()))
}

func TestSenderAndSourceRefusals(t *testing.T) {
	cases := map[string]func(*world, sendEnvelope){
		"the sender domain is not active": func(w *world, envelope sendEnvelope) {
			w.domains["example.com"] = Domain{DomainId: "example.com", State: "disabled", DkimSelector: "mesail"}
		},
		"the sender domain is not hosted": func(w *world, envelope sendEnvelope) { delete(w.domains, "example.com") },
		"the message source belongs to another domain": func(w *world, envelope sendEnvelope) {
			w.sources["messaging "+envelope.Payload.MessageId] = MessageSource{MessageId: envelope.Payload.MessageId, DomainId: "example.org", Body: "secret"}
		},
	}
	for detail, change := range cases {
		t.Run(detail, func(t *testing.T) {
			w := newWorld(t, nil)
			envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
			w.mustSend(envelope)
			change(w, envelope)
			equal(t, QueueReport{Picked: 1, Failed: 1}, w.run(time.Now()))
			equal(t, detail, w.item("example.com", envelope.EventId).Detail)
			equal(t, 0, len(w.relay.Messages()))
		})
	}
}

func TestAMissingSourceOrDirectoryDefers(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	w.mustSend(envelope)
	delete(w.sources, "messaging "+envelope.Payload.MessageId)

	equal(t, QueueReport{Picked: 1, Deferred: 1}, w.run(time.Now()))
	equal(t, "the producer answered 404 for the message source", w.item("example.com", envelope.EventId).Detail)
}

func TestAnInactiveRecipientDomainGoesToTheRelay(t *testing.T) {
	w := newWorld(t, func(w *world) { w.domains["paused.test"] = Domain{DomainId: "paused.test", State: "pending"} })
	w.mustSend(w.event("example.com", "alice@example.com", "zoe@paused.test"))

	equal(t, QueueReport{Picked: 1, Sent: 1}, w.run(time.Now()))
	equal(t, 0, len(w.handOffs))
	equal(t, 1, len(w.relay.Messages()))
}

func TestAnUnreachableDirectoryForTheRecipientDefers(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	w.mustSend(envelope)
	w.desk.Run.Delivery.Transport.Directory.BaseUrl = "http://127.0.0.1:1"

	equal(t, QueueReport{Picked: 1, Deferred: 1}, w.run(time.Now()))
	equal(t, "the directory did not answer", w.item("example.com", envelope.EventId).Detail)
}

func TestHandOffAnswersMapToStates(t *testing.T) {
	cases := map[int]string{201: "sent", 404: "bounced", 413: "bounced", 400: "bounced", 500: "deferred"}
	for answer, state := range cases {
		w := newWorld(t, func(w *world) { w.answers = []int{answer} })
		envelope := w.event("example.com", "alice@example.com", "eve@example.org")
		w.mustSend(envelope)
		w.run(time.Now())
		equal(t, state, w.item("example.com", envelope.EventId).State)
	}
}

func TestNotifyIsASourceProducer(t *testing.T) {
	w := newWorld(t, nil)
	w.sources["notify msg_summary"] = MessageSource{MessageId: "msg_summary", DomainId: "example.com", Body: "Your summary"}
	sources := w.desk.Intake.Sources

	source, err := sources.Source(Item{DomainId: "example.com", Producer: "notify", MessageId: "msg_summary"})
	_, unknownErr := sources.Source(Item{DomainId: "example.com", Producer: "search", MessageId: "msg_summary"})

	equal(t, nil, err)
	equal(t, "Your summary", source.Body)
	equal(t, `no message source is known for the producer "search"`, unknownErr.Error())
	equal(t, []string{"source notify example.com"}, w.recorded())
}

func TestReceiveRefusesBadCallersAndEvents(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	foreignSender := w.event("example.com", "mallory@example.org", "bob@partner.test")
	wrongProducer := w.event("example.com", "alice@example.com", "bob@partner.test")
	wrongProducer.Producer = "channels"
	unknownProducer := w.event("example.com", "alice@example.com", "bob@partner.test")
	unknownProducer.Producer = "search"

	_, withoutToken := w.desk.Receive("", envelope)
	_, member := w.desk.Receive("Bearer member-example.com", envelope)
	_, otherRealm := w.desk.Receive("Bearer token-example.org", envelope)
	_, sender := w.send(foreignSender)
	_, producer := w.send(wrongProducer)
	_, body := w.desk.Receive("Bearer token-example.com", make(chan int))

	notService, mismatch, invalid := events.NotService{}, DomainMismatch{}, events.InvalidEnvelope{}
	equal(t, true, errors.As(withoutToken, &notService))
	equal(t, true, errors.As(member, &notService))
	equal(t, true, errors.As(otherRealm, &mismatch))
	equal(t, "the caller may not act for the domain example.com", otherRealm.Error())
	equal(t, true, strings.Contains(sender.Error(), "from is missing or not valid"))
	equal(t, true, errors.As(producer, &invalid))
	equal(t, true, errors.As(body, &invalid))
	equal(t, 0, len(w.changes()))
}

func TestTheIntakeRefusesAnUnknownProducer(t *testing.T) {
	w := newWorld(t, nil)
	err := w.desk.Intake.Handle(events.Envelope{EventID: "e", Type: "email.send.requested", DomainId: "example.com", Producer: "search", Payload: []byte(`{}`)})
	equal(t, true, strings.Contains(err.Error(), "producer"))
}

func TestTheOutboxKeepsEventsUntilMessagingAccepts(t *testing.T) {
	w := newWorld(t, func(w *world) { w.eventAnswer = http.StatusServiceUnavailable })
	w.mustSend(w.event("example.com", "alice@example.com", "bob@partner.test"))

	waiting, err := w.desk.Pending()
	equal(t, nil, err)
	equal(t, 1, len(waiting))
	equal(t, 1, waiting[0].Fields.OutboxPending)
	w.eventAnswer = http.StatusOK
	time.Sleep(1100 * time.Millisecond)
	report, err := w.desk.Relay()
	equal(t, nil, err)
	equal(t, outbox.Report{Documents: 1, Delivered: 1}, report)
	after, _ := w.desk.Pending()
	equal(t, 0, len(after))
	equal(t, 2, len(w.changes()))
}

func TestADeadLetterCanBeSentAgain(t *testing.T) {
	w := newWorld(t, func(w *world) { w.eventAnswer = http.StatusBadRequest })
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	w.mustSend(envelope)
	item := w.item("example.com", envelope.EventId)

	dead, err := w.desk.DeadLetters()
	equal(t, nil, err)
	equal(t, 1, len(dead))
	equal(t, "messaging", dead[0].Subscriber)
	w.eventAnswer = http.StatusOK
	report, err := w.desk.Requeue(RequeueRequest{DomainId: "example.com", ItemId: item.Id, EventId: dead[0].Envelope.EventID, Subscriber: "messaging"})
	equal(t, nil, err)
	equal(t, 1, report.Delivered)
	after, _ := w.desk.DeadLetters()
	equal(t, 0, len(after))
}

func TestRequeueReportsProblems(t *testing.T) {
	w := newWorld(t, nil)
	_, missing := w.desk.Requeue(RequeueRequest{DomainId: "example.com", ItemId: "nothing", EventId: "e", Subscriber: "messaging"})
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	w.mustSend(envelope)
	item := w.item("example.com", envelope.EventId)
	_, unknownLetter := w.desk.Requeue(RequeueRequest{DomainId: "example.com", ItemId: item.Id, EventId: "e", Subscriber: "messaging"})
	w.documents.fail("Patch", errors.New("store down"))
	_, saveErr := w.desk.Relayer.Requeue(RequeueRequest{DomainId: "example.com", ItemId: item.Id, EventId: "e", Subscriber: "messaging"})

	notFound := outbox.NotFound{}
	equal(t, true, errors.As(missing, &notFound))
	equal(t, true, unknownLetter != nil)
	equal(t, true, saveErr != nil)
}

func TestARunningClaimIsNotDeliveredTwice(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	w.mustSend(envelope)
	now := time.Now()
	item := w.item("example.com", envelope.EventId)
	_, free, err := w.desk.Run.Queue.Claim(item, now)
	equal(t, true, free)
	equal(t, nil, err)

	done, err := w.desk.Run.Delivery.Deliver(item, now)
	equal(t, nil, err)
	equal(t, "queued", done.State)
	equal(t, 0, len(w.relay.Messages()))
}

func TestAFailedDeskReportsItsProblem(t *testing.T) {
	problem := errors.New("settings are broken")
	desk := Desk{Failure: problem}
	_, receiveErr := desk.Receive("", nil)
	_, deliverErr := desk.Deliver()
	_, relayErr := desk.Relay()
	_, pendingErr := desk.Pending()
	_, deadErr := desk.DeadLetters()
	_, requeueErr := desk.Requeue(RequeueRequest{})
	for _, err := range []error{receiveErr, deliverErr, relayErr, pendingErr, deadErr, requeueErr} {
		equal(t, problem, err)
	}
}

func TestAttachmentsAreSentFromTheDomainFolder(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	w.sources["messaging "+envelope.Payload.MessageId] = MessageSource{MessageId: envelope.Payload.MessageId, DomainId: "example.com", Body: "see attached", AttachmentIds: []string{"att_1"}}
	w.mustSend(envelope)
	err := w.desk.Run.Delivery.Builder.Blobs.Write(blobKey("example.com", "attachment-att_1"), strings.NewReader("pdf bytes"), 9)
	equal(t, nil, err)

	equal(t, QueueReport{Picked: 1, Sent: 1}, w.run(time.Now()))
	equal(t, true, strings.Contains(string(w.relay.Messages()[0].Data), "filename=att_1"))
}

func TestAMissingAttachmentDefers(t *testing.T) {
	w := newWorld(t, nil)
	envelope := w.event("example.com", "alice@example.com", "bob@partner.test")
	w.sources["messaging "+envelope.Payload.MessageId] = MessageSource{MessageId: envelope.Payload.MessageId, DomainId: "example.com", AttachmentIds: []string{"att_2"}}
	w.mustSend(envelope)

	equal(t, QueueReport{Picked: 1, Deferred: 1}, w.run(time.Now()))
}
