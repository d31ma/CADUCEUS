package mime

import (
	"reflect"
	"testing"
)

func FuzzHeaders(f *testing.F) {
	for _, seed := range (testMessages{}).all() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		headers, err := Parser{}.Headers(raw)
		if err == nil && len(headers.To) > len(raw) {
			t.Fatalf("more addresses than bytes: %d", len(headers.To))
		}
	})
}

func FuzzParse(f *testing.F) {
	for _, seed := range (testMessages{}).all() {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		message, err := Parser{}.Parse(raw)
		if err != nil {
			return
		}
		headers, headersErr := Parser{}.Headers(raw)
		if headersErr != nil || !reflect.DeepEqual(message.Headers, headers) {
			t.Fatalf("headers differ: %+v, %+v, %v", message.Headers, headers, headersErr)
		}
		if len(message.Attachments) > (Parser{}).MaxParts() {
			t.Fatalf("too many attachments: %d", len(message.Attachments))
		}
	})
}
