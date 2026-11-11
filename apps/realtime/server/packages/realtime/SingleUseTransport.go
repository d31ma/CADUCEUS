package realtime

import (
	"net/http"
	"time"
)

type SingleUseTransport struct{}

func (single SingleUseTransport) Client(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: single.transport(http.DefaultTransport)}
}

func (SingleUseTransport) transport(shared http.RoundTripper) http.RoundTripper {
	guarded, isTransport := shared.(*http.Transport)
	if !isTransport {
		return shared
	}
	own := guarded.Clone()
	own.DisableKeepAlives = true
	return own
}
