package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"mesail.dev/edge/smtp/reception"
)

type mailInCall struct {
	path          string
	authorization string
	body          map[string]any
}

func mailInServer(t *testing.T, status int, answer string, call *mailInCall) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call.path = request.URL.Path
		call.authorization = request.Header.Get("Authorization")
		content, _ := io.ReadAll(request.Body)
		json.Unmarshal(content, &call.body)
		writer.WriteHeader(status)
		writer.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func sampleEnvelope() reception.InboundEnvelope {
	return reception.InboundEnvelope{
		EnvelopeFrom: "dan@outside.test",
		Recipients:   []string{"ana@example.com"},
		RawBlobKey:   "example.com/inbound/abc.eml",
		Size:         42,
		Headers:      reception.InboundHeaders{References: []string{}, To: []reception.MailboxAddress{}, Cc: []reception.MailboxAddress{}},
		AuthResults:  reception.AuthResults{Spf: "pass", Dkim: "none", Dmarc: "pass", DmarcPolicy: "none"},
	}
}

func TestMailInCreatedGivesTheInboundId(t *testing.T) {
	call := &mailInCall{}
	address := mailInServer(t, http.StatusCreated, `{"inboundId":"in_1"}`, call)
	inboundId, err := NewHttpMailIn(address, NewStaticTokens("service-token"), http.DefaultClient).Accept(context.Background(), sampleEnvelope())
	if err != nil || inboundId != "in_1" {
		t.Fatalf("unexpected answer %q %v", inboundId, err)
	}
	if call.path != "/api/v1/mail-in/messages" || call.authorization != "Bearer service-token" {
		t.Fatalf("unexpected call %+v", call)
	}
}

func TestMailInBodyUsesTheWireNames(t *testing.T) {
	call := &mailInCall{}
	address := mailInServer(t, http.StatusCreated, `{"inboundId":"in_1"}`, call)
	NewHttpMailIn(address, NewStaticTokens("t"), http.DefaultClient).Accept(context.Background(), sampleEnvelope())
	headers := call.body["headers"].(map[string]any)
	results := call.body["authResults"].(map[string]any)
	if call.body["rawBlobKey"] != "example.com/inbound/abc.eml" || call.body["envelopeFrom"] != "dan@outside.test" || call.body["size"] != float64(42) {
		t.Fatalf("unexpected body %+v", call.body)
	}
	if headers["references"] == nil || results["dmarcPolicy"] != "none" || results["suspicious"] != false {
		t.Fatalf("unexpected nested body %+v %+v", headers, results)
	}
}

func TestMailInOkIsNotEnough(t *testing.T) {
	address := mailInServer(t, http.StatusOK, `{"inboundId":"in_1"}`, &mailInCall{})
	_, err := NewHttpMailIn(address, NewStaticTokens("t"), http.DefaultClient).Accept(context.Background(), sampleEnvelope())
	var unexpected UnexpectedStatus
	if !errors.As(err, &unexpected) || unexpected.Status != http.StatusOK {
		t.Fatalf("want UnexpectedStatus 200, got %v", err)
	}
}

func TestMailInCreatedWithoutIdIsAnError(t *testing.T) {
	address := mailInServer(t, http.StatusCreated, `{}`, &mailInCall{})
	_, err := NewHttpMailIn(address, NewStaticTokens("t"), http.DefaultClient).Accept(context.Background(), sampleEnvelope())
	if err == nil || err.Error() != "the answer has no inboundId" {
		t.Fatalf("want MissingValue, got %v", err)
	}
}

func TestMailInCreatedWithBrokenJsonIsAnError(t *testing.T) {
	address := mailInServer(t, http.StatusCreated, `not json`, &mailInCall{})
	_, err := NewHttpMailIn(address, NewStaticTokens("t"), http.DefaultClient).Accept(context.Background(), sampleEnvelope())
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestMailInUnreachableIsAnError(t *testing.T) {
	_, err := NewHttpMailIn("http://127.0.0.1:1", NewStaticTokens("t"), http.DefaultClient).Accept(context.Background(), sampleEnvelope())
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestMailInTokenFailureIsAnError(t *testing.T) {
	_, err := NewHttpMailIn("http://127.0.0.1:1", failingTokens{}, http.DefaultClient).Accept(context.Background(), sampleEnvelope())
	if err == nil {
		t.Fatal("want an error")
	}
}
