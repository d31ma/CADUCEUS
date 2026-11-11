package notify

import (
	"encoding/base64"
	"testing"
)

func TestCursorRoundTrip(t *testing.T) {
	createdAt, id, err := Cursor{}.Decode(Cursor{}.Encode("2026-11-10T12:00:00Z", "n-1"))
	expectNoError(t, err)
	expectSame(t, []string{createdAt, id}, []string{"2026-11-10T12:00:00Z", "n-1"})
}

func TestCursorIsUrlSafe(t *testing.T) {
	expectSame(t, Cursor{}.Encode("??>>", "~~"), "Pz8-Pgp-fg")
}

func TestCursorRefusesBadBase64(t *testing.T) {
	_, _, err := Cursor{}.Decode("not base64!")
	expectInvalidPage(t, err)
}

func TestCursorRefusesMissingSeparator(t *testing.T) {
	_, _, err := Cursor{}.Decode(base64.RawURLEncoding.EncodeToString([]byte("2026")))
	expectInvalidPage(t, err)
}

func TestCursorRefusesEmptyTime(t *testing.T) {
	_, _, err := Cursor{}.Decode(base64.RawURLEncoding.EncodeToString([]byte("\nid")))
	expectInvalidPage(t, err)
}
