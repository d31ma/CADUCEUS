package sending

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"mesail.dev/packages/core"
)

func attachedRequest(attachmentIds ...string) core.SendRequest {
	sendRequest := request("c1", "files", to("bo@example.com", "chat", "member"))
	sendRequest.AttachmentIds = attachmentIds
	return sendRequest
}

func wantNothingWritten(t *testing.T, s *scene) {
	t.Helper()
	if len(s.messages()) != 0 || len(s.conversations()) != 0 || len(storedUses(s)) != 0 {
		t.Fatal("nothing may be written")
	}
}

func TestTheMessageKeepsTheDetailsOfItsAttachments(t *testing.T) {
	s := newScene(t)
	s.world.Attachments.Infos["example.com/att_1"] = core.AttachmentInfo{AttachmentId: "att_1", FileName: "plan.pdf", MediaType: "application/pdf", Size: 2048, State: "ready"}
	outcome := s.mustSend(t, attachedRequest("att_1", "att_2"))
	want := []core.AttachmentRef{
		{AttachmentId: "att_1", FileName: "plan.pdf", MediaType: "application/pdf", Size: 2048},
		{AttachmentId: "att_2", FileName: "file-2.txt", MediaType: "application/octet-stream", Size: 1000},
	}
	stored := s.stored(t, outcome)
	if got := (core.AttachmentCodec{}).Refs(stored); !reflect.DeepEqual(got, want) {
		t.Fatalf("stored %+v", got)
	}
	if got := (core.AttachmentCodec{}).Refs(outcome.Message.Message); !reflect.DeepEqual(got, want) {
		t.Fatalf("answered %+v", got)
	}
	if !slices.Equal(stored.AttachmentIds, []string{"att_1", "att_2"}) {
		t.Fatalf("ids %v", stored.AttachmentIds)
	}
	if names := created(t, stored).FileNames; !slices.Equal(names, []string{"plan.pdf", "file-2.txt"}) {
		t.Fatalf("file names %v", names)
	}
}

func TestAMessageWithoutAttachmentsAsksNothing(t *testing.T) {
	s := newScene(t)
	outcome := s.mustSend(t, request("c1", "hi", to("bo@example.com", "chat", "member")))
	if len(s.world.Attachments.Asked) != 0 || s.stored(t, outcome).AttachmentsJson != "[]" {
		t.Fatalf("asked %v", s.world.Attachments.Asked)
	}
	if names := created(t, outcome.Message.Message).FileNames; len(names) != 0 {
		t.Fatalf("file names %v", names)
	}
}

func TestAnUnknownAttachmentIsInvalid(t *testing.T) {
	s := newScene(t)
	s.world.Attachments.Ready("example.org", "att_x", "other.txt", 10)
	for _, attachmentId := range []string{"att_missing", "att_x"} {
		_, err := s.send(attachedRequest("att_1", attachmentId))
		wantInvalid(t, err, "attachmentIds")
	}
	wantNothingWritten(t, s)
}

func TestAnAttachmentThatIsNotReadyIsInvalid(t *testing.T) {
	s := newScene(t)
	for _, state := range []string{"pending", "blocked"} {
		s.world.Attachments.Infos["example.com/att_1"] = core.AttachmentInfo{AttachmentId: "att_1", FileName: "a.txt", Size: 1, State: state}
		_, err := s.send(attachedRequest("att_1"))
		wantInvalid(t, err, "attachmentIds")
	}
	wantNothingWritten(t, s)
}

func TestAttachmentsOverFiftyMebibytesAreTooLarge(t *testing.T) {
	s := newScene(t)
	s.world.Attachments.Ready("example.com", "att_1", "a.bin", 25<<20)
	s.world.Attachments.Ready("example.com", "att_2", "b.bin", 25<<20+1)
	_, err := s.send(attachedRequest("att_1", "att_2"))
	tooLarge := &core.TooLarge{}
	if !errors.As(err, &tooLarge) || tooLarge.Message == "" {
		t.Fatalf("want TooLarge, got %#v", err)
	}
	wantNothingWritten(t, s)
}

func TestAttachmentsOfExactlyFiftyMebibytesAreAllowed(t *testing.T) {
	s := newScene(t)
	s.world.Attachments.Ready("example.com", "att_1", "a.bin", 25<<20)
	s.world.Attachments.Ready("example.com", "att_2", "b.bin", 25<<20)
	s.mustSend(t, attachedRequest("att_1", "att_2"))
}

func TestAnAttachmentsFailureAnswersUnavailable(t *testing.T) {
	s := newScene(t)
	s.world.Attachments.Failure = boom()
	_, err := s.send(attachedRequest("att_1"))
	wantUnavailable(t, err)
	wantNothingWritten(t, s)
}

func TestARetryDoesNotAskTheAttachmentsServiceAgain(t *testing.T) {
	s := newScene(t)
	first := s.mustSend(t, attachedRequest("att_1"))
	s.world.Attachments.Failure = boom()
	again := s.mustSend(t, attachedRequest("att_1"))
	if len(s.world.Attachments.Asked) != 1 || wireOf(again.Message.Message) != wireOf(first.Message.Message) {
		t.Fatalf("asked %v", s.world.Attachments.Asked)
	}
}
