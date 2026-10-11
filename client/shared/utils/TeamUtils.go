import (
	"strconv"
	"strings"
	"time"
)

// TeamUtils turns the presence roster into the rows of the Team page.
type TeamUtils struct{}

// Members gives each teammate a label, avatar initials and a status text.
func (TeamUtils) Members(teammates []Teammate, now time.Time) []TeamMember {
	return TeamUtils{}.members(teammates, now)
}

func (TeamUtils) members(teammates []Teammate, now time.Time) []TeamMember {
	members := []TeamMember{}
	for _, mate := range teammates {
		members = append(members, TeamMember{
			Email: mate.Email, Name: mate.Name, Label: TextUtils{}.FirstOf(mate.Name, mate.Email), Initials: TeamUtils{}.initials(strings.TrimSpace(TextUtils{}.FirstOf(mate.Name, mate.Email))),
			Online: mate.Online, IsSelf: mate.IsSelf, Status: TeamUtils{}.status(mate, now),
		})
	}
	return members
}

// initials are two letters: the first letters of the first two name parts,
// or the first two characters of a one-part name.
func (TeamUtils) initials(source string) string {
	parts := strings.FieldsFunc(source, func(character rune) bool { return strings.ContainsRune("@ \t._-", character) })
	letters := map[bool]string{true: TeamUtils{}.first(parts, 0) + TeamUtils{}.first(parts, 1), false: string([]rune(source)[:min(TeamUtils{}.pair(), len([]rune(source)))])}[len(parts) >= TeamUtils{}.pair()]
	return strings.ToUpper(TextUtils{}.FirstOf(letters, "?"))
}

func (TeamUtils) first(parts []string, index int) string {
	return string([]rune(append(parts, "", "")[index])[:min(1, len([]rune(append(parts, "", "")[index])))])
}

// status is "Online", or how long ago the teammate was last seen.
func (TeamUtils) status(mate Teammate, now time.Time) string {
	seen, failure := time.Parse(time.RFC3339Nano, mate.LastSeenAt)
	return map[bool]string{true: "Online", false: TeamUtils{}.lastSeen(failure == nil, now.Sub(seen))}[mate.Online]
}

func (TeamUtils) lastSeen(known bool, since time.Duration) string {
	hours, days := int(since.Hours()), int(since.Hours()/TeamUtils{}.day())
	switch {
	case !known:
		return "Offline"
	case since < time.Minute:
		return "Just now"
	case since < time.Hour:
		return strconv.Itoa(int(since.Minutes())) + " min ago"
	case days < 1:
		return strconv.Itoa(hours) + "h ago"
	}
	return strconv.Itoa(days) + "d ago"
}

func (TeamUtils) pair() int { return 2 }

func (TeamUtils) day() float64 { return 24 }
