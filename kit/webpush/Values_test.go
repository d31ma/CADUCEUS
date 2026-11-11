package webpush

import (
	"encoding/json"
	"testing"
)

func TestOutcomeAndPlatformValues(t *testing.T) {
	values := map[string]string{
		Outcomes{}.Delivered(): "delivered",
		Outcomes{}.Gone():      "gone",
		Outcomes{}.Retry():     "retry",
		Outcomes{}.Refused():   "refused",
		Platforms{}.Web():      "web",
	}
	for got, want := range values {
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}

func TestMessageJsonNames(t *testing.T) {
	encoded, err := json.Marshal(Message{Title: "t", Body: "b", Tag: "g", Url: "/u", NotificationId: "n", Kind: "k", ConversationId: "c", MessageId: "m"})
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"title":"t","body":"b","tag":"g","url":"/u","notificationId":"n","kind":"k","conversationId":"c","messageId":"m"}`
	if string(encoded) != expected {
		t.Fatalf("json %s", encoded)
	}
}

func TestPushFailureMessage(t *testing.T) {
	message := PushFailure{Status: 413, Detail: "too big"}.Error()
	if message != "the push service did not accept the message (status 413): too big" {
		t.Fatalf("message %q", message)
	}
}
