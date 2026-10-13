import (
	"fmt"
	"regexp"
	"strings"
)

// ContentNames checks and de-duplicates the names of a mailbox's signatures
// and templates.
type ContentNames struct{}

// Normalized answers the trimmed name with runs of white space collapsed, or
// why it is refused.
func (ContentNames) Normalized(value string) (string, string) {
	name := TextValue{}.Collapsed(Security{}.Trim(value))
	if name == "" {
		return "", "name is required"
	}
	if (TextValue{}).Length(name) > 100 || !regexp.MustCompile(`^[\p{L}\p{N}_\- ]+$`).MatchString(name) {
		return "", "name may contain letters, numbers, spaces, dashes, and underscores (100 characters maximum)"
	}
	return name, ""
}

// Unique answers requested, or requested with the first free " N" suffix
// when another record (not excludeId) already has that name, ignoring case.
func (ContentNames) Unique(requested string, records []FyloRecord, excludeId string) string {
	used := map[string]bool{}
	for _, record := range records {
		if record.Data["id"] != excludeId {
			used[strings.ToLower(TextValue{}.Of(RecordValues{}.Or(record.Data["name"], "")))] = true
		}
	}
	if !used[strings.ToLower(requested)] {
		return requested
	}
	suffix := 1
	for used[strings.ToLower(fmt.Sprintf("%s %d", requested, suffix))] {
		suffix++
	}
	return fmt.Sprintf("%s %d", requested, suffix)
}
