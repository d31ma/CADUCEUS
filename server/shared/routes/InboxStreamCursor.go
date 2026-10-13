import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
)

// InboxStreamCursor is the opaque two-source cursor of the inbox stream:
// hosted and connected-account mail advance independently, so moving one
// never skips a gap in the other. Its text form is "v1." and the base64url
// of its JSON, as the JavaScript stream wrote it; a bare ISO time (the
// legacy form) starts both sources there.
type InboxStreamCursor struct {
	Hosted   InboxStreamPosition `json:"hosted"`
	External InboxStreamPosition `json:"external"`
}

func (InboxStreamCursor) longestId() int { return 2048 }

// Parsed answers the cursor a client sent.
func (cursor InboxStreamCursor) Parsed(value string) InboxStreamCursor {
	if encoded, isVersioned := strings.CutPrefix(value, "v1."); isVersioned {
		if decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(strings.NewReplacer("+", "-", "/", "_").Replace(encoded), "=")); err == nil {
			var fields map[string]any
			if json.Unmarshal(decoded, &fields) == nil && fields != nil {
				return InboxStreamCursor{Hosted: cursor.valid(fields["hosted"]), External: cursor.valid(fields["external"])}
			}
		}
	}
	source := cursor.position(value)
	return InboxStreamCursor{Hosted: source, External: source}
}

// Text answers the cursor's opaque text form.
func (cursor InboxStreamCursor) Text() string {
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(cursor)
	return "v1." + base64.RawURLEncoding.EncodeToString(bytes.TrimRight(encoded.Bytes(), "\n"))
}

// After reports whether an email comes after a source position.
func (InboxStreamCursor) After(email map[string]any, position InboxStreamPosition) bool {
	at := TextValue{}.Of(TextValue{}.Or(email["receivedAt"], ""))
	return at > position.At || at == position.At && TextValue{}.Of(TextValue{}.Or(email["id"], "")) > position.Id
}

// ReplaySince answers the listing bound just before a position's time, so
// mail stamped in the same millisecond is listed again and judged by id.
func (InboxStreamCursor) ReplaySince(at string) string {
	parsed := EpochTime{}.Parse(at)
	if parsed != parsed {
		return at
	}
	return EpochTime{}.Text(int64(parsed) - 1)
}

// position answers a source position at an ISO time, or at now.
func (InboxStreamCursor) position(value string) InboxStreamPosition {
	moment := EpochTime{}
	if parsed := moment.Parse(value); parsed == parsed {
		return InboxStreamPosition{At: moment.Text(int64(parsed))}
	}
	return InboxStreamPosition{At: moment.NowText()}
}

func (cursor InboxStreamCursor) valid(value any) InboxStreamPosition {
	fields, _ := value.(map[string]any)
	at, atIsText := fields["at"].(string)
	id, idIsText := fields["id"].(string)
	parsed := EpochTime{}.Parse(at)
	if !atIsText || parsed != parsed || !idIsText || (TextValue{}).Length(id) > cursor.longestId() {
		return cursor.position("")
	}
	return InboxStreamPosition{At: EpochTime{}.Text(int64(parsed)), Id: id}
}
