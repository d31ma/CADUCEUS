package adapters

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"mesail.dev/edge/smtp/reception"
)

type seenRequest struct {
	path          string
	authorization string
}

func directoryServer(t *testing.T, status int, body string, seen *seenRequest) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seen.path = request.URL.EscapedPath()
		seen.authorization = request.Header.Get("Authorization")
		writer.WriteHeader(status)
		writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server.URL + "/"
}

func TestDirectoryFoundAddressIsHosted(t *testing.T) {
	seen := &seenRequest{}
	address := directoryServer(t, http.StatusOK, `{"address":"ana@example.com","domainId":"example.com","kind":"member","targetId":"mem_1"}`, seen)
	found, err := NewHttpDirectory(address, NewStaticTokens("service-token"), http.DefaultClient).Lookup(context.Background(), "ana@example.com")
	if err != nil || found.DomainId != "example.com" || found.TargetId != "mem_1" {
		t.Fatalf("unexpected answer %+v %v", found, err)
	}
	if seen.authorization != "Bearer service-token" {
		t.Fatalf("unexpected authorization %q", seen.authorization)
	}
}

func TestDirectoryPathIsEscaped(t *testing.T) {
	seen := &seenRequest{}
	address := directoryServer(t, http.StatusOK, `{}`, seen)
	NewHttpDirectory(address, NewStaticTokens("t"), http.DefaultClient).Lookup(context.Background(), "a/b?c@example.com")
	if seen.path != "/api/v1/directory/addresses/a%2Fb%3Fc@example.com" {
		t.Fatalf("unexpected path %q", seen.path)
	}
}

func TestDirectoryDomainNotHosted(t *testing.T) {
	address := directoryServer(t, http.StatusNotFound, `{"code":"domain_not_hosted","message":"no"}`, &seenRequest{})
	_, err := NewHttpDirectory(address, NewStaticTokens("t"), http.DefaultClient).Lookup(context.Background(), "a@else.test")
	if !errors.Is(err, reception.DomainNotHosted{}) {
		t.Fatalf("want DomainNotHosted, got %v", err)
	}
}

func TestDirectoryOtherNotFoundIsUnknown(t *testing.T) {
	address := directoryServer(t, http.StatusNotFound, `{"code":"not_found","message":"no"}`, &seenRequest{})
	_, err := NewHttpDirectory(address, NewStaticTokens("t"), http.DefaultClient).Lookup(context.Background(), "x@example.com")
	if !errors.Is(err, reception.UnknownAddress{}) {
		t.Fatalf("want UnknownAddress, got %v", err)
	}
}

func TestDirectoryNotFoundWithoutBodyIsUnknown(t *testing.T) {
	address := directoryServer(t, http.StatusNotFound, ``, &seenRequest{})
	_, err := NewHttpDirectory(address, NewStaticTokens("t"), http.DefaultClient).Lookup(context.Background(), "x@example.com")
	if !errors.Is(err, reception.UnknownAddress{}) {
		t.Fatalf("want UnknownAddress, got %v", err)
	}
}

func TestDirectoryServerErrorIsTemporary(t *testing.T) {
	address := directoryServer(t, http.StatusServiceUnavailable, `{}`, &seenRequest{})
	_, err := NewHttpDirectory(address, NewStaticTokens("t"), http.DefaultClient).Lookup(context.Background(), "x@example.com")
	var unexpected UnexpectedStatus
	if !errors.As(err, &unexpected) || unexpected.Status != http.StatusServiceUnavailable || unexpected.Error() != "directory answered HTTP 503" {
		t.Fatalf("want UnexpectedStatus 503, got %v", err)
	}
}

func TestDirectoryUnreachableIsAnError(t *testing.T) {
	_, err := NewHttpDirectory("http://127.0.0.1:1", NewStaticTokens("t"), http.DefaultClient).Lookup(context.Background(), "x@example.com")
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestDirectoryTokenFailureIsAnError(t *testing.T) {
	_, err := NewHttpDirectory("http://127.0.0.1:1", failingTokens{}, http.DefaultClient).Lookup(context.Background(), "x@example.com")
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestDirectoryBadBaseUrlIsAnError(t *testing.T) {
	_, err := NewHttpDirectory("http://bad host", NewStaticTokens("t"), http.DefaultClient).Lookup(context.Background(), "x@example.com")
	if err == nil {
		t.Fatal("want an error")
	}
}

type failingTokens struct{}

func (failingTokens) Token(_ context.Context) (string, error) {
	return "", errors.New("no token")
}
