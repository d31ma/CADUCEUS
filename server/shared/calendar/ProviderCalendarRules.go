import (
	"crypto/sha256"
	"encoding/base64"
)

// ProviderCalendarRules hold the limits and identities of events that come
// from a provider's CalDAV calendars.
type ProviderCalendarRules struct{}

// Merged answers the local events and the provider events, without the
// imported invitation copies of events the provider already has.
func (ProviderCalendarRules) Merged(local []map[string]any, provider []map[string]any) []map[string]any {
	uids := map[string]bool{}
	for _, event := range provider {
		if uid := (RecordValues{}).Text(event["icsUid"]); uid != "" {
			uids[uid] = true
		}
	}
	merged := []map[string]any{}
	for _, event := range local {
		if event["source"] != "invite" || !uids[(RecordValues{}).Text(event["icsUid"])] {
			merged = append(merged, event)
		}
	}
	return append(merged, provider...)
}

// EventId answers the stable id of a provider event: a digest of its
// resource and UID.
func (rules ProviderCalendarRules) EventId(resourceId string, uid string) string {
	return "provider-" + rules.ShortHash(resourceId+"\x00"+uid)
}

// ShortHash answers the first 24 characters of a value's SHA-256 digest in
// unpadded base64url.
func (ProviderCalendarRules) ShortHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return base64.RawURLEncoding.EncodeToString(digest[:])[:24]
}

func (ProviderCalendarRules) MostResources() int { return 10_000 }

func (ProviderCalendarRules) MostEvents() int { return 20_000 }

func (ProviderCalendarRules) LargestResource() int { return 2 << 20 }
