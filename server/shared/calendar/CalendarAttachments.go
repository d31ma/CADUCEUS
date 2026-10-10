import "strings"

// CalendarAttachments recognise the attachments of a message that hold an
// iCalendar payload. Providers disagree about the MIME type, so the file
// name is the strongest signal: .ics, then text/calendar, then the legacy
// .vcs, then application/ics.
type CalendarAttachments struct{}

// IsCalendar reports whether an attachment holds a calendar payload.
func (attachments CalendarAttachments) IsCalendar(attachment ParsedAttachment) bool {
	return attachments.rank(attachment) >= 0
}

func (CalendarAttachments) rank(attachment ParsedAttachment) int {
	filename := strings.ToLower(attachment.Filename)
	contentType := strings.ToLower(Security{}.Trim(strings.SplitN(attachment.ContentType, ";", 2)[0]))
	switch {
	case strings.HasSuffix(filename, ".ics"):
		return 0
	case contentType == "text/calendar":
		return 1
	case strings.HasSuffix(filename, ".vcs"):
		return 2
	case contentType == "application/ics":
		return 3
	}
	return -1
}
