package search

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func fakeServices(t *testing.T, conversations string, channels string, status int) MembershipClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer ana" {
			writer.WriteHeader(http.StatusUnauthorized)
			return
		}
		writer.WriteHeader(status)
		switch {
		case request.URL.Path == "/api/v1/messaging/conversations" && request.URL.Query().Get("cursor") == "":
			writer.Write([]byte(conversations))
		case request.URL.Path == "/api/v1/messaging/conversations":
			writer.Write([]byte(`{"items":[{"conversationId":"cnv_page2"}],"nextCursor":""}`))
		case request.URL.Path == "/api/v1/channels":
			writer.Write([]byte(channels))
		}
	}))
	t.Cleanup(server.Close)
	return NewMembershipClient(server.URL+"/", server.URL, server.Client())
}

func TestMembershipReadableJoinsConversationsAndChannels(t *testing.T) {
	client := fakeServices(t, `{"items":[{"conversationId":"cnv_direct"}],"nextCursor":""}`, `[{"conversationId":"cnv_public"}]`, http.StatusOK)
	readable, err := client.Readable("Bearer ana")
	if err != nil || !readable["cnv_direct"] || !readable["cnv_public"] || len(readable) != 2 {
		t.Fatalf("got %v %v", readable, err)
	}
}

func TestMembershipReadableFollowsTheConversationCursor(t *testing.T) {
	client := fakeServices(t, `{"items":[{"conversationId":"cnv_page1"}],"nextCursor":"next"}`, `[]`, http.StatusOK)
	readable, err := client.Readable("Bearer ana")
	if err != nil || !readable["cnv_page1"] || !readable["cnv_page2"] {
		t.Fatalf("got %v %v", readable, err)
	}
}

func TestMembershipReadableFailsWhenMessagingRefuses(t *testing.T) {
	client := fakeServices(t, `{}`, `[]`, http.StatusOK)
	_, err := client.Readable("Bearer eve")
	if !errors.As(err, &MembershipFailed{}) || err.Error() != "the messaging service answered status 401" {
		t.Fatalf("got %v", err)
	}
}

func TestMembershipReadableFailsWhenChannelsFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/v1/channels" {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.Write([]byte(`{"items":[],"nextCursor":""}`))
	}))
	defer server.Close()
	_, err := NewMembershipClient(server.URL, server.URL, server.Client()).Readable("Bearer ana")
	if !errors.As(err, &MembershipFailed{}) {
		t.Fatalf("got %v", err)
	}
}

func TestMembershipReadableFailsWhenTheServiceIsDown(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	address := server.URL
	server.Close()
	if _, err := NewMembershipClient(address, address, http.DefaultClient).Readable("Bearer ana"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestMembershipReadableRefusesABadAddress(t *testing.T) {
	if _, err := NewMembershipClient("http://bad host", "", http.DefaultClient).Readable("Bearer ana"); err == nil {
		t.Fatal("expected an error")
	}
}
