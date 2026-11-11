package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func quickRetry() RetryPolicy {
	return RetryPolicy{Delays: []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}}
}

func countingServer(t *testing.T, statuses ...int) (*httptest.Server, *atomic.Int32) {
	calls := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		index := int(calls.Add(1)) - 1
		status := statuses[min(index, len(statuses)-1)]
		writer.WriteHeader(status)
		if status == http.StatusOK {
			writer.Write([]byte(`{"domainId":"example.com","expiresAt":"2026-11-11T00:00:00Z"}`))
		}
	}))
	t.Cleanup(server.Close)
	return server, calls
}

func TestMessageRetriesAnUnavailableBackend(t *testing.T) {
	server, calls := countingServer(t, http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK)
	client := NewRetryingClient(server.URL, "secret", time.Second, quickRetry())
	reply, err := client.Message(context.Background(), MessageRequest{ConnectionId: "c1"})
	if err != nil || reply.DomainId != "example.com" || calls.Load() != 3 {
		t.Fatalf("got %+v %v after %d calls", reply, err, calls.Load())
	}
}

func TestMessageGivesUpAfterThreeRetries(t *testing.T) {
	server, calls := countingServer(t, http.StatusBadGateway)
	client := NewRetryingClient(server.URL, "secret", time.Second, quickRetry())
	_, err := client.Message(context.Background(), MessageRequest{ConnectionId: "c1"})
	if err == nil || calls.Load() != 4 {
		t.Fatalf("got %v after %d calls", err, calls.Load())
	}
}

func TestMessageDoesNotRetryARefusal(t *testing.T) {
	server, calls := countingServer(t, http.StatusBadRequest, http.StatusOK)
	client := NewRetryingClient(server.URL, "secret", time.Second, quickRetry())
	_, err := client.Message(context.Background(), MessageRequest{ConnectionId: "c1"})
	if err == nil || calls.Load() != 1 {
		t.Fatalf("got %v after %d calls", err, calls.Load())
	}
}

func TestTooManyRequestsAndTimeoutsAreRetried(t *testing.T) {
	server, calls := countingServer(t, http.StatusTooManyRequests, http.StatusRequestTimeout, http.StatusNoContent)
	client := NewRetryingClient(server.URL, "secret", time.Second, quickRetry())
	err := client.Disconnect(context.Background(), DisconnectRequest{ConnectionId: "c1"})
	if err != nil || calls.Load() != 3 {
		t.Fatalf("got %v after %d calls", err, calls.Load())
	}
}

func TestANetworkFailureIsRetried(t *testing.T) {
	client := NewRetryingClient("http://127.0.0.1:1", "secret", time.Second, quickRetry())
	err := client.Connect(context.Background(), ConnectRequest{ConnectionId: "c1", Kind: "socket"})
	if err == nil || !quickRetry().Retryable(err) {
		t.Fatalf("got %v", err)
	}
}

func TestACancelledWaitStopsRetrying(t *testing.T) {
	server, calls := countingServer(t, http.StatusServiceUnavailable)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := NewRetryingClient(server.URL, "secret", time.Second, RetryPolicy{Delays: []time.Duration{time.Hour}})
	err := client.Connect(ctx, ConnectRequest{ConnectionId: "c1", Kind: "socket"})
	if err == nil || calls.Load() > 1 {
		t.Fatalf("got %v after %d calls", err, calls.Load())
	}
}

func TestDefaultPolicyFitsInsideTheAuthWindow(t *testing.T) {
	total := time.Duration(0)
	for _, delay := range DefaultRetryPolicy().Delays {
		total += delay
	}
	if total >= 10*time.Second || len(DefaultRetryPolicy().Delays) != 3 {
		t.Fatalf("total %s", total)
	}
}
