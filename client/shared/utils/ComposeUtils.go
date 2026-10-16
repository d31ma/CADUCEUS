import (
	"encoding/json"
	"net/url"
	"slices"
)

// ComposeUtils hands a prefill to the composer the way the shell's compose()
// did: the fields go to sessionStorage as caduceus_compose_prefill and then
// /compose opens. A blank composer removes the key.
type ComposeUtils struct{}

// Stored is the storage write for these composer fields.
func (ComposeUtils) Stored(fields ComposeFields) StorageWritten {
	encoded, _ := json.Marshal(ComposeUtils{}.compact(fields))
	key := ComposeUtils{}.key()
	return StorageWritten{Area: ComposeUtils{}.area(), Values: map[string]string{key: string(encoded)}, Removed: []string{}}
}

// compact stores genuine reader quote text once, preserving the public Body contract.
func (ComposeUtils) compact(fields ComposeFields) ComposeFields {
	if fields.Quote != nil && slices.Contains([]string{"reply", "forward"}, fields.Quote.Mode) && fields.Body == (QuotedMessageUtils{}).Html(*fields.Quote) {
		fields.Body = ""
	}
	return fields
}

// Cleared is the storage write for a blank composer.
func (ComposeUtils) Cleared() StorageWritten {
	return StorageWritten{Area: ComposeUtils{}.area(), Values: map[string]string{}, Removed: []string{ComposeUtils{}.key()}}
}

// Opened is the navigation to the composer.
func (ComposeUtils) Opened() NavigationRequested {
	return NavigationRequested{Target: "/compose"}
}

// Message is the navigation to a message's own page.
func (ComposeUtils) Message(emailID string) NavigationRequested {
	return NavigationRequested{Target: "/email/" + url.PathEscape(emailID)}
}

func (ComposeUtils) key() string { return "caduceus_compose_prefill" }

func (ComposeUtils) area() string { return "session" }
