package mailchecks

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"mesail.dev/edge/smtp/reception"
	"mesail.dev/packages/mailauth"
)

type fakeChecker struct {
	results mailauth.Results
	input   mailauth.Input
}

func (checker *fakeChecker) Check(_ context.Context, input mailauth.Input) mailauth.Results {
	checker.input = input
	return checker.results
}

type panickingChecker struct{}

func (panickingChecker) Check(_ context.Context, _ mailauth.Input) mailauth.Results {
	panic("resolver broke")
}

func TestVerdictCarriesTheResultsAndTheHeaderField(t *testing.T) {
	checker := &fakeChecker{results: mailauth.Results{Spf: "pass", SpfDomain: "outside.test", Dkim: []mailauth.DkimResult{{Result: "pass", Domain: "outside.test", Selector: "s1"}}, Dmarc: "pass", DmarcPolicy: "none", FromDomain: "outside.test"}}
	input := reception.AuthInput{RemoteIp: net.ParseIP("192.0.2.7"), Helo: "mail.outside.test", MailFrom: "dan@outside.test", Raw: []byte("x"), ReceivedAt: time.Unix(1, 0)}
	verdict := NewMailAuthChecker(checker, "mx.mesail.test").Check(context.Background(), input)
	if verdict.Results.Spf != "pass" || verdict.Results.Dkim != "pass" || verdict.Results.Dmarc != "pass" || verdict.Reject || verdict.Results.Suspicious {
		t.Fatalf("unexpected verdict %+v", verdict)
	}
	if !strings.HasPrefix(verdict.HeaderField, "Authentication-Results: mx.mesail.test") {
		t.Fatalf("unexpected header field %q", verdict.HeaderField)
	}
	if checker.input.Helo != "mail.outside.test" || checker.input.MailFrom != "dan@outside.test" || !checker.input.RemoteIp.Equal(input.RemoteIp) {
		t.Fatalf("unexpected input %+v", checker.input)
	}
}

func TestRejectAndSuspiciousArePassedOn(t *testing.T) {
	checker := &fakeChecker{results: mailauth.Results{Spf: "fail", Dmarc: "fail", DmarcPolicy: "reject", Reject: true}}
	verdict := NewMailAuthChecker(checker, "mx.mesail.test").Check(context.Background(), reception.AuthInput{})
	if !verdict.Reject || verdict.Results.DmarcPolicy != "reject" {
		t.Fatalf("unexpected verdict %+v", verdict)
	}
}

func TestAPanicInTheCheckNeverFailsDelivery(t *testing.T) {
	verdict := NewMailAuthChecker(panickingChecker{}, "mx.mesail.test").Check(context.Background(), reception.AuthInput{})
	if verdict.Reject || verdict.Results.Spf != "none" {
		t.Fatalf("unexpected verdict %+v", verdict)
	}
}
