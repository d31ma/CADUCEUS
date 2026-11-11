package webpush

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type capturedRequest struct {
	method  string
	path    string
	header  http.Header
	body    []byte
	written bool
}

func pushServer(t *testing.T, status int, answer string) (*httptest.Server, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		*captured = capturedRequest{method: request.Method, path: request.URL.Path, header: request.Header.Clone(), body: body, written: true}
		writer.Header().Set("Location", "https://push.example.com/elsewhere")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	return server, captured
}

func clientFor(server *httptest.Server) *http.Client {
	transport := server.Client().Transport.(*http.Transport).Clone()
	address := server.Listener.Addr().String()
	transport.DialContext = func(ctx context.Context, network string, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, address)
	}
	return &http.Client{Transport: transport}
}

func testTarget(client *http.Client, keys VapidKeys) WebPushTarget {
	policy := EndpointPolicy{Hosts: []string{"example.com"}}
	return NewWebPushTarget(NewVapidSigner(keys, fixedNow), NewEncryptor(rand.Reader), policy, client)
}

func testMessage() Message {
	return Message{Title: "Ana mentioned you", Body: "", Tag: "ntf_1", Url: "/c/1", NotificationId: "ntf_1", Kind: "mention", ConversationId: "cnv_1", MessageId: "msg_1"}
}

func TestSendPostsAnEncryptedSignedMessage(t *testing.T) {
	server, captured := pushServer(t, http.StatusCreated, "")
	subscriber := newTestSubscriber(t, "https://push.example.com/send/abc")
	outcome, err := testTarget(clientFor(server), testKeys()).Send(subscriber.subscription, testMessage())
	if err != nil || outcome != (Outcomes{}).Delivered() {
		t.Fatalf("outcome %q error %v", outcome, err)
	}
	if captured.method != http.MethodPost || captured.path != "/send/abc" {
		t.Fatalf("request %s %s", captured.method, captured.path)
	}
	expectedHeaders := map[string]string{
		"Content-Encoding": "aes128gcm",
		"Content-Type":     "application/octet-stream",
		"Ttl":              "86400",
		"Urgency":          "normal",
	}
	for name, value := range expectedHeaders {
		if captured.header.Get(name) != value {
			t.Fatalf("header %s = %q", name, captured.header.Get(name))
		}
	}
	authorization := captured.header.Get("Authorization")
	if !strings.HasPrefix(authorization, "vapid t=") || !strings.HasSuffix(authorization, ", k="+testPublicKey) {
		t.Fatalf("authorization %q", authorization)
	}
	decrypted := decryptForTest(t, captured.body, subscriber.privateKey, subscriber.authSecret)
	message := Message{}
	err = json.Unmarshal(decrypted, &message)
	if err != nil {
		t.Fatal(err)
	}
	if message != testMessage() {
		t.Fatalf("message %+v", message)
	}
}

func TestSendMapsStatusToOutcome(t *testing.T) {
	cases := []struct {
		status  int
		outcome string
		failure bool
	}{
		{http.StatusOK, Outcomes{}.Delivered(), false},
		{http.StatusNotFound, Outcomes{}.Gone(), false},
		{http.StatusGone, Outcomes{}.Gone(), false},
		{http.StatusTooManyRequests, Outcomes{}.Retry(), true},
		{http.StatusInternalServerError, Outcomes{}.Retry(), true},
		{http.StatusServiceUnavailable, Outcomes{}.Retry(), true},
		{http.StatusBadRequest, Outcomes{}.Refused(), true},
		{http.StatusForbidden, Outcomes{}.Refused(), true},
		{http.StatusRequestEntityTooLarge, Outcomes{}.Refused(), true},
		{http.StatusFound, Outcomes{}.Refused(), true},
	}
	for _, testCase := range cases {
		t.Run(http.StatusText(testCase.status), func(t *testing.T) {
			server, _ := pushServer(t, testCase.status, "  service says no  ")
			subscriber := newTestSubscriber(t, "https://push.example.com/send/abc")
			outcome, err := testTarget(clientFor(server), testKeys()).Send(subscriber.subscription, testMessage())
			if outcome != testCase.outcome {
				t.Fatalf("outcome %q", outcome)
			}
			failure := PushFailure{}
			if errors.As(err, &failure) != testCase.failure {
				t.Fatalf("error %v", err)
			}
			if testCase.failure && (failure.Status != testCase.status || failure.Detail != "service says no") {
				t.Fatalf("failure %+v", failure)
			}
		})
	}
}

func TestSendRetriesWhenTheServiceCannotBeReached(t *testing.T) {
	server, _ := pushServer(t, http.StatusCreated, "")
	client := clientFor(server)
	server.Close()
	subscriber := newTestSubscriber(t, "https://push.example.com/send/abc")
	outcome, err := testTarget(client, testKeys()).Send(subscriber.subscription, testMessage())
	failure := PushFailure{}
	if outcome != (Outcomes{}).Retry() || !errors.As(err, &failure) || failure.Status != 0 {
		t.Fatalf("outcome %q error %v", outcome, err)
	}
	if strings.Contains(failure.Detail, "/send/abc") {
		t.Fatalf("detail leaks the endpoint: %q", failure.Detail)
	}
}

func TestSendRefusesBeforeAnyRequest(t *testing.T) {
	server, captured := pushServer(t, http.StatusCreated, "")
	subscriber := newTestSubscriber(t, "https://push.example.com/send/abc")
	brokenKeys := testKeys()
	brokenKeys.PrivateKey = "%%%"
	brokenSubscription := subscriber.subscription
	brokenSubscription.Auth = "AAAA"
	outsideSubscription := subscriber.subscription
	outsideSubscription.Endpoint = "https://attacker.test/x"
	cases := map[string]struct {
		keys         VapidKeys
		subscription Subscription
		message      Message
	}{
		"endpoint outside the policy": {testKeys(), outsideSubscription, testMessage()},
		"broken vapid keys":           {brokenKeys, subscriber.subscription, testMessage()},
		"broken subscription keys":    {testKeys(), brokenSubscription, testMessage()},
		"message too large":           {testKeys(), subscriber.subscription, Message{Body: strings.Repeat("a", 5000)}},
	}
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			outcome, err := testTarget(clientFor(server), testCase.keys).Send(testCase.subscription, testCase.message)
			if outcome != (Outcomes{}).Refused() || err == nil {
				t.Fatalf("outcome %q error %v", outcome, err)
			}
			if captured.written {
				t.Fatal("a request reached the push service")
			}
		})
	}
}

func TestWebPushTargetIsATarget(t *testing.T) {
	var target Target = testTarget(nil, testKeys())
	if target == nil {
		t.Fatal("no target")
	}
}

func TestNilClientGetsAPlainClientWithoutRedirects(t *testing.T) {
	client := WebPushTarget{}.withoutRedirects(nil)
	if client == nil || client.CheckRedirect == nil {
		t.Fatal("client not prepared")
	}
	if client.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatal("redirects are followed")
	}
}

func TestCallerClientIsNotChanged(t *testing.T) {
	client := &http.Client{}
	_ = testTarget(client, testKeys())
	if client.CheckRedirect != nil {
		t.Fatal("caller client was changed")
	}
}

func TestResponseDetailReportsAReadFailure(t *testing.T) {
	detail := WebPushTarget{}.responseDetail(io.MultiReader(strings.NewReader("partial"), failingReader{}))
	if detail != "partial (the response body could not be read: reader failed)" {
		t.Fatalf("detail %q", detail)
	}
}

func TestResponseDetailIsLimited(t *testing.T) {
	detail := WebPushTarget{}.responseDetail(strings.NewReader(strings.Repeat("a", 2000)))
	if len(detail) != 512 {
		t.Fatalf("detail length %d", len(detail))
	}
}

func TestNetworkDetailWithoutAUrlError(t *testing.T) {
	if (WebPushTarget{}).networkDetail(errors.New("plain")) != "plain" {
		t.Fatal("plain error detail lost")
	}
}
