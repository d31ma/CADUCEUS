package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestSessionRoutesAreLimitedPerAddress(t *testing.T) {
	fake := newFakeAws(t)
	fake.answer = `{"statusCode":200,"body":"ok"}`
	settings := settingsFor(fake, "")
	settings.Limited = []string{"POST /api/v1/identity/session"}
	settings.RateLimit = 2
	handler := handlerFor(t, settings)
	codes := []int{}
	for range 3 {
		codes = append(codes, call(t, handler, http.MethodPost, "/api/v1/identity/session", "").Code)
	}
	other := httptest.NewRequest(http.MethodPost, "/api/v1/identity/session", nil)
	other.RemoteAddr = "198.51.100.7:4000"
	otherReply := httptest.NewRecorder()
	handler.ServeHTTP(otherReply, other)
	unlimited := call(t, handler, http.MethodGet, "/api/v1/identity/health", "")
	refused := call(t, handler, http.MethodPost, "/api/v1/identity/session", "")
	if codes[0] != 200 || codes[1] != 200 || codes[2] != http.StatusTooManyRequests || otherReply.Code != 200 || unlimited.Code != 200 || refused.Header().Get("Retry-After") != "60" {
		t.Fatalf("got %v, other %d, unlimited %d, retry %q", codes, otherReply.Code, unlimited.Code, refused.Header().Get("Retry-After"))
	}
}

func TestRateWindowStartsAgainAfterItEnds(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	limiter := RateLimiter{}.New(1, time.Minute, func() time.Time { return now })
	first := limiter.Allow("192.0.2.1")
	second := limiter.Allow("192.0.2.1")
	now = now.Add(time.Minute)
	third := limiter.Allow("192.0.2.1")
	if !first || second || !third || len(limiter.windows) != 1 {
		t.Fatalf("got %v %v %v, %d windows", first, second, third, len(limiter.windows))
	}
}

func TestClientWithoutPortIsKeptWhole(t *testing.T) {
	if (RateLimitHandler{}).client("192.0.2.9") != "192.0.2.9" {
		t.Fatal("want the address itself")
	}
}

func TestSingleWriterFunctionGetsOneRequestAtATime(t *testing.T) {
	limits := ConcurrencyLimits{}.Of([]string{"identity", "directory"}, []string{"directory"}, "mesail-", 1000)
	release, err := QueuedEntry{}.Take(context.Background(), limits.Gate("mesail-directory"), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	waiting, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, blocked := QueuedEntry{}.Take(waiting, limits.Gate("mesail-directory"), time.Now)
	other, otherErr := QueuedEntry{}.Take(context.Background(), limits.Gate("mesail-identity"), time.Now)
	release()
	again, againErr := QueuedEntry{}.Take(context.Background(), limits.Gate("mesail-directory"), time.Now)
	if blocked == nil || otherErr != nil || againErr != nil || limits.Gates["mesail-identity"].capacity != 1000 {
		t.Fatalf("blocked %v, other %v, again %v", blocked, otherErr, againErr)
	}
	other()
	again()
}

func TestSingleWriterRequestsAreQueuedNotRefused(t *testing.T) {
	fake := newFakeAws(t)
	fake.answer = `{"statusCode":200,"body":"ok"}`
	settings := settingsFor(fake, "")
	settings.SingleWriters = []string{"mail-in"}
	handler := handlerFor(t, settings)
	codes := make([]int, 5)
	var group sync.WaitGroup
	for index := range codes {
		group.Add(1)
		go func() {
			defer group.Done()
			codes[index] = call(t, handler, http.MethodGet, "/api/v1/mail-in/health", "").Code
		}()
	}
	group.Wait()
	if codes[0]+codes[1]+codes[2]+codes[3]+codes[4] != 5*http.StatusOK {
		t.Fatalf("got %v", codes)
	}
}

func TestSettingsReadTheLimits(t *testing.T) {
	values := map[string]string{
		"MESAIL_ROUTER_UPSTREAMS":      "identity=http://x",
		"MESAIL_ROUTER_SINGLE_WRITERS": "directory,mail-in",
		"MESAIL_ROUTER_RATE_LIMIT":     "5",
	}
	settings, err := SettingsReader{Lookup: lookupIn(values)}.Read()
	if err != nil || settings.RateLimit != 5 || len(settings.SingleWriters) != 2 || settings.Limited[1] != "POST /api/v1/identity/session/refresh" {
		t.Fatalf("got %+v, %v", settings, err)
	}
}
