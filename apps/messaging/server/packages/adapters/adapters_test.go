package adapters

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mesail.dev/packages/core"
	"mesail.dev/packages/events"
	"mesail.dev/packages/outbox"
)

type fixedTokens struct {
	realms  []string
	failure error
}

func (tokens *fixedTokens) Authorization(realm string) (string, error) {
	tokens.realms = append(tokens.realms, realm)
	return "Bearer token-for-" + realm, tokens.failure
}

type seenRequest struct {
	method        string
	path          string
	authorization string
	body          string
}

func serve(t *testing.T, status int, answer string) (string, *seenRequest) {
	t.Helper()
	seen := &seenRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		*seen = seenRequest{method: request.Method, path: request.URL.EscapedPath(), authorization: request.Header.Get("Authorization"), body: string(body)}
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	return server.URL, seen
}

func call() JsonCall {
	return NewJsonCall(JsonCall{}.DefaultTimeout())
}

func isUnavailable(err error) bool {
	var failure *core.Unavailable
	return errors.As(err, &failure)
}

func isNotFound(err error) bool {
	var failure *core.NotFound
	return errors.As(err, &failure)
}

func TestDefaultTimeoutIsThirtySeconds(t *testing.T) {
	if (JsonCall{}).DefaultTimeout() != 30*time.Second {
		t.Fatal("want 30 s")
	}
}

func TestResolveForwardsTheCallersTokenAndDecodesRecipients(t *testing.T) {
	base, seen := serve(t, http.StatusOK, `{"recipients":[{"address":"ana@example.com","kind":"member","memberId":"m1","displayName":"Ana"}]}`)
	tokens := &fixedTokens{}
	resolved, err := NewDirectoryClient(base+"/", tokens, call()).Resolve("Bearer caller", []string{"ana@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 1 || resolved[0].MemberId != "m1" || resolved[0].Kind != "member" {
		t.Fatalf("got %+v", resolved)
	}
	if seen.method != http.MethodPost || seen.path != "/api/v1/directory/recipients/resolve" || seen.authorization != "Bearer caller" {
		t.Fatalf("got %+v", seen)
	}
	if seen.body != `{"addresses":["ana@example.com"]}` {
		t.Fatalf("got body %s", seen.body)
	}
	if len(tokens.realms) != 0 {
		t.Fatal("resolve must not fetch a service token")
	}
}

func TestResolveFailureIsUnavailable(t *testing.T) {
	base, _ := serve(t, http.StatusBadGateway, `{}`)
	_, err := NewDirectoryClient(base, &fixedTokens{}, call()).Resolve("Bearer caller", []string{"a@b.c"})
	if !isUnavailable(err) || err.Error() != "the service answered 502" {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestResolveNotFoundIsUnavailable(t *testing.T) {
	base, _ := serve(t, http.StatusNotFound, `{}`)
	_, err := NewDirectoryClient(base, &fixedTokens{}, call()).Resolve("Bearer caller", []string{"a@b.c"})
	if !isUnavailable(err) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestResolveOfABadAnswerIsUnavailable(t *testing.T) {
	base, _ := serve(t, http.StatusOK, `not json`)
	_, err := NewDirectoryClient(base, &fixedTokens{}, call()).Resolve("Bearer caller", []string{"a@b.c"})
	if !isUnavailable(err) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestResolveWhenTheDirectoryIsDownIsUnavailable(t *testing.T) {
	_, err := NewDirectoryClient("http://127.0.0.1:1", &fixedTokens{}, call()).Resolve("Bearer caller", []string{"a@b.c"})
	if !isUnavailable(err) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestResolveOfABadUrlIsUnavailable(t *testing.T) {
	_, err := NewDirectoryClient("http://bad host", &fixedTokens{}, call()).Resolve("Bearer caller", []string{"a@b.c"})
	if !isUnavailable(err) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestResolveOfAnUnencodableBodyIsUnavailable(t *testing.T) {
	_, err := call().Send(JsonRequest{Method: http.MethodPost, Url: "http://127.0.0.1:1", Body: func() {}}, &struct{}{})
	if !isUnavailable(err) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestAddressUsesAServiceTokenOfTheDomainAndEscapesThePath(t *testing.T) {
	base, seen := serve(t, http.StatusOK, `{"address":"a b@example.com","domainId":"example.com","kind":"member","targetId":"m1"}`)
	tokens := &fixedTokens{}
	entry, found, err := NewDirectoryClient(base, tokens, call()).Address("example.com", "a b@example.com")
	if err != nil || !found || entry.TargetId != "m1" {
		t.Fatalf("got %+v %v %v", entry, found, err)
	}
	if seen.method != http.MethodGet || seen.path != "/api/v1/directory/addresses/a%20b@example.com" || seen.authorization != "Bearer token-for-example.com" {
		t.Fatalf("got %+v", seen)
	}
	if seen.body != "" {
		t.Fatalf("a GET has no body, got %q", seen.body)
	}
}

func TestAddressNotFoundIsNotFound(t *testing.T) {
	base, _ := serve(t, http.StatusNotFound, `{"code":"not_found"}`)
	_, found, err := NewDirectoryClient(base, &fixedTokens{}, call()).Address("example.com", "nobody@example.com")
	if err != nil || found {
		t.Fatalf("want not found, got %v %v", found, err)
	}
}

func TestAddressOfAnotherDomainIsNotFound(t *testing.T) {
	base, _ := serve(t, http.StatusOK, `{"address":"a@other.test","domainId":"other.test","kind":"member","targetId":"m1"}`)
	_, found, err := NewDirectoryClient(base, &fixedTokens{}, call()).Address("example.com", "a@other.test")
	if err != nil || found {
		t.Fatalf("want not found, got %v %v", found, err)
	}
}

func TestAddressWithoutAServiceTokenIsUnavailable(t *testing.T) {
	_, _, err := NewDirectoryClient("http://127.0.0.1:1", &fixedTokens{failure: errors.New("no keycloak")}, call()).Address("example.com", "a@example.com")
	if !isUnavailable(err) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestAddressFailureIsUnavailable(t *testing.T) {
	base, _ := serve(t, http.StatusInternalServerError, `{}`)
	_, _, err := NewDirectoryClient(base, &fixedTokens{}, call()).Address("example.com", "a@example.com")
	if !isUnavailable(err) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestContentReadsTheInboundMessageWithAServiceToken(t *testing.T) {
	base, seen := serve(t, http.StatusOK, `{"text":"hi","bodyFormat":"text","blocks":[{"kind":"paragraph","spans":[{"text":"hi"}]}],"truncated":false,"attachments":[]}`)
	content, err := NewMailInClient(base, &fixedTokens{}, call()).Content("example.com", "in_1")
	if err != nil || content.Text != "hi" || len(content.Blocks) != 1 {
		t.Fatalf("got %+v %v", content, err)
	}
	if seen.path != "/api/v1/mail-in/messages/in_1/content" || seen.authorization != "Bearer token-for-example.com" {
		t.Fatalf("got %+v", seen)
	}
}

func TestOriginalReadsTheDownload(t *testing.T) {
	base, seen := serve(t, http.StatusOK, `{"url":"https://blobs/x.eml","expiresAt":"2026-11-10T12:00:00Z"}`)
	download, err := NewMailInClient(base, &fixedTokens{}, call()).Original("example.com", "in_1")
	if err != nil || download.Url != "https://blobs/x.eml" {
		t.Fatalf("got %+v %v", download, err)
	}
	if seen.path != "/api/v1/mail-in/messages/in_1/original" {
		t.Fatalf("got %+v", seen)
	}
}

func TestMissingInboundMessageIsNotFound(t *testing.T) {
	base, _ := serve(t, http.StatusNotFound, `{}`)
	_, err := NewMailInClient(base, &fixedTokens{}, call()).Original("example.com", "in_1")
	if !isNotFound(err) {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestMailInFailureIsUnavailable(t *testing.T) {
	base, _ := serve(t, http.StatusServiceUnavailable, `{}`)
	_, err := NewMailInClient(base, &fixedTokens{}, call()).Content("example.com", "in_1")
	if !isUnavailable(err) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestMailInWithoutAServiceTokenIsUnavailable(t *testing.T) {
	_, err := NewMailInClient("http://127.0.0.1:1", &fixedTokens{failure: errors.New("no keycloak")}, call()).Content("example.com", "in_1")
	if !isUnavailable(err) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestDomainPusherUsesATokenOfTheEnvelopeDomain(t *testing.T) {
	base, seen := serve(t, http.StatusOK, `{}`)
	tokens := &fixedTokens{}
	envelope := events.Envelope{EventID: "evt_1", Type: "message.created", DomainId: "example.com", Payload: json.RawMessage(`{}`)}
	err := NewDomainPusher(tokens, time.Second).Push(outbox.Subscriber{Name: "search", URL: base + "/api/v1/search/events"}, envelope)
	if err != nil {
		t.Fatal(err)
	}
	if seen.authorization != "Bearer token-for-example.com" || seen.path != "/api/v1/search/events" || !strings.Contains(seen.body, "evt_1") {
		t.Fatalf("got %+v", seen)
	}
}

func TestDomainPusherWithoutATokenFails(t *testing.T) {
	failure := errors.New("no keycloak")
	err := NewDomainPusher(&fixedTokens{failure: failure}, time.Second).Push(outbox.Subscriber{Name: "search", URL: "http://127.0.0.1:1"}, events.Envelope{DomainId: "example.com"})
	if !errors.Is(err, failure) {
		t.Fatalf("want the token failure, got %v", err)
	}
}

func TestDomainPusherReportsARefusal(t *testing.T) {
	base, _ := serve(t, http.StatusBadRequest, `{}`)
	err := NewDomainPusher(&fixedTokens{}, time.Second).Push(outbox.Subscriber{Name: "search", URL: base}, events.Envelope{DomainId: "example.com"})
	var failure outbox.DeliveryFailure
	if !errors.As(err, &failure) || !failure.Permanent {
		t.Fatalf("want a permanent DeliveryFailure, got %v", err)
	}
}

func TestRelayTriggerPostsWithAnOperatorToken(t *testing.T) {
	base, seen := serve(t, http.StatusOK, `{}`)
	tokens := &fixedTokens{}
	if err := NewRelayTrigger(tokens, time.Second).Run(base + "/api/v1/messaging/outbox/relay"); err != nil {
		t.Fatal(err)
	}
	if seen.method != http.MethodPost || seen.path != "/api/v1/messaging/outbox/relay" || seen.authorization != "Bearer token-for-mesail-operators" {
		t.Fatalf("got %+v", seen)
	}
}

func TestRelayTriggerReportsARefusal(t *testing.T) {
	base, _ := serve(t, http.StatusForbidden, `{}`)
	err := NewRelayTrigger(&fixedTokens{}, time.Second).Run(base)
	if !isUnavailable(err) || err.Error() != "the outbox relay route answered 403" {
		t.Fatalf("got %v", err)
	}
}

func TestRelayTriggerFailures(t *testing.T) {
	failure := errors.New("no keycloak")
	if !errors.Is(NewRelayTrigger(&fixedTokens{failure: failure}, time.Second).Run("http://127.0.0.1:1"), failure) {
		t.Fatal("want the token failure")
	}
	if NewRelayTrigger(&fixedTokens{}, time.Second).Run("http://bad host/relay") == nil {
		t.Fatal("want a bad url failure")
	}
	if NewRelayTrigger(&fixedTokens{}, time.Second).Run("http://127.0.0.1:1/relay") == nil {
		t.Fatal("want a connection failure")
	}
}

func TestScheduledSweepReadsTheRelayUrl(t *testing.T) {
	if (ScheduledSweep{}).RelayUrl(`{"yon":{"messaging":{"relayurl":"http://x/relay"}}}`) != "http://x/relay" {
		t.Fatal("want the relay url")
	}
	if (ScheduledSweep{}).RelayUrl(`not json`) != "" {
		t.Fatal("want no url")
	}
}

func TestScheduledSweepFetchesAnOperatorTokenAndCallsTheRoute(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	tokenPath := ""
	keycloak := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		tokenPath = request.URL.Path
		_, _ = writer.Write([]byte(`{"access_token":"operator-token","expires_in":60}`))
	}))
	t.Cleanup(keycloak.Close)
	relay, seen := serve(t, http.StatusOK, `{}`)
	t.Setenv("YON_APPLICATION_JSON", `{"yon":{"oidc":{"keycloak":"`+keycloak.URL+`/idp/realms/{realm}/protocol/openid-connect"},"service":{"client":{"secret":"s"}},"messaging":{"relayurl":"`+relay+`/relay"}}}`)
	if err := (ScheduledSweep{}).Run(); err != nil {
		t.Fatal(err)
	}
	if tokenPath != "/idp/realms/mesail-operators/protocol/openid-connect/token" || seen.authorization != "Bearer operator-token" {
		t.Fatalf("got %q %+v", tokenPath, seen)
	}
}

func TestRichTextRendererMakesBlocksFromMarkdown(t *testing.T) {
	text, err := RichTextRenderer{}.Markdown("# Title\n\nSome **bold** and [a link](https://example.com)")
	if err != nil || len(text.Blocks) != 2 || text.Truncated {
		t.Fatalf("got %+v %v", text, err)
	}
	if text.Blocks[0].Kind != "heading" || text.Blocks[0].Level != 1 {
		t.Fatalf("first block %+v", text.Blocks[0])
	}
	if !hasBoldSpan(text.Blocks[1].Spans) || !hasLink(text.Blocks[1].Spans, "https://example.com") {
		t.Fatalf("second block %+v", text.Blocks[1])
	}
}

func TestRichTextRendererDropsUnsafeLinks(t *testing.T) {
	text, _ := RichTextRenderer{}.Markdown("[click](javascript:alert(1))")
	if hasLink(text.Blocks[0].Spans, "javascript:alert(1)") {
		t.Fatalf("an unsafe link was kept: %+v", text.Blocks)
	}
}

func TestRichTextRendererOfAnEmptyBodyHasNoBlocks(t *testing.T) {
	text, err := RichTextRenderer{}.Markdown("  \n\n ")
	if err != nil || len(text.Blocks) != 0 {
		t.Fatalf("got %+v %v", text, err)
	}
}

func hasBoldSpan(spans []core.Span) bool {
	for _, span := range spans {
		if span.Bold && span.Text == "bold" {
			return true
		}
	}
	return false
}

func hasLink(spans []core.Span, href string) bool {
	for _, span := range spans {
		if span.Href == href {
			return true
		}
	}
	return false
}

func TestDescribeReadsTheMetaWithAServiceTokenOfTheDomain(t *testing.T) {
	base, seen := serve(t, http.StatusOK, `{"attachmentId":"att_1","fileName":"plan.pdf","mediaType":"application/pdf","size":2048,"sha256":"ab","state":"ready"}`)
	tokens := &fixedTokens{}
	info, found, err := NewAttachmentsClient(base+"/", tokens, call()).Describe("example.com", "att 1")
	if err != nil || !found {
		t.Fatalf("got %v %v", found, err)
	}
	want := core.AttachmentInfo{AttachmentId: "att_1", FileName: "plan.pdf", MediaType: "application/pdf", Size: 2048, State: "ready"}
	if info != want {
		t.Fatalf("got %+v", info)
	}
	if seen.method != http.MethodGet || seen.path != "/api/v1/attachments/att%201/meta" || seen.authorization != "Bearer token-for-example.com" || seen.body != "" {
		t.Fatalf("got %+v", seen)
	}
}

func TestDescribeOfAnUnknownAttachmentIsNotFound(t *testing.T) {
	base, _ := serve(t, http.StatusNotFound, `{"code":"not_found"}`)
	_, found, err := NewAttachmentsClient(base, &fixedTokens{}, call()).Describe("example.com", "att_1")
	if err != nil || found {
		t.Fatalf("want not found, got %v %v", found, err)
	}
}

func TestDescribeFailureIsUnavailable(t *testing.T) {
	base, _ := serve(t, http.StatusForbidden, `{"code":"forbidden"}`)
	_, found, err := NewAttachmentsClient(base, &fixedTokens{}, call()).Describe("example.com", "att_1")
	if !isUnavailable(err) || found {
		t.Fatalf("want Unavailable, got %v %v", found, err)
	}
}

func TestDescribeWhenTheServiceIsDownIsUnavailable(t *testing.T) {
	_, _, err := NewAttachmentsClient("http://127.0.0.1:1", &fixedTokens{}, call()).Describe("example.com", "att_1")
	if !isUnavailable(err) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}

func TestDescribeWithoutAServiceTokenIsUnavailable(t *testing.T) {
	_, _, err := NewAttachmentsClient("http://127.0.0.1:1", &fixedTokens{failure: errors.New("no keycloak")}, call()).Describe("example.com", "att_1")
	if !isUnavailable(err) {
		t.Fatalf("want Unavailable, got %v", err)
	}
}
