package smtpclient

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/smtpclient/sink"
)

var testMessage = []byte("From: a@sender.test\r\nTo: b@example.test\r\nSubject: hi\r\n\r\nhello\r\n")

func directTo(t *testing.T, routes map[string]string, records map[string][]*net.MX) (DirectDelivery, *routingDialer) {
	t.Helper()
	dialer := &routingDialer{routes: routes}
	return DirectDelivery{Hosts: MxHosts{Resolver: fakeResolver{records: records}}, Dialer: dialer, HeloName: "out.sender.test"}, dialer
}

func TestDirectDeliveryUsesOpportunisticStartTls(t *testing.T) {
	server := startSink(t, sink.Options{Tls: selfSigned(t).Server})
	delivery, _ := directTo(t, map[string]string{"mx.example.test:25": server.Address()}, map[string][]*net.MX{"example.test": {{Host: "mx.example.test.", Pref: 10}}})
	reply, err := delivery.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@Example.test"}}, testMessage)
	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if reply.Code != 250 || reply.Enhanced != "2.0.0" || reply.Text != "OK queued" {
		t.Fatalf("reply: %+v", reply)
	}
	got := server.Messages()
	if len(got) != 1 || !got[0].Tls || string(got[0].Data) != string(testMessage) || got[0].From != "a@sender.test" {
		t.Fatalf("messages: %+v", got)
	}
}

func TestDirectDeliveryWithoutStartTlsStaysPlain(t *testing.T) {
	server := startSink(t, sink.Options{})
	delivery, _ := directTo(t, map[string]string{"mx.example.test:2525": server.Address()}, map[string][]*net.MX{"example.test": {{Host: "mx.example.test", Pref: 10}}})
	delivery.Port = 2525
	if _, err := delivery.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if server.Messages()[0].Tls {
		t.Fatalf("want plain delivery")
	}
}

func TestDirectDeliveryFallsBackOnTemporaryReply(t *testing.T) {
	busy := startSink(t, sink.Options{RejectRecipient: rejectWith(451, "4.3.0 Try again later")})
	ready := startSink(t, sink.Options{})
	routes := map[string]string{"mx1.example.test:25": busy.Address(), "mx2.example.test:25": ready.Address()}
	records := map[string][]*net.MX{"example.test": {{Host: "mx2.example.test", Pref: 20}, {Host: "mx1.example.test", Pref: 10}}}
	delivery, dialer := directTo(t, routes, records)
	if _, err := delivery.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if strings.Join(dialer.dialed, ",") != "mx1.example.test:25,mx2.example.test:25" {
		t.Fatalf("dialed: %v", dialer.dialed)
	}
	if len(busy.Messages()) != 0 || len(ready.Messages()) != 1 {
		t.Fatalf("message must reach the second host only")
	}
}

func TestDirectDeliveryStopsOnPermanentReply(t *testing.T) {
	refusing := startSink(t, sink.Options{RejectRecipient: rejectWith(550, "5.1.1 User unknown")})
	ready := startSink(t, sink.Options{})
	routes := map[string]string{"mx1.example.test:25": refusing.Address(), "mx2.example.test:25": ready.Address()}
	records := map[string][]*net.MX{"example.test": {{Host: "mx1.example.test", Pref: 10}, {Host: "mx2.example.test", Pref: 20}}}
	delivery, dialer := directTo(t, routes, records)
	reply, err := delivery.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage)
	failure := asDeliveryError(t, err)
	if !failure.Permanent || failure.Host != "mx1.example.test" || reply.Code != 550 || reply.Enhanced != "5.1.1" || reply.Text != "User unknown" {
		t.Fatalf("failure: %+v reply %+v", failure, reply)
	}
	if len(dialer.dialed) != 1 || len(ready.Messages()) != 0 {
		t.Fatalf("second host must not be tried: %v", dialer.dialed)
	}
}

func TestDirectDeliveryReturnsLastTemporaryFailure(t *testing.T) {
	busy := startSink(t, sink.Options{RejectRecipient: rejectWith(450, "4.2.1 Mailbox busy")})
	routes := map[string]string{"mx1.example.test:25": closedAddress(t), "mx2.example.test:25": busy.Address()}
	records := map[string][]*net.MX{"example.test": {{Host: "mx1.example.test", Pref: 10}, {Host: "mx2.example.test", Pref: 20}}}
	delivery, _ := directTo(t, routes, records)
	reply, err := delivery.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage)
	failure := asDeliveryError(t, err)
	if failure.Permanent || !reply.Temporary() || reply.Code != 450 || reply.Enhanced != "4.2.1" || failure.Host != "mx2.example.test" {
		t.Fatalf("failure: %+v", failure)
	}
}

func TestDirectDeliveryUsesImplicitMx(t *testing.T) {
	server := startSink(t, sink.Options{})
	delivery, _ := directTo(t, map[string]string{"example.test:25": server.Address()}, nil)
	delivery.Hosts.Resolver = fakeResolver{err: &net.DNSError{Err: "no such host", Name: "example.test", IsNotFound: true}}
	if _, err := delivery.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if len(server.Messages()) != 1 {
		t.Fatalf("want delivery to the domain itself")
	}
}

func TestDirectDeliveryRefusesNullMx(t *testing.T) {
	delivery, dialer := directTo(t, nil, map[string][]*net.MX{"example.test": {{Host: ".", Pref: 0}}})
	reply, err := delivery.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage)
	failure := asDeliveryError(t, err)
	if !failure.Permanent || reply.Code != 556 || reply.Enhanced != "5.1.10" || failure.Host != "example.test" || len(dialer.dialed) != 0 {
		t.Fatalf("failure: %+v", failure)
	}
}

func TestDirectDeliveryLookupFailureIsTemporary(t *testing.T) {
	delivery, _ := directTo(t, nil, nil)
	delivery.Hosts.Resolver = fakeResolver{err: &net.DNSError{Err: "server misbehaving", Name: "example.test", IsTemporary: true}}
	reply, err := delivery.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage)
	failure := asDeliveryError(t, err)
	if failure.Permanent || !reply.Temporary() || !strings.Contains(failure.Error(), "server misbehaving") {
		t.Fatalf("failure: %+v", failure)
	}
}

func TestDirectDeliveryDialFailureIsTemporary(t *testing.T) {
	delivery, _ := directTo(t, map[string]string{"mx.example.test:25": closedAddress(t)}, map[string][]*net.MX{"example.test": {{Host: "mx.example.test"}}})
	reply, err := delivery.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage)
	failure := asDeliveryError(t, err)
	if failure.Permanent || reply.Code != 0 || !reply.Temporary() || failure.Cause == nil {
		t.Fatalf("failure: %+v", failure)
	}
}

func TestDirectDeliveryNeedsOneDomain(t *testing.T) {
	delivery, _ := directTo(t, nil, nil)
	envelopes := []Envelope{
		{From: "a@sender.test", Recipients: []string{"b@example.test", "c@other.test"}},
		{From: "a@sender.test"},
	}
	for _, envelope := range envelopes {
		reply, err := delivery.Deliver(context.Background(), envelope, testMessage)
		if !asDeliveryError(t, err).Permanent || reply.Code != 554 {
			t.Fatalf("envelope %+v: want permanent 554, got %+v", envelope, reply)
		}
	}
}

func TestDirectDeliveryWithNoHostsIsTemporary(t *testing.T) {
	delivery, _ := directTo(t, nil, nil)
	reply, err := delivery.tryHosts(context.Background(), nil, Envelope{}, testMessage)
	if asDeliveryError(t, err).Permanent || !reply.Temporary() {
		t.Fatalf("want temporary failure")
	}
}

func TestDirectDeliveryUsesGivenTlsConfig(t *testing.T) {
	certificates := selfSigned(t)
	server := startSink(t, sink.Options{Tls: certificates.Server})
	delivery, _ := directTo(t, map[string]string{"127.0.0.1:25": server.Address()}, map[string][]*net.MX{"example.test": {{Host: "127.0.0.1"}}})
	delivery.Tls = certificates.Client
	if _, err := delivery.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage); err != nil {
		t.Fatalf("deliver: %v", err)
	}
}

func relayTo(server *sink.Sink, certificates sink.SelfSigned) RelayDelivery {
	return RelayDelivery{Address: server.Address(), HeloName: "out.sender.test", Username: "relay", Password: "secret", RequireTls: true, Tls: certificates.Client}
}

func TestRelayDeliveryAuthenticatesOverStartTls(t *testing.T) {
	certificates := selfSigned(t)
	server := startSink(t, sink.Options{Tls: certificates.Server, Username: "relay", Password: "secret"})
	reply, err := relayTo(server, certificates).Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test", "c@other.test"}}, testMessage)
	if err != nil {
		t.Fatalf("deliver: %v", err)
	}
	got := server.Messages()
	if reply.Code != 250 || len(got) != 1 || !got[0].Tls || got[0].AuthUser != "relay" || len(got[0].Recipients) != 2 {
		t.Fatalf("reply %+v messages %+v", reply, got)
	}
}

func TestRelayDeliveryWithImplicitTls(t *testing.T) {
	certificates := selfSigned(t)
	server := startSink(t, sink.Options{Tls: certificates.Server, ImplicitTls: true, Username: "relay", Password: "secret"})
	relay := relayTo(server, certificates)
	relay.ImplicitTls = true
	if _, err := relay.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if got := server.Messages(); len(got) != 1 || got[0].AuthUser != "relay" {
		t.Fatalf("messages: %+v", got)
	}
}

func TestRelayDeliveryRequiresOfferedStartTls(t *testing.T) {
	server := startSink(t, sink.Options{})
	relay := relayTo(server, selfSigned(t))
	reply, err := relay.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage)
	failure := asDeliveryError(t, err)
	if failure.Permanent || !reply.Temporary() || !strings.Contains(failure.Error(), "STARTTLS") || len(server.Messages()) != 0 {
		t.Fatalf("failure: %+v", failure)
	}
}

func TestRelayDeliveryNeverSendsCredentialsWithoutTls(t *testing.T) {
	server := startSink(t, sink.Options{Username: "relay", Password: "secret"})
	relay := relayTo(server, selfSigned(t))
	relay.RequireTls = false
	_, err := relay.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage)
	failure := asDeliveryError(t, err)
	if failure.Permanent || !strings.Contains(failure.Error(), "without TLS") {
		t.Fatalf("failure: %+v", failure)
	}
}

func TestRelayDeliveryWrongPasswordIsPermanent(t *testing.T) {
	certificates := selfSigned(t)
	server := startSink(t, sink.Options{Tls: certificates.Server, Username: "relay", Password: "secret"})
	relay := relayTo(server, certificates)
	relay.Password = "wrong"
	reply, err := relay.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage)
	if !asDeliveryError(t, err).Permanent || reply.Code != 535 || reply.Enhanced != "5.7.8" {
		t.Fatalf("reply: %+v", reply)
	}
}

func TestRelayDeliveryStopsAtRejectedRecipient(t *testing.T) {
	reject := func(address string) (int, string) {
		if address == "gone@example.test" {
			return 550, "5.1.1 No such user"
		}
		return 0, ""
	}
	server := startSink(t, sink.Options{RejectRecipient: reject})
	relay := RelayDelivery{Address: server.Address()}
	reply, err := relay.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"ok@example.test", "gone@example.test"}}, testMessage)
	if !asDeliveryError(t, err).Permanent || reply.Code != 550 || len(server.Messages()) != 0 {
		t.Fatalf("reply: %+v", reply)
	}
}

func TestRelayDeliveryDataRejectionIsPermanent(t *testing.T) {
	server := startSink(t, sink.Options{MaxSize: 8})
	reply, err := RelayDelivery{Address: server.Address()}.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage)
	if !asDeliveryError(t, err).Permanent || reply.Code != 552 || reply.Enhanced != "5.3.4" {
		t.Fatalf("reply: %+v", reply)
	}
}

func TestRelayDeliveryStopsAtContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	reply, err := RelayDelivery{Address: silentListener(t)}.Deliver(ctx, Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage)
	failure := asDeliveryError(t, err)
	if failure.Permanent || !reply.Temporary() || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("failure: %+v", failure)
	}
}

func TestRelayDeliveryThroughProxy(t *testing.T) {
	server := startSink(t, sink.Options{})
	proxy := startProxy(t, http.StatusOK)
	relay := RelayDelivery{Address: server.Address(), Dialer: ProxyDialer{ProxyUrl: proxy.url(url.UserPassword("guard", "token")), Timeout: time.Second}}
	if _, err := relay.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	targets, headers := proxy.seen()
	if len(targets) != 1 || targets[0] != "CONNECT "+server.Address() || headers[0] != "Basic Z3VhcmQ6dG9rZW4=" {
		t.Fatalf("proxy saw %v %v", targets, headers)
	}
	if len(server.Messages()) != 1 {
		t.Fatalf("message must arrive through the tunnel")
	}
}

func TestProxyRefusalIsTemporary(t *testing.T) {
	proxy := startProxy(t, http.StatusForbidden)
	relay := RelayDelivery{Address: "relay.example.test:587", Dialer: ProxyDialer{ProxyUrl: proxy.url(nil)}}
	reply, err := relay.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage)
	failure := asDeliveryError(t, err)
	if failure.Permanent || !reply.Temporary() || !strings.Contains(failure.Error(), "403") || failure.Host != "relay.example.test" {
		t.Fatalf("failure: %+v", failure)
	}
	if _, headers := proxy.seen(); headers[0] != "" {
		t.Fatalf("no credentials expected, got %q", headers[0])
	}
}

func TestProxyDialFailure(t *testing.T) {
	proxyUrl, _ := url.Parse("http://" + closedAddress(t))
	if _, err := (ProxyDialer{ProxyUrl: proxyUrl}).DialContext(context.Background(), "tcp", "relay.example.test:587"); err == nil {
		t.Fatalf("want dial failure")
	}
}

func TestProxyHandshakeStopsAtContextDeadline(t *testing.T) {
	proxyUrl, _ := url.Parse("http://" + silentListener(t))
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := (ProxyDialer{ProxyUrl: proxyUrl}).DialContext(ctx, "tcp", "relay.example.test:587"); err == nil {
		t.Fatalf("want deadline failure")
	}
}

func TestProxyAddressDefaultsToPort80(t *testing.T) {
	proxyUrl, _ := url.Parse("http://127.0.0.1")
	if got := (ProxyDialer{ProxyUrl: proxyUrl}).proxyAddress(); got != "127.0.0.1:80" {
		t.Fatalf("got %s", got)
	}
}

func TestDialersFromEnvironment(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("https_proxy", "")
	if dialer, err := (ProxyDialers{}).FromEnvironment(); err != nil || dialer != (DirectDialer{}) {
		t.Fatalf("want DirectDialer, got %#v %v", dialer, err)
	}
	t.Setenv("https_proxy", "127.0.0.1:3128")
	dialer, err := ProxyDialers{}.FromEnvironment()
	if proxy, found := dialer.(ProxyDialer); err != nil || !found || proxy.ProxyUrl.Host != "127.0.0.1:3128" {
		t.Fatalf("lowercase setting: %#v %v", dialer, err)
	}
	t.Setenv("HTTPS_PROXY", "http://guard.local:8080")
	dialer, _ = ProxyDialers{}.FromEnvironment()
	if dialer.(ProxyDialer).ProxyUrl.Host != "guard.local:8080" {
		t.Fatalf("uppercase setting must win: %#v", dialer)
	}
	t.Setenv("HTTPS_PROXY", "http://[::1")
	if _, err := (ProxyDialers{}).FromEnvironment(); err == nil {
		t.Fatalf("want parse error")
	}
}

func TestReplyClasses(t *testing.T) {
	cases := []struct {
		reply     Reply
		temporary bool
		permanent bool
	}{
		{Reply{Code: 0}, true, false},
		{Reply{Code: 250}, false, false},
		{Reply{Code: 421}, true, false},
		{Reply{Code: 554}, false, true},
	}
	for _, testCase := range cases {
		if testCase.reply.Temporary() != testCase.temporary || testCase.reply.Permanent() != testCase.permanent {
			t.Fatalf("%+v", testCase)
		}
	}
}

func TestDeliveryErrorText(t *testing.T) {
	failure := DeliveryError{Reply: Reply{Code: 550, Enhanced: "5.1.1", Text: "User unknown"}, Host: "mx.example.test", Permanent: true}
	if failure.Error() != "smtp delivery via mx.example.test failed: 550 5.1.1 User unknown" || failure.Unwrap() != nil {
		t.Fatalf("got %q", failure.Error())
	}
}

func TestEnhancedCodeParsing(t *testing.T) {
	cases := map[string]Reply{
		"5.1.1 User unknown":  {Code: 550, Enhanced: "5.1.1", Text: "User unknown"},
		"User unknown":        {Code: 550, Text: "User unknown"},
		"4.7.1":               {Code: 550, Enhanced: "4.7.1"},
		"5.7.1 Line\nsecond":  {Code: 550, Enhanced: "5.7.1", Text: "Line\nsecond"},
		"5.1.1000 not a code": {Code: 550, Text: "5.1.1000 not a code"},
	}
	for text, want := range cases {
		if got := (failures{}).parse(550, text); got != want {
			t.Fatalf("%q: got %+v", text, got)
		}
	}
}

func TestMxHostsWithEmptyAnswerUsesDomain(t *testing.T) {
	hosts, err := MxHosts{Resolver: fakeResolver{}}.ForDomain(context.Background(), "example.test")
	if err != nil || strings.Join(hosts, ",") != "example.test" {
		t.Fatalf("hosts %v err %v", hosts, err)
	}
}

func TestImplicitTlsHandshakeFailureIsTemporary(t *testing.T) {
	server := startSink(t, sink.Options{})
	relay := RelayDelivery{Address: server.Address(), ImplicitTls: true}
	reply, err := relay.Deliver(context.Background(), Envelope{From: "a@sender.test", Recipients: []string{"b@example.test"}}, testMessage)
	if asDeliveryError(t, err).Permanent || !reply.Temporary() || len(server.Messages()) != 0 {
		t.Fatalf("reply: %+v", reply)
	}
}
