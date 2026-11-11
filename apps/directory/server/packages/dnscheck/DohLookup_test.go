package dnscheck

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

type seenRequest struct {
	name   string
	kind   string
	accept string
}

func dohServer(t *testing.T, status int, body string, seen *seenRequest) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		*seen = seenRequest{name: request.URL.Query().Get("name"), kind: request.URL.Query().Get("type"), accept: request.Header.Get("Accept")}
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func dohLookup(server *httptest.Server) DohLookup {
	return NewDohLookup(server.URL+"/dns-query", server.Client())
}

func TestDohMxReturnsHostsWithoutPreferenceOrDot(t *testing.T) {
	seen := seenRequest{}
	server := dohServer(t, http.StatusOK, `{"Status":0,"Answer":[{"name":"example.com.","type":15,"data":"10 MX.Example.NET."},{"name":"example.com.","type":15,"data":"20 backup.example.net."}]}`, &seen)
	got, err := dohLookup(server).Mx("example.com")
	if err != nil || !slices.Equal(got, []string{"mx.example.net", "backup.example.net"}) {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestDohQuerySendsNameTypeAndAcceptHeader(t *testing.T) {
	seen := seenRequest{}
	server := dohServer(t, http.StatusOK, `{"Status":0}`, &seen)
	_, err := dohLookup(server).Txt("_dmarc.example.com&type=A")
	want := seenRequest{name: "_dmarc.example.com&type=A", kind: "TXT", accept: "application/dns-json"}
	if err != nil || seen != want {
		t.Fatalf("got %+v, %v", seen, err)
	}
}

func TestDohMxQueryAsksForTypeMx(t *testing.T) {
	seen := seenRequest{}
	server := dohServer(t, http.StatusOK, `{"Status":0}`, &seen)
	_, err := dohLookup(server).Mx("example.com")
	if err != nil || seen.kind != "MX" {
		t.Fatalf("got %+v, %v", seen, err)
	}
}

func TestDohTxtJoinsSplitStrings(t *testing.T) {
	seen := seenRequest{}
	server := dohServer(t, http.StatusOK, `{"Status":0,"Answer":[{"name":"example.com.","type":16,"data":"\"v=spf1 \" \"mx -all\""}]}`, &seen)
	got, err := dohLookup(server).Txt("example.com")
	if err != nil || !slices.Equal(got, []string{"v=spf1 mx -all"}) {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDohTxtUnescapesQuotes(t *testing.T) {
	seen := seenRequest{}
	server := dohServer(t, http.StatusOK, `{"Status":0,"Answer":[{"name":"example.com.","type":16,"data":"\"say \\\"hi\\\"\""}]}`, &seen)
	got, err := dohLookup(server).Txt("example.com")
	if err != nil || !slices.Equal(got, []string{`say "hi"`}) {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDohTxtIgnoresCnameAnswers(t *testing.T) {
	seen := seenRequest{}
	server := dohServer(t, http.StatusOK, `{"Status":0,"Answer":[{"name":"_dmarc.example.com.","type":5,"data":"dmarc.provider.net."},{"name":"dmarc.provider.net.","type":16,"data":"\"v=DMARC1; p=reject\""}]}`, &seen)
	got, err := dohLookup(server).Txt("_dmarc.example.com")
	if err != nil || !slices.Equal(got, []string{"v=DMARC1; p=reject"}) {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDohNameErrorGivesNoRecords(t *testing.T) {
	seen := seenRequest{}
	server := dohServer(t, http.StatusOK, `{"Status":3}`, &seen)
	got, err := dohLookup(server).Mx("nowhere.example.com")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestDohServerFailureStatusIsLookupFailed(t *testing.T) {
	seen := seenRequest{}
	server := dohServer(t, http.StatusOK, `{"Status":2}`, &seen)
	_, err := dohLookup(server).Txt("example.com")
	want := LookupFailed{Name: "example.com", Status: 2}
	var failed LookupFailed
	if !errors.As(err, &failed) || failed != want {
		t.Fatalf("got %v", err)
	}
}

func TestDohHttpErrorIsLookupFailed(t *testing.T) {
	seen := seenRequest{}
	server := dohServer(t, http.StatusInternalServerError, `oops`, &seen)
	_, err := dohLookup(server).Mx("example.com")
	want := LookupFailed{Name: "example.com", Status: http.StatusInternalServerError}
	var failed LookupFailed
	if !errors.As(err, &failed) || failed != want {
		t.Fatalf("got %v", err)
	}
}

func TestDohBadJsonIsAnError(t *testing.T) {
	seen := seenRequest{}
	server := dohServer(t, http.StatusOK, `not json`, &seen)
	if _, err := dohLookup(server).Txt("example.com"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDohBadEndpointIsAnError(t *testing.T) {
	if _, err := NewDohLookup("://bad", http.DefaultClient).Mx("example.com"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDohUnreachableServerIsAWrappedError(t *testing.T) {
	seen := seenRequest{}
	server := dohServer(t, http.StatusOK, `{"Status":0}`, &seen)
	lookup := dohLookup(server)
	server.Close()
	if _, err := lookup.Txt("example.com"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestLookupFailedNamesTheNameAndStatus(t *testing.T) {
	got := LookupFailed{Name: "example.com", Status: 2}.Error()
	if got != `dns lookup of "example.com" failed with status 2` {
		t.Fatalf("got %q", got)
	}
}
