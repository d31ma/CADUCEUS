package core

import (
	"reflect"
	"testing"
)

func TestAttachmentRefsComeBackFromTheStoredText(t *testing.T) {
	refs := []AttachmentRef{{AttachmentId: "att_1", FileName: "plan.pdf", MediaType: "application/pdf", Size: 2048}}
	message := Message{AttachmentIds: []string{"att_1"}, AttachmentsJson: AttachmentCodec{}.Encode(refs)}
	got := AttachmentCodec{}.Refs(message)
	if !reflect.DeepEqual(got, refs) {
		t.Fatalf("got %+v", got)
	}
}

func TestNoAttachmentsEncodeAsAnEmptyList(t *testing.T) {
	encoded := AttachmentCodec{}.Encode(nil)
	if encoded != "[]" {
		t.Fatalf("got %q", encoded)
	}
}

func TestAnOldMessageKeepsItsIdsAlone(t *testing.T) {
	got := AttachmentCodec{}.Refs(Message{AttachmentIds: []string{"att_1", "att_2"}})
	want := []AttachmentRef{{AttachmentId: "att_1"}, {AttachmentId: "att_2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestTooLargeSaysWhy(t *testing.T) {
	if (&TooLarge{Message: "too big"}).Error() != "too big" {
		t.Fatal("the message was lost")
	}
}
