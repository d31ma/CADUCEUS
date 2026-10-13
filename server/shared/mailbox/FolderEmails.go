import "strings"

// FolderEmails walks one folder of the allowed domains through FYLO's
// bounded query pages, answering normalised emails the mailbox may see. A
// cursor failure ends the walk with its error rather than restarting it, so
// an export never repeats messages.
type FolderEmails struct {
	Database   FyloDatabase
	Domains    []string
	Folder     string
	OwnerEmail string
	PageSize   int
	domain     int
	cursor     string
	started    bool
	pending    []map[string]any
}

// Next answers the next email, or nil once the walk is over.
func (emails *FolderEmails) Next() (map[string]any, error) {
	for len(emails.pending) == 0 {
		if emails.started && emails.cursor == "" {
			emails.domain++
			emails.started = false
		}
		if emails.domain >= len(emails.Domains) {
			return nil, nil
		}
		if err := emails.read(); err != nil {
			return nil, err
		}
	}
	email := emails.pending[0]
	emails.pending = emails.pending[1:]
	return email, nil
}

func (emails *FolderEmails) read() error {
	query := FyloCollection{}.Equal("domain", emails.Domains[emails.domain])
	page, err := emails.Database.FindDocsPage(FyloCollections{}.Named().Emails, query, emails.PageSize, emails.cursor)
	if err != nil {
		return err
	}
	emails.started = true
	emails.cursor = page.NextCursor
	records := EmailsRecords{}
	for _, document := range page.Documents {
		email := records.Normalize(FyloRecord{}.Decoded(document).Data)
		if (TextValue{}).Of(email["folder"]) == emails.Folder && records.CanAccessEmail(email, emails.OwnerEmail) {
			emails.pending = append(emails.pending, email)
		}
	}
	return nil
}

// Selected answers the folder a walk reads: the named one, lower-cased, or
// the inbox.
func (FolderEmails) Selected(folder string) string {
	if folder == "" {
		folder = "inbox"
	}
	return strings.ToLower(folder)
}
