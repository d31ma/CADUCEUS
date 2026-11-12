package adapters

import (
	"context"
	"errors"
	"io"
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
	bodies  chan string
	address string
}

func newTokenServer(t *testing.T, status int, answer string) *tokenServer {
	t.Helper()
	tokens := &tokenServer{bodies: make(chan string, 100)}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		tokens.calls.Add(1)
		body, _ := io.ReadAll(request.Body)
		tokens.bodies <- request.Header.Get("Content-Type") + " " + string(body)
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

func identityTokens(t *testing.T, address string, now func() time.Time) *IdentityTokens {
	t.Helper()
	settings := TokenSettings{Url: address, SecretFile: secretFile(t), CacheFolder: t.TempDir()}
	return NewIdentityTokens(settings, http.DefaultClient, now)
}

func TestIdentityTokensSendJsonForTheWholeInstallAndCacheTheToken(t *testing.T) {
	server := newTokenServer(t, http.StatusOK, `{"accessToken":"first","expiresIn":300}`)
	fakeClock := &clock{now: time.Unix(1000, 0)}
	tokens := identityTokens(t, server.address, fakeClock.read)
	first, err := tokens.Token(context.Background())
	second, _ := tokens.Token(context.Background())
	body := <-server.bodies
	if err != nil || first != "first" || second != "first" || server.calls.Load() != 1 {
		t.Fatalf("want one call and a cached token, got %q %q %d %v", first, second, server.calls.Load(), err)
	}
	if body != `application/json {"clientId":"mesail-service","clientSecret":"the-secret","domainId":"*"}` {
		t.Fatalf("unexpected request %q", body)
	}
}

func TestIdentityTokensRefreshTenSecondsBeforeExpiry(t *testing.T) {
	server := newTokenServer(t, http.StatusOK, `{"accessToken":"token","expiresIn":300}`)
	fakeClock := &clock{now: time.Unix(1000, 0)}
	tokens := identityTokens(t, server.address, fakeClock.read)
	tokens.Token(context.Background())
	fakeClock.advance(289 * time.Second)
	tokens.Token(context.Background())
	calls := server.calls.Load()
	fakeClock.advance(1 * time.Second)
	tokens.Token(context.Background())
	if calls != 1 || server.calls.Load() != 2 {
		t.Fatalf("want a refresh at 290 seconds, got %d then %d calls", calls, server.calls.Load())
	}
}

func TestIdentityTokensAreSafeForManyCallers(t *testing.T) {
	server := newTokenServer(t, http.StatusOK, `{"accessToken":"token","expiresIn":300}`)
	tokens := identityTokens(t, server.address, time.Now)
	group := sync.WaitGroup{}
	for range 20 {
		group.Go(func() { tokens.Token(context.Background()) })
	}
	group.Wait()
	if server.calls.Load() != 1 {
		t.Fatalf("want one call, got %d", server.calls.Load())
	}
}

func TestIdentityTokensRefusedIsAnError(t *testing.T) {
	server := newTokenServer(t, http.StatusUnauthorized, `{"error":"invalid_client"}`)
	_, err := identityTokens(t, server.address, time.Now).Token(context.Background())
	if err == nil || strings.Contains(err.Error(), "the-secret") {
		t.Fatalf("want an error without the secret, got %v", err)
	}
}

func TestIdentityTokensServerErrorIsAnError(t *testing.T) {
	server := newTokenServer(t, http.StatusServiceUnavailable, `{}`)
	_, err := identityTokens(t, server.address, time.Now).Token(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestIdentityTokensWithoutTokenIsAnError(t *testing.T) {
	server := newTokenServer(t, http.StatusOK, `{"expiresIn":300}`)
	_, err := identityTokens(t, server.address, time.Now).Token(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestIdentityTokensWithBrokenJsonIsAnError(t *testing.T) {
	server := newTokenServer(t, http.StatusOK, `not json`)
	_, err := identityTokens(t, server.address, time.Now).Token(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
}

func TestIdentityTokensWithoutSecretFileIsAnError(t *testing.T) {
	settings := TokenSettings{Url: "http://127.0.0.1:1", SecretFile: filepath.Join(t.TempDir(), "missing"), CacheFolder: t.TempDir()}
	_, err := NewIdentityTokens(settings, http.DefaultClient, time.Now).Token(context.Background())
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("want a missing file error, got %v", err)
	}
}

func TestIdentityTokensUnreachableIsAnError(t *testing.T) {
	_, err := identityTokens(t, "http://127.0.0.1:1", time.Now).Token(context.Background())
	if err == nil {
		t.Fatal("want an error")
	}
}
