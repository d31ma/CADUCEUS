package caldav

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/d31ma/caduceus/internal/accounts"
)

func TestClientDiscoversCalendarsAndDownloadsEventResources(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if username, password, ok := request.BasicAuth(); !ok || username != "alice" || password != "secret" {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		response.Header().Set("Content-Type", "application/xml")
		response.WriteHeader(statusMultiStatus)
		switch request.URL.Path {
		case "/.well-known/caldav":
			_, _ = fmt.Fprint(response, multistatus(`<d:response><d:href>/</d:href><d:propstat><d:prop><d:current-user-principal><d:href>/principals/alice/</d:href></d:current-user-principal></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`))
		case "/principals/alice/":
			_, _ = fmt.Fprint(response, multistatus(`<d:response><d:href>/principals/alice/</d:href><d:propstat><d:prop><c:calendar-home-set><d:href>/calendars/alice/</d:href></c:calendar-home-set></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`))
		case "/calendars/alice/":
			_, _ = fmt.Fprint(response, multistatus(`<d:response><d:href>/calendars/alice/work/</d:href><d:propstat><d:prop><d:displayname>Work</d:displayname><d:resourcetype><c:calendar/></d:resourcetype><d:sync-token>sync-1</d:sync-token><cs:getctag>ctag-1</cs:getctag><ical:calendar-color>#336699FF</ical:calendar-color></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`))
		case "/calendars/alice/work/":
			if request.Method != "REPORT" {
				t.Fatalf("calendar method = %s", request.Method)
			}
			_, _ = fmt.Fprint(response, multistatus(`<d:response><d:href>/calendars/alice/work/one.ics</d:href><d:propstat><d:prop><d:getetag>&quot;1&quot;</d:getetag><c:calendar-data>BEGIN:VCALENDAR&#13;&#10;BEGIN:VEVENT&#13;&#10;UID:one&#13;&#10;SUMMARY:One&#13;&#10;END:VEVENT&#13;&#10;END:VCALENDAR&#13;&#10;</c:calendar-data></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`))
		default:
			t.Fatalf("unexpected CalDAV path %s", request.URL.Path)
		}
	}))
	defer server.Close()
	client, err := NewClient(Options{HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	discovery, err := client.Discover(context.Background(), &accounts.CalDAVEndpoint{URL: server.URL}, accounts.Credentials{Username: "alice", Password: "secret"})
	if err != nil || len(discovery.Calendars) != 1 || discovery.Calendars[0].Name != "Work" || discovery.Calendars[0].Color != "#336699FF" {
		t.Fatalf("discovery=%#v err=%v", discovery, err)
	}
	resources, err := client.InitialSync(context.Background(), discovery.Calendars[0], Auth{Username: "alice", Password: "secret"})
	if err != nil || len(resources) != 1 || !strings.Contains(resources[0].ICalendar, "UID:one") || resources[0].ETag != `"1"` {
		t.Fatalf("resources=%#v err=%v", resources, err)
	}
}

func TestClientUsesConditionalResourceWritesAndBoundedFetch(t *testing.T) {
	calendar := "BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nUID:one\r\nSUMMARY:One\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if username, password, ok := request.BasicAuth(); !ok || username != "alice" || password != "secret" {
			response.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch request.Method {
		case http.MethodPut:
			if request.Header.Get("Content-Type") != "text/calendar; charset=utf-8" {
				t.Fatalf("content type = %q", request.Header.Get("Content-Type"))
			}
			if request.URL.Query().Get("create") == "1" {
				if request.Header.Get("If-None-Match") != "*" || request.Header.Get("If-Match") != "" {
					t.Fatalf("create preconditions = %#v", request.Header)
				}
				response.WriteHeader(http.StatusCreated)
				return
			}
			if request.Header.Get("If-Match") != `"7"` {
				t.Fatalf("update If-Match = %q", request.Header.Get("If-Match"))
			}
			response.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			response.Header().Set("ETag", `"8"`)
			_, _ = response.Write([]byte(calendar))
		case http.MethodDelete:
			if request.Header.Get("If-Match") != `"8"` {
				t.Fatalf("delete If-Match = %q", request.Header.Get("If-Match"))
			}
			response.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", request.Method)
		}
	}))
	defer server.Close()
	client, err := NewClient(Options{HTTPClient: server.Client()})
	if err != nil {
		t.Fatal(err)
	}
	auth := Auth{Username: "alice", Password: "secret"}
	if _, err := client.Put(context.Background(), server.URL+"/one.ics?create=1", calendar, "", true, auth); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Put(context.Background(), server.URL+"/one.ics", calendar, `"7"`, false, auth); err != nil {
		t.Fatal(err)
	}
	fetched, err := client.Fetch(context.Background(), server.URL+"/one.ics", auth)
	if err != nil || fetched.ETag != `"8"` || fetched.ICalendar != calendar || fetched.Href == "" {
		t.Fatalf("fetched=%#v err=%v", fetched, err)
	}
	if err := client.Delete(context.Background(), server.URL+"/one.ics", `"8"`, auth); err != nil {
		t.Fatal(err)
	}
}

func multistatus(content string) string {
	return `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:" xmlns:c="urn:ietf:params:xml:ns:caldav" xmlns:cs="http://calendarserver.org/ns/" xmlns:ical="http://apple.com/ns/ical/">` + content + `</d:multistatus>`
}
