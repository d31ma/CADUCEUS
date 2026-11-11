package transport

import (
	"net/http"
	"testing"
	"time"
)

type otherRoundTripper struct{}

func (otherRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return nil, nil
}

func TestClientTurnsKeepAlivesOff(t *testing.T) {
	client := SingleUseTransport{}.Client(5 * time.Second)
	own, isTransport := client.Transport.(*http.Transport)
	if !isTransport || !own.DisableKeepAlives {
		t.Fatalf("got %#v", client.Transport)
	}
}

func TestClientKeepsTheTimeout(t *testing.T) {
	if got := (SingleUseTransport{}).Client(7 * time.Second).Timeout; got != 7*time.Second {
		t.Fatalf("got %v", got)
	}
}

func TestClientNeverChangesTheSharedTransport(t *testing.T) {
	SingleUseTransport{}.Client(time.Second)
	if http.DefaultTransport.(*http.Transport).DisableKeepAlives {
		t.Fatal("the shared default transport was changed")
	}
}

func TestClientKeepsTheProxySetting(t *testing.T) {
	own := SingleUseTransport{}.Client(time.Second).Transport.(*http.Transport)
	if own.Proxy == nil {
		t.Fatal("the proxy setting was lost; the egress guard needs it")
	}
}

func TestAnUnknownRoundTripperIsUsedAsItIs(t *testing.T) {
	shared := otherRoundTripper{}
	if got := (SingleUseTransport{}).transport(shared); got != shared {
		t.Fatalf("got %#v", got)
	}
}
