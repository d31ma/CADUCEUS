package reception

import (
	"context"
	"errors"
	"net"
	"regexp"
	"strings"
	"sync"
	"testing"
)

type fakeChecker struct {
	verdict AuthVerdict
	inputs  []AuthInput
}

func (checker *fakeChecker) Check(_ context.Context, input AuthInput) AuthVerdict {
	checker.inputs = append(checker.inputs, input)
	return checker.verdict
}

type fakeHeaders struct {
	headers InboundHeaders
	err     error
}

func (reader fakeHeaders) Headers(_ []byte) (InboundHeaders, error) {
	return reader.headers, reader.err
}

type memoryStore struct {
	mutex   sync.Mutex
	content map[string][]byte
	err     error
}

func (store *memoryStore) Put(_ context.Context, key string, content []byte) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.err != nil {
		return store.err
	}
	store.content[key] = append([]byte{}, content...)
	return nil
}

type fakeMailIn struct {
	envelopes []InboundEnvelope
	err       error
}

func (mailIn *fakeMailIn) Accept(_ context.Context, envelope InboundEnvelope) (string, error) {
	mailIn.envelopes = append(mailIn.envelopes, envelope)
	if mailIn.err != nil {
		return "", mailIn.err
	}
	return "in_" + envelope.Recipients[0], nil
}

type receptionParts struct {
	checker *fakeChecker
	store   *memoryStore
	mailIn  *fakeMailIn
	headers fakeHeaders
}

func newParts() *receptionParts {
	return &receptionParts{
		checker: &fakeChecker{verdict: AuthVerdict{Results: AuthResults{Spf: "pass", Dkim: "pass", Dmarc: "pass", DmarcPolicy: "none"}, HeaderField: "Authentication-Results: mx.test; spf=pass\r\n"}},
		store:   &memoryStore{content: map[string][]byte{}},
		mailIn:  &fakeMailIn{},
		headers: fakeHeaders{headers: InboundHeaders{MessageId: "m-1@outside.test", Subject: "Hello", References: []string{}, To: []MailboxAddress{}, Cc: []MailboxAddress{}}},
	}
}

func (parts *receptionParts) reception() Reception {
	return NewReception(Parts{Checker: parts.checker, Headers: parts.headers, Store: parts.store, MailIn: parts.mailIn})
}

func sampleTransaction() Transaction {
	return Transaction{
		EnvelopeFrom:  "dan@outside.test",
		Recipients:    []Recipient{{Address: "bob@example.com", DomainId: "example.com"}, {Address: "cy@example.org", DomainId: "example.org"}, {Address: "ana@example.com", DomainId: "example.com"}},
		Raw:           []byte("From: dan@outside.test\r\nSubject: Hello\r\n\r\nHi\r\n"),
		RemoteAddress: "192.0.2.7:4567",
		Helo:          "mail.outside.test",
		Tls:           true,
	}
}

func TestTwoRecipientDomainsGiveTwoRawCopiesAndTwoMailInCalls(t *testing.T) {
	parts := newParts()
	receipt, err := parts.reception().Deliver(context.Background(), sampleTransaction())
	if err != nil {
		t.Fatal(err)
	}
	if len(parts.store.content) != 2 || len(parts.mailIn.envelopes) != 2 {
		t.Fatalf("want 2 copies and 2 calls, got %d and %d", len(parts.store.content), len(parts.mailIn.envelopes))
	}
	first, second := parts.mailIn.envelopes[0], parts.mailIn.envelopes[1]
	if strings.Join(first.Recipients, ",") != "ana@example.com,bob@example.com" || strings.Join(second.Recipients, ",") != "cy@example.org" {
		t.Fatalf("recipients are not split by domain: %v and %v", first.Recipients, second.Recipients)
	}
	if !regexp.MustCompile(`^example\.com/inbound-[0-9a-f]{64}$`).MatchString(first.RawBlobKey) {
		t.Fatalf("unexpected key %q", first.RawBlobKey)
	}
	if strings.Join(receipt.InboundIds, ",") != "in_ana@example.com,in_cy@example.org" {
		t.Fatalf("unexpected receipt %v", receipt.InboundIds)
	}
}

func TestStoredCopyStartsWithTheAuthenticationResultsField(t *testing.T) {
	parts := newParts()
	transaction := sampleTransaction()
	parts.reception().Deliver(context.Background(), transaction)
	envelope := parts.mailIn.envelopes[0]
	stored := string(parts.store.content[envelope.RawBlobKey])
	if stored != parts.checker.verdict.HeaderField+string(transaction.Raw) {
		t.Fatalf("unexpected stored copy %q", stored)
	}
	if envelope.Size != int64(len(stored)) {
		t.Fatalf("size %d does not match stored copy %d", envelope.Size, len(stored))
	}
}

func TestEnvelopeCarriesTransactionHeadersAndAuthResults(t *testing.T) {
	parts := newParts()
	parts.reception().Deliver(context.Background(), sampleTransaction())
	envelope := parts.mailIn.envelopes[0]
	if envelope.EnvelopeFrom != "dan@outside.test" || envelope.Helo != "mail.outside.test" || !envelope.Tls || envelope.RemoteAddress != "192.0.2.7:4567" {
		t.Fatalf("unexpected envelope %+v", envelope)
	}
	if envelope.Headers.MessageId != "m-1@outside.test" || envelope.AuthResults.Dmarc != "pass" {
		t.Fatalf("unexpected headers or results %+v", envelope)
	}
}

func TestCheckerReceivesTheRemoteIpWithoutPort(t *testing.T) {
	parts := newParts()
	parts.reception().Deliver(context.Background(), sampleTransaction())
	if !parts.checker.inputs[0].RemoteIp.Equal(net.ParseIP("192.0.2.7")) {
		t.Fatalf("unexpected ip %v", parts.checker.inputs[0].RemoteIp)
	}
}

func TestRemoteAddressWithoutPortStillGivesTheIp(t *testing.T) {
	parts := newParts()
	transaction := sampleTransaction()
	transaction.RemoteAddress = "2001:db8::1"
	parts.reception().Deliver(context.Background(), transaction)
	if !parts.checker.inputs[0].RemoteIp.Equal(net.ParseIP("2001:db8::1")) {
		t.Fatalf("unexpected ip %v", parts.checker.inputs[0].RemoteIp)
	}
}

func TestDmarcRejectStoresNothing(t *testing.T) {
	parts := newParts()
	parts.checker.verdict.Reject = true
	_, err := parts.reception().Deliver(context.Background(), sampleTransaction())
	if !errors.Is(err, Refused{}) {
		t.Fatalf("want Refused, got %v", err)
	}
	if len(parts.store.content) != 0 || len(parts.mailIn.envelopes) != 0 {
		t.Fatal("a rejected message was stored or announced")
	}
}

func TestMailInFailureFailsTheDelivery(t *testing.T) {
	parts := newParts()
	parts.mailIn.err = errors.New("mail-in answered HTTP 503")
	_, err := parts.reception().Deliver(context.Background(), sampleTransaction())
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestStoreFailureSkipsMailIn(t *testing.T) {
	parts := newParts()
	parts.store.err = errors.New("disk full")
	_, err := parts.reception().Deliver(context.Background(), sampleTransaction())
	if err == nil || len(parts.mailIn.envelopes) != 0 {
		t.Fatalf("want an error and no mail-in call, got %v and %d calls", err, len(parts.mailIn.envelopes))
	}
}

func TestRetryGivesTheSameRawKey(t *testing.T) {
	first, second := newParts(), newParts()
	first.reception().Deliver(context.Background(), sampleTransaction())
	retried := sampleTransaction()
	retried.Recipients = []Recipient{retried.Recipients[2], retried.Recipients[1], retried.Recipients[0]}
	second.checker.verdict.HeaderField = "Authentication-Results: mx.test; spf=temperror\r\n"
	second.reception().Deliver(context.Background(), retried)
	if first.mailIn.envelopes[0].RawBlobKey != second.mailIn.envelopes[0].RawBlobKey {
		t.Fatal("a retried transaction got a different key")
	}
}

func TestDifferentSenderGivesADifferentRawKey(t *testing.T) {
	group := DomainGroup{DomainId: "example.com", Recipients: []string{"ana@example.com"}}
	raw := []byte("Subject: x\r\n\r\nbody\r\n")
	if (RawKey{}).For(raw, "a@outside.test", group) == (RawKey{}).For(raw, "b@outside.test", group) {
		t.Fatal("the key ignores the envelope sender")
	}
}

func TestHeaderParseFailureStillDeliversWithEmptyLists(t *testing.T) {
	parts := newParts()
	parts.headers = fakeHeaders{err: errors.New("malformed")}
	_, err := parts.reception().Deliver(context.Background(), sampleTransaction())
	if err != nil {
		t.Fatal(err)
	}
	headers := parts.mailIn.envelopes[0].Headers
	if headers.References == nil || headers.To == nil || headers.Cc == nil {
		t.Fatalf("lists must not be null: %+v", headers)
	}
}

func TestSkippedChecksSayNoneAndNeverReject(t *testing.T) {
	verdict := SkippedChecks{}.Check(context.Background(), AuthInput{})
	if verdict.Reject || verdict.HeaderField != "" || verdict.Results.Spf != "none" || verdict.Results.Dmarc != "none" {
		t.Fatalf("unexpected verdict %+v", verdict)
	}
}

func TestErrorTextsAreReadable(t *testing.T) {
	texts := Refused{}.Error() + "|" + UnknownAddress{}.Error() + "|" + DomainNotHosted{}.Error()
	if strings.Contains(texts, "||") || strings.HasPrefix(texts, "|") || strings.HasSuffix(texts, "|") {
		t.Fatalf("an error has no text: %q", texts)
	}
}
