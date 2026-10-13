import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
)

// TrackingRoute answers the public tracking capabilities: /track/open, which
// records a valid open and always answers the same pixel, and /track/click,
// which records a valid click and redirects only to the stored original.
type TrackingRoute struct {
	Tracking *TrackingHelper
}

// Open answers GET /track/open.
func (route TrackingRoute) Open(request *http.Request) *http.Response {
	tokens := TrackingTokens{}
	capability, valid := tokens.Read(request.URL.Query().Get("t"), tokens.open(), EpochTime{}.Now())
	if valid {
		err := ProcessLock{Name: "tracking"}.Held(func() error {
			_, err := route.Tracking.RecordOpen(capability.TrackingId, capability.Recipient)
			return err
		})
		if err != nil {
			log.Printf("[track/open] Failed to process tracking capability: %v", err)
		}
	}
	return route.pixel()
}

// Click answers GET /track/click.
func (route TrackingRoute) Click(request *http.Request) *http.Response {
	tokens := TrackingTokens{}
	capability, valid := tokens.Read(request.URL.Query().Get("t"), tokens.click(), EpochTime{}.Now())
	if !valid {
		return route.invalidLink()
	}
	recorded := false
	err := ProcessLock{Name: "tracking"}.Held(func() error {
		var err error
		recorded, err = route.Tracking.RecordClick(TrackingClick{TrackingId: capability.TrackingId, Recipient: capability.Recipient, Url: capability.Url, LinkIndex: float64(capability.LinkIndex)})
		return err
	})
	if err != nil {
		log.Printf("[track/click] Failed to process tracking capability: %v", err)
	}
	if err != nil || !recorded {
		return route.invalidLink()
	}
	// Tachyon writes a response without a content type as JSON, so the
	// empty redirect body is marked as text.
	header := http.Header{}
	header.Set("Content-Type", "text/plain; charset=utf-8")
	header.Set("Location", capability.Url)
	header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	return &http.Response{StatusCode: http.StatusFound, Header: header, Body: io.NopCloser(strings.NewReader(""))}
}

func (route TrackingRoute) invalidLink() *http.Response {
	header := http.Header{}
	header.Set("Content-Type", "text/plain; charset=utf-8")
	header.Set("Cache-Control", "no-store")
	header.Set("Content-Security-Policy", "default-src 'none'")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	body := "This tracked link is invalid or has expired."
	return &http.Response{StatusCode: http.StatusNotFound, Header: header, Body: io.NopCloser(strings.NewReader(body)), ContentLength: int64(len(body))}
}

// pixel answers the 1x1 transparent GIF (43 bytes) every open request gets.
func (TrackingRoute) pixel() *http.Response {
	gif := []byte{
		0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00, 0xFF, 0xFF, 0xFF,
		0x00, 0x00, 0x00, 0x21, 0xF9, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x2C, 0x00, 0x00, 0x00, 0x00,
		0x01, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x44, 0x01, 0x00, 0x3B,
	}
	header := http.Header{}
	header.Set("Content-Type", "image/gif")
	header.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	header.Set("Content-Security-Policy", "default-src 'none'")
	header.Set("Cross-Origin-Resource-Policy", "cross-origin")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Pragma", "no-cache")
	header.Set("Expires", "0")
	return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(bytes.NewReader(gif)), ContentLength: int64(len(gif))}
}
