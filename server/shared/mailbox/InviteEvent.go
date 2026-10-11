import (
	"regexp"
	"strings"
)

// InviteEvent is one VEVENT of an invite: its UID and attendees, and whether
// it carried the SUMMARY and DTSTART a complete event needs.
type InviteEvent struct {
	Uid       string
	Attendees []map[string]any
	summary   bool
	start     bool
}

func (event *InviteEvent) apply(name string, parameters map[string]string, value string) {
	switch name {
	case "UID":
		event.Uid = value
	case "SUMMARY":
		event.summary = value != ""
	case "DTSTART":
		event.start = regexp.MustCompile(`^\d{8}(T\d{6}Z?)?$`).MatchString(strings.TrimSpace(value))
	case "ATTENDEE":
		email := strings.ToLower(regexp.MustCompile(`(?i)^mailto:`).ReplaceAllString(strings.TrimSpace(value), ""))
		if email == "" {
			return
		}
		statuses := map[string]string{
			"ACCEPTED": "accepted", "DECLINED": "declined", "TENTATIVE": "tentative",
			"NEEDS-ACTION": "needsAction", "DELEGATED": "delegated", "COMPLETED": "completed",
		}
		status := statuses[parameters["PARTSTAT"]]
		if status == "" {
			status = "needsAction"
		}
		event.Attendees = append(event.Attendees, map[string]any{"email": email, "name": parameters["CN"], "status": status})
	}
}
