package main

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mesail.dev/edge/smtp/protocol"
	"mesail.dev/edge/smtp/reception"
	"mesail.dev/edge/smtp/startup"
)

type endToEnd struct {
	address   string
	rawFolder string
	mutex     sync.Mutex
	stored    []bool
	envelopes []reception.InboundEnvelope
}

func directoryHandler(writer http.ResponseWriter, request *http.Request) {
	address := strings.TrimPrefix(request.URL.Path, "/api/v1/directory/addresses/")
	writer.Header().Set("Content-Type", "application/json")
	switch {
	case request.Header.Get("Authorization") != "Bearer service-token":
		writer.WriteHeader(http.StatusUnauthorized)
	case address == "ana@example.com":
		writer.Write([]byte(`{"address":"ana@example.com","domainId":"example.com","kind":"member","targetId":"mem_ana"}`))
	case strings.HasSuffix(address, "@example.com"):
		writer.WriteHeader(http.StatusNotFound)
		writer.Write([]byte(`{"code":"not_found","message":"no such address"}`))
	default:
		writer.WriteHeader(http.StatusNotFound)
		writer.Write([]byte(`{"code":"domain_not_hosted","message":"not hosted"}`))
	}
}

func (test *endToEnd) mailInHandler(writer http.ResponseWriter, request *http.Request) {
	var envelope reception.InboundEnvelope
	json.NewDecoder(request.Body).Decode(&envelope)
	_, err := os.Stat(filepath.Join(test.rawFolder, filepath.FromSlash(envelope.RawBlobKey)))
	test.mutex.Lock()
	test.stored = append(test.stored, err == nil)
	test.envelopes = append(test.envelopes, envelope)
	test.mutex.Unlock()
	writer.WriteHeader(http.StatusCreated)
	writer.Write([]byte(`{"inboundId":"in_1"}`))
}

func startEndToEnd(t *testing.T) *endToEnd {
	t.Helper()
	test := &endToEnd{rawFolder: t.TempDir()}
	directory := httptest.NewServer(http.HandlerFunc(directoryHandler))
	mailIn := httptest.NewServer(http.HandlerFunc(test.mailInHandler))
	t.Cleanup(directory.Close)
	t.Cleanup(mailIn.Close)
	settings := startup.ListenerSettings{Hostname: "mx.mesail.test", MaxSize: 26214400, MaxConnections: 10, DirectoryUrl: directory.URL, MailInUrl: mailIn.URL, RawStoreFolder: test.rawFolder, ServiceToken: "service-token"}
	built, err := startup.Wiring{}.Build(settings, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := protocol.NewServer(built)
	go server.Serve(listener)
	t.Cleanup(func() { server.Shutdown(t.Context()) })
	test.address = listener.Addr().String()
	return test
}

func TestEndToEndUnknownHostedRecipientIsRefusedAtRcpt(t *testing.T) {
	test := startEndToEnd(t)
	client, err := smtp.Dial(test.address)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Mail("dan@outside.test"); err != nil {
		t.Fatal(err)
	}
	err = client.Rcpt("nobody@example.com")
	if !replyIs(err, 550, "5.1.1") {
		t.Fatalf("want 550 5.1.1 at RCPT TO, got %v", err)
	}
}

func TestEndToEndMailForAnUnhostedDomainIsNotRelayed(t *testing.T) {
	test := startEndToEnd(t)
	client, err := smtp.Dial(test.address)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Mail("dan@outside.test"); err != nil {
		t.Fatal(err)
	}
	err = client.Rcpt("someone@not-hosted.test")
	if !replyIs(err, 550, "5.7.1") {
		t.Fatalf("want 550 5.7.1, got %v", err)
	}
}

func TestEndToEndHostedRecipientIsAcceptedAfterMailIn(t *testing.T) {
	test := startEndToEnd(t)
	client, err := smtp.Dial(test.address)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Mail("dan@outside.test"); err != nil {
		t.Fatal(err)
	}
	if err := client.Rcpt("ana@example.com"); err != nil {
		t.Fatal(err)
	}
	writer, err := client.Data()
	if err != nil {
		t.Fatal(err)
	}
	message := "From: dan@outside.test\r\nTo: ana@example.com\r\nSubject: Hello\r\nMessage-ID: <bb-1@outside.test>\r\n\r\nHi Ana\r\n"
	if _, err := writer.Write([]byte(message)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("DATA was not accepted: %v", err)
	}
	test.mutex.Lock()
	defer test.mutex.Unlock()
	if len(test.envelopes) != 1 || !test.stored[0] || test.envelopes[0].Headers.MessageId != "bb-1@outside.test" || test.envelopes[0].Recipients[0] != "ana@example.com" {
		t.Fatalf("mail-in did not see a stored raw copy: %+v %v", test.envelopes, test.stored)
	}
}

func TestProgramStopsWithAClearErrorWhenSettingsAreMissing(t *testing.T) {
	t.Setenv("MESAIL_DIRECTORY_URL", "")
	err := Program{}.Run()
	if err == nil || !strings.Contains(err.Error(), "MESAIL_DIRECTORY_URL") {
		t.Fatalf("want a missing setting error, got %v", err)
	}
}

func replyIs(err error, code int, enhancedCode string) bool {
	var reply *textproto.Error
	return errors.As(err, &reply) && reply.Code == code && strings.HasPrefix(reply.Msg, enhancedCode)
}
