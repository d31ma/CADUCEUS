package carddav

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/d31ma/caduceus/internal/accounts"
)

func TestClientDiscoversPrincipalHomeAndAddressBooks(t *testing.T) {
	var calls []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls = append(calls, request.Method+" "+request.URL.Path+" depth="+request.Header.Get("Depth"))
		username, password, ok := request.BasicAuth()
		if !ok || username != "dav-user" || password != "top-secret" {
			response.Header().Set("WWW-Authenticate", `Basic realm="contacts"`)
			http.Error(response, "authentication required", http.StatusUnauthorized)
			return
		}
		body, _ := io.ReadAll(request.Body)
		switch request.URL.Path {
		case "/.well-known/carddav":
			if request.Method != "PROPFIND" || !strings.Contains(string(body), "current-user-principal") {
				t.Fatalf("bootstrap request = %s %q", request.Method, body)
			}
			multistatus(response, `<d:response><d:href>/.well-known/carddav</d:href><d:propstat><d:prop><d:current-user-principal><d:href>/principals/alice/</d:href></d:current-user-principal></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`)
		case "/principals/alice/":
			if !strings.Contains(string(body), "addressbook-home-set") {
				t.Fatalf("principal request body = %q", body)
			}
			multistatus(response, `<d:response><d:href>/principals/alice/</d:href><d:propstat><d:prop><card:addressbook-home-set><d:href>/dav/addressbooks/alice/</d:href></card:addressbook-home-set></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`)
		case "/dav/addressbooks/alice/":
			if request.Header.Get("Depth") != "1" || !strings.Contains(string(body), "sync-token") {
				t.Fatalf("home request headers=%v body=%q", request.Header, body)
			}
			multistatus(response, `<d:response><d:href>/dav/addressbooks/alice/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`+
				`<d:response><d:href>/dav/addressbooks/alice/default/</d:href><d:propstat><d:prop><d:displayname>People &amp; Teams</d:displayname><d:resourcetype><d:collection/><card:addressbook/></d:resourcetype><d:sync-token>https://dav.example/sync/7</d:sync-token><cs:getctag>ctag-7</cs:getctag></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`)
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	client := mustClient(t, Options{HTTPClient: server.Client()})
	discovery, err := client.Discover(context.Background(), &accounts.CardDAVEndpoint{URL: server.URL, Username: "dav-user", AllowInsecureTLS: true}, accounts.Credentials{Password: "top-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(discovery.PrincipalURL, "/principals/alice/") || !strings.HasSuffix(discovery.HomeURL, "/dav/addressbooks/alice/") {
		t.Fatalf("discovery = %#v", discovery)
	}
	if len(discovery.AddressBooks) != 1 || discovery.AddressBooks[0].Name != "People & Teams" || discovery.AddressBooks[0].SyncToken != "https://dav.example/sync/7" || discovery.AddressBooks[0].CTag != "ctag-7" {
		t.Fatalf("address books = %#v", discovery.AddressBooks)
	}
	wantCalls := []string{"PROPFIND /.well-known/carddav depth=0", "PROPFIND /principals/alice/ depth=0", "PROPFIND /dav/addressbooks/alice/ depth=1"}
	if fmt.Sprint(calls) != fmt.Sprint(wantCalls) {
		t.Fatalf("calls=%v want=%v", calls, wantCalls)
	}
}

func TestClientPerformsBoundedInitialAddressBookQuery(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if request.Method != "REPORT" || request.Header.Get("Depth") != "1" || !strings.Contains(string(body), "addressbook-query") || !strings.Contains(string(body), "address-data") {
			t.Fatalf("request=%s headers=%v body=%q", request.Method, request.Header, body)
		}
		multistatus(response, `<d:response><d:href>/dav/addressbooks/alice/default/a.vcf</d:href><d:propstat><d:prop><d:getetag>&quot;etag-a&quot;</d:getetag><card:address-data>BEGIN:VCARD&#13;&#10;VERSION:3.0&#13;&#10;UID:a-uid&#13;&#10;FN:Alice Example&#13;&#10;EMAIL;TYPE=work:alice@example.com&#13;&#10;END:VCARD&#13;&#10;</card:address-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`+
			`<d:response><d:href>/dav/addressbooks/alice/default/gone.vcf</d:href><d:propstat><d:prop/><d:status>HTTP/1.1 404 Not Found</d:status></d:propstat></d:response>`)
	}))
	defer server.Close()

	client := mustClient(t, Options{HTTPClient: server.Client(), MaxResponseBytes: 1 << 20})
	contacts, err := client.InitialSync(context.Background(), AddressBook{URL: server.URL + "/dav/addressbooks/alice/default/"}, Auth{Username: "alice", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if len(contacts) != 1 || contacts[0].Href != server.URL+"/dav/addressbooks/alice/default/a.vcf" || contacts[0].ETag != `"etag-a"` || !strings.Contains(contacts[0].VCard, "FN:Alice Example") {
		t.Fatalf("contacts = %#v", contacts)
	}
}

func TestClientUsesSyncTokenAndMultigetsOnlyChangedContacts(t *testing.T) {
	var calls []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		calls = append(calls, request.Method+" "+request.URL.Path+" "+string(body))
		if strings.Contains(string(body), "sync-collection") {
			if !strings.Contains(string(body), "sync/7") || strings.Contains(string(body), "a.vcf</d:href>") {
				t.Fatalf("sync request=%q", body)
			}
			response.Header().Set("Content-Type", "application/xml")
			response.WriteHeader(StatusMultiStatus)
			_, _ = response.Write([]byte(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:"><d:sync-token>https://dav.example/sync/8</d:sync-token><d:response><d:href>/books/default/a.vcf</d:href><d:propstat><d:prop><d:getetag>&quot;8&quot;</d:getetag></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response><d:response><d:href>/books/default/gone.vcf</d:href><d:status>HTTP/1.1 404 Not Found</d:status></d:response></d:multistatus>`))
			return
		}
		if !strings.Contains(string(body), "addressbook-multiget") || !strings.Contains(string(body), "/books/default/a.vcf") || strings.Contains(string(body), "gone.vcf") {
			t.Fatalf("multiget request=%q", body)
		}
		multistatus(response, `<d:response><d:href>/books/default/a.vcf</d:href><d:propstat><d:prop><d:getetag>&quot;8&quot;</d:getetag><card:address-data>BEGIN:VCARD&#13;&#10;FN:Alice Updated&#13;&#10;END:VCARD&#13;&#10;</card:address-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`)
	}))
	defer server.Close()
	client := mustClient(t, Options{HTTPClient: server.Client()})
	delta, err := client.IncrementalSync(context.Background(), AddressBook{URL: server.URL + "/books/default/"}, "https://dav.example/sync/7", Auth{Username: "alice", Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if delta.SyncToken != "https://dav.example/sync/8" || len(delta.Changed) != 1 || delta.Changed[0].ETag != `"8"` || len(delta.Deleted) != 1 || delta.Deleted[0] != server.URL+"/books/default/gone.vcf" || len(calls) != 2 {
		t.Fatalf("delta=%#v calls=%d", delta, len(calls))
	}
}

func TestClientClassifiesRejectedSyncTokenForFullRefreshFallback(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { response.WriteHeader(http.StatusForbidden) }))
	defer server.Close()
	client := mustClient(t, Options{HTTPClient: server.Client()})
	_, err := client.IncrementalSync(context.Background(), AddressBook{URL: server.URL + "/books/default/"}, "expired", Auth{Username: "alice", Password: "secret"})
	if err != ErrSyncTokenInvalid {
		t.Fatalf("error=%v", err)
	}
}

func TestClientUsesConditionalHeadersForUpdateCreateAndDelete(t *testing.T) {
	var calls []string
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		calls = append(calls, strings.Join([]string{request.Method, request.Header.Get("If-Match"), request.Header.Get("If-None-Match"), string(body)}, "|"))
		switch request.Method {
		case http.MethodPut:
			response.Header().Set("ETag", `"new"`)
			response.WriteHeader(http.StatusNoContent)
		case http.MethodDelete:
			response.WriteHeader(http.StatusNoContent)
		}
	}))
	defer server.Close()
	client := mustClient(t, Options{HTTPClient: server.Client()})
	auth := Auth{Username: "alice", Password: "secret"}
	if etag, err := client.Put(context.Background(), server.URL+"/books/a.vcf", "BEGIN:VCARD\r\nFN:Alice\r\nEND:VCARD\r\n", `"old"`, false, auth); err != nil || etag != `"new"` {
		t.Fatalf("update etag=%q err=%v", etag, err)
	}
	if _, err := client.Put(context.Background(), server.URL+"/books/new.vcf", "BEGIN:VCARD\r\nFN:New\r\nEND:VCARD\r\n", "", true, auth); err != nil {
		t.Fatal(err)
	}
	if err := client.Delete(context.Background(), server.URL+"/books/a.vcf", `"new"`, auth); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || !strings.HasPrefix(calls[0], `PUT|"old"||`) || !strings.HasPrefix(calls[1], "PUT||*|") || calls[2] != `DELETE|"new"||` {
		t.Fatalf("calls=%#v", calls)
	}
}

func TestClientFetchesOneBoundedVCardForCrashRecovery(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || !strings.Contains(request.Header.Get("Accept"), "text/vcard") {
			t.Fatalf("request=%s accept=%q", request.Method, request.Header.Get("Accept"))
		}
		response.Header().Set("Content-Type", "text/vcard")
		response.Header().Set("ETag", `"8"`)
		_, _ = response.Write([]byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:uid-a\r\nFN:Alice\r\nEND:VCARD\r\n"))
	}))
	defer server.Close()
	client := mustClient(t, Options{HTTPClient: server.Client(), MaxResponseBytes: 1 << 20})
	contact, err := client.Fetch(context.Background(), server.URL+"/books/a.vcf", Auth{Username: "alice", Password: "secret"})
	if err != nil || contact.Href != server.URL+"/books/a.vcf" || contact.ETag != `"8"` || !strings.Contains(contact.VCard, "UID:uid-a") {
		t.Fatalf("contact=%#v err=%v", contact, err)
	}
}

func TestClientReportsConditionalConflictWithoutLeakingResponse(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusPreconditionFailed)
		_, _ = response.Write([]byte("remote private contact body"))
	}))
	defer server.Close()
	client := mustClient(t, Options{HTTPClient: server.Client()})
	_, err := client.Put(context.Background(), server.URL+"/a.vcf", "BEGIN:VCARD\r\nFN:Alice\r\nEND:VCARD\r\n", `"stale"`, false, Auth{Username: "alice", Password: "secret"})
	if err != ErrPrecondition || strings.Contains(err.Error(), "private") {
		t.Fatalf("error=%v", err)
	}
}

func TestClientRejectsAuthenticationAndOversizedResponsesWithoutLeakingBody(t *testing.T) {
	t.Run("authentication", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusUnauthorized)
			_, _ = response.Write([]byte("password=top-secret rejected by upstream"))
		}))
		defer server.Close()
		client := mustClient(t, Options{HTTPClient: server.Client()})
		_, err := client.Discover(context.Background(), &accounts.CardDAVEndpoint{URL: server.URL, AllowInsecureTLS: true}, accounts.Credentials{Username: "alice", Password: "top-secret"})
		if err != ErrAuthentication || strings.Contains(err.Error(), "top-secret") || strings.Contains(err.Error(), "upstream") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("body limit", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(StatusMultiStatus)
			_, _ = response.Write([]byte(strings.Repeat("x", 1025)))
		}))
		defer server.Close()
		client := mustClient(t, Options{HTTPClient: server.Client(), MaxResponseBytes: 1024})
		_, err := client.Discover(context.Background(), &accounts.CardDAVEndpoint{URL: server.URL, AllowInsecureTLS: true}, accounts.Credentials{Username: "alice", Password: "secret"})
		if err != ErrResponseTooLarge {
			t.Fatalf("error = %v", err)
		}
	})
}

func multistatus(response http.ResponseWriter, inner string) {
	response.Header().Set("Content-Type", "application/xml; charset=utf-8")
	response.WriteHeader(StatusMultiStatus)
	_, _ = response.Write([]byte(`<?xml version="1.0" encoding="utf-8"?><d:multistatus xmlns:d="DAV:" xmlns:card="urn:ietf:params:xml:ns:carddav" xmlns:cs="http://calendarserver.org/ns/">` + inner + `</d:multistatus>`))
}

func mustClient(t *testing.T, options Options) *Client {
	t.Helper()
	client, err := NewClient(options)
	if err != nil {
		t.Fatal(err)
	}
	return client
}
