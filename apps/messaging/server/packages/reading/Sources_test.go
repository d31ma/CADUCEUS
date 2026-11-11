package reading_test

import (
	"fmt"
	"testing"

	"mesail.dev/packages/core"
	"mesail.dev/packages/oidc"
	"mesail.dev/packages/reading"
)

func service(domainId string) oidc.Caller {
	return oidc.Caller{DomainId: domainId, Roles: []string{"Service"}}
}

func TestTheSourceGivesBodyAndAttachments(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	message.AttachmentIds = []string{"att_1"}
	s.world.Stores.Messages.Save(message)
	source, err := reading.NewSources(s.stores, s.world.Ports).Source(service("example.com"), msgId(message))
	wantNoError(t, err)
	want := core.MessageSource{MessageId: msgId(message), DomainId: "example.com", Body: "hello", AttachmentIds: []string{"att_1"}}
	if fmt.Sprint(source) != fmt.Sprint(want) {
		t.Fatalf("got %+v", source)
	}
}

func TestTheSourceOfAnotherDomainIsNotFound(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	_, err := reading.NewSources(s.stores, s.world.Ports).Source(service("example.org"), msgId(message))
	wantError[*core.NotFound](t, err)
}

func TestTheSourceOfAMalformedIdIsNotFound(t *testing.T) {
	s := newScene(t)
	_, err := reading.NewSources(s.stores, s.world.Ports).Source(service("example.com"), "cnv_1")
	wantError[*core.NotFound](t, err)
}

func TestTheSourceReportsABrokenStore(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	s.breaker.broken["messages.find"] = true
	_, err := reading.NewSources(s.stores, s.world.Ports).Source(service("example.com"), msgId(message))
	wantError[*core.Unavailable](t, err)
}

func TestTheOriginalOfAReceivedEmailIsADownload(t *testing.T) {
	s := newScene(t)
	conversation, _ := s.chat()
	email := s.post(conversation, core.Message{Sequence: 2, AuthorKind: "external", IsEmail: true, Email: core.EmailMeta{InboundId: "in1"}})
	s.world.Inbound.Originals["example.com/in1"] = core.Download{Url: "https://files.example.com/in1.eml", ExpiresAt: "2026-11-10T13:00:00Z"}
	download, err := reading.NewSources(s.stores, s.world.Ports).Original(s.caller("bo"), msgId(email))
	wantNoError(t, err)
	if download.Url != "https://files.example.com/in1.eml" {
		t.Fatalf("got %+v", download)
	}
}

func TestAChatMessageHasNoOriginal(t *testing.T) {
	s := newScene(t)
	_, message := s.chat()
	_, err := reading.NewSources(s.stores, s.world.Ports).Original(s.caller("bo"), msgId(message))
	wantError[*core.NotFound](t, err)
}

func TestTheOriginalNeedsAReader(t *testing.T) {
	s := newScene(t)
	conversation, _ := s.chat()
	email := s.post(conversation, core.Message{Sequence: 2, Email: core.EmailMeta{InboundId: "in1"}})
	_, err := reading.NewSources(s.stores, s.world.Ports).Original(s.caller("cy"), msgId(email))
	wantError[*core.NotFound](t, err)
}
