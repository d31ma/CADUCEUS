package adapters

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type tokenServer struct {
	calls   atomic.Int32
	forms   chan string
	address string
}

func newTokenServer(t *testing.T, status int, answer string) *tokenServer {
	t.Helper()
	tokens := &tokenServer{forms: make(chan string, 100)}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		tokens.calls.Add(1)
		request.ParseForm()
		tokens.forms <- request.PostForm.Encode()
		writer.WriteHeader(status)
		writer.Write([]byte(answer))
	}))
	t.Cleanup(server.Close)
	tokens.address = server.URL
	return tokens
}

func secretFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(path, []byte("the-secret\n"), 0o600)
	return path
}

type clock struct {
	mutex sync.Mutex
	now   time.Time
}

func (clock *clock) read() time.Time {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	return clock.now
}

func (clock *clock) advance(duration time.Duration) {
	clock.mutex.Lock()
	defer clock.mutex.Unlock()
	clock.now = clock.now.Add(duration)
}

func TestStaticTokensGiveTheSameToken(t *testing.T) {
	token, err := NewStaticTokens("abc").Token(context.Background())
	if err != nil || token != "abc" {
		t.Fatalf("unexpected token %q %v", token, err)
	}
}

func TestClientCredentialsSendTheFormAndCacheTheToken(t *testing.T) {
	server := newTokenServer(t, http.StatusOK, `{"access_token":"first","expires_in":300}`)
	fakeClock := &clock{now: time.Unix(1000, 0)}
	tokens := NewClientCredentialTokens(TokenSettings{Url: server.address, ClientId: "smtp-edge", SecretFile: secretFile(t)}, http.DefaultClient, fakeClock.read)
	first, err := tokens.Token(context.Background())
	second, _ := tokens.Token(context.Background())
	form := <-server.forms
	if err != nil || first != "first" || second != "first" || server.calls.Load() != 1 {
		t.Fatalf("want one call and a cached token, got %q %q %d %v", first, second, server.calls.Load(), err)
	}
	if !strings.Contains(form, "grant_type=client_credentials") || !strings.Contains(form, "client_id=smtp-edge") || !strings.Contains(form, "client_secret=the-secret") {
		t.Fatalf("unexpected form %q", form)
	}
}

func TestClientCredentialsRefreshThirtySecondsBeforeExpiry(t *testing.T) {
	server := newTokenServer(t, http.StatusOK, `{"access_token":"token","expires_in":300}`)
	fakeClock := &clock{now: time.Unix(1000, 0)}
	tokens := NewClientCredentialTokens(TokenSettings{Url: server.address, ClientId: "smtp-edge", SecretFile: secretFile(t)}, http.DefaultClient, fakeClock.read)
	tokens.Token(context.Background())
	fakeClock.advance(269 * 1e9)
	tokens.Token(context.Background())
	calls := server.calls.Load()
	fakeClock.advance(1 * 1e9)
	tokens.Token(context.Background())
	if calls != 1 || server.calls.Load() != 2 {
		t.Fatalf("want a refresh at 270 seconds, got %d then %d calls", calls, server.calls.Load())
	}
}

func TestClientCredentialsAreSafeForManyCallers(t *testing.T) {
	server := newTokenServer(t, http.StatusOK, `{"access_token":"token","expires_in":300}`)
	tokens := NewClientCredentialTokens(TokenSettings{Url: server.address, ClientId: "smtp-edge", SecretFile: secretFile(t)}, http.DefaultClient, time.Now)
	group := sync.WaitGroup{}
	for range 20 {
		group.Go(func() { tokens.Token(context.Background()) })
	}
	group.Wait()
	if server.calls.Load() != 1 {
		t.Fatalf("want one call, got %d", server.calls.Load())
	}
}

func TestClientCredentialsRefusedIsAnError(t *testing.T) {
	server := newTokenServer(t, http.StatusUnauthorized, `{"error":"invalid_client"}`)
	_, err := NewClientCredentialTokens(TokenSettings{Url: server.address, ClientId: "smtp-edge", SecretFile: secretFile(t)}, http.DefaultClient, time.Now).Token(context.Background())
	if err == nil || strings.Contains(err.Error(), "the-secret") {
		t.Fatalf("want an error without the secret, got %v", err)
	}
}

func TestClientCredentialsWithoutTokenIsAnError(t *testing.T) {
	server := newTokenServer(t, http.StatusOK, `{"expires_in":300}`)
	_, err := NewClientCredentialTokens(TokenSettings{Url: server.address, ClientId: "smtp-edge", SecretFile: secretFile(t)}, http.DefaultClient, time.Now).Token(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestClientCredentialsWithBrokenJsonIsAnError(t *testing.T) {
	server := newTokenServer(t, http.StatusOK, `not json`)
	_, err := NewClientCredentialTokens(TokenSettings{Url: server.address, ClientId: "smtp-edge", SecretFile: secretFile(t)}, http.DefaultClient, time.Now).Token(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestClientCredentialsWithoutSecretFileIsAnError(t *testing.T) {
	_, err := NewClientCredentialTokens(TokenSettings{Url: "http://127.0.0.1:1", ClientId: "smtp-edge", SecretFile: filepath.Join(t.TempDir(), "missing")}, http.DefaultClient, time.Now).Token(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestClientCredentialsUnreachableIsAnError(t *testing.T) {
	_, err := NewClientCredentialTokens(TokenSettings{Url: "http://127.0.0.1:1", ClientId: "smtp-edge", SecretFile: secretFile(t)}, http.DefaultClient, time.Now).Token(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestClientCredentialsBadUrlIsAnError(t *testing.T) {
	_, err := NewClientCredentialTokens(TokenSettings{Url: "http://bad host", ClientId: "smtp-edge", SecretFile: secretFile(t)}, http.DefaultClient, time.Now).Token(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
}
