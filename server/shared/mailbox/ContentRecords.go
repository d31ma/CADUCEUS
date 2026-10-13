import (
	"crypto/rand"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

// ContentRecords holds what signature and template records share: their
// generated ids, their ISO timestamps and their order by name.
type ContentRecords struct{}

// NewId answers "<prefix>-<base-36 time>-<8 random base-36 digits>".
func (ContentRecords) NewId(prefix string) string {
	digits := "0123456789abcdefghijklmnopqrstuvwxyz"
	var random strings.Builder
	for index := 0; index < 8; index++ {
		pick, _ := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		random.WriteByte(digits[pick.Int64()])
	}
	return prefix + "-" + strconv.FormatInt(EpochTime{}.Now(), 36) + "-" + random.String()
}

// Iso answers new Date(value).toISOString() for stored milliseconds or text.
func (ContentRecords) Iso(value any) string {
	if number, isNumber := (RecordValues{}).Number(value); isNumber {
		return EpochTime{}.Text(int64(number))
	}
	parsed := EpochTime{}.Parse(TextValue{}.Of(value))
	if math.IsNaN(parsed) {
		return ""
	}
	return EpochTime{}.Text(int64(parsed))
}

// Timestamps answers the createdAt and updatedAt (createdAt when never
// updated) of a record as ISO text.
func (records ContentRecords) Timestamps(entry map[string]any) (string, string) {
	return records.Iso(entry["createdAt"]), records.Iso(RecordValues{}.Or(entry["updatedAt"], entry["createdAt"]))
}

// ByName answers the records' fields sorted by name.
func (ContentRecords) ByName(records []FyloRecord) []map[string]any {
	entries := StoredRecords{}.Listed(records)
	sort.SliceStable(entries, func(left, right int) bool {
		return TextOrder{}.Compare(TextValue{}.Of(entries[left]["name"]), TextValue{}.Of(entries[right]["name"])) < 0
	})
	return entries
}

// Merged answers entry with patch applied, as {...entry, ...patch}.
func (ContentRecords) Merged(entry map[string]any, patch map[string]any) map[string]any {
	merged := TextValue{}.Copy(entry)
	for key, value := range patch {
		merged[key] = value
	}
	return merged
}

// Viewed answers the named fields a record has plus its ISO createdAt and
// updatedAt.
func (records ContentRecords) Viewed(entry map[string]any, fields ...string) map[string]any {
	createdAt, updatedAt := records.Timestamps(entry)
	view := map[string]any{"createdAt": createdAt, "updatedAt": updatedAt}
	for _, field := range fields {
		if value, present := entry[field]; present {
			view[field] = value
		}
	}
	return view
}
