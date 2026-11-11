package realtime

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestTheClientKeepsTheGuardProxyButNeverReusesAConnection(t *testing.T) {
	guard, _ := url.Parse("http://127.0.0.1:9")
	shared := &http.Transport{Proxy: http.ProxyURL(guard)}
	own := SingleUseTransport{}.transport(shared).(*http.Transport)
	request, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:8117/@connections/c1", nil)
	proxy, _ := own.Proxy(request)
	if !own.DisableKeepAlives || proxy == nil || proxy.Host != "127.0.0.1:9" || own == shared {
		t.Fatalf("got keep-alives off=%v proxy=%v", own.DisableKeepAlives, proxy)
	}
}

func TestAnUnknownRoundTripperIsUsedAsItIs(t *testing.T) {
	other := http.RoundTripper(roundTripperFunc(func(*http.Request) (*http.Response, error) { return nil, nil }))
	if (SingleUseTransport{}).transport(other) == nil {
		t.Fatal("no transport")
	}
}

func TestTheClientHasTheTimeout(t *testing.T) {
	if (SingleUseTransport{}).Client(3*time.Second).Timeout != 3*time.Second {
		t.Fatal("wrong timeout")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (function roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
