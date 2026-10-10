import (
	"net/url"
	"strings"
)

// MailLayoutService is the split view of the inbox and folder pages: the list
// on one side, the selected message's reader on the other.
//
// @Service
type MailLayoutService struct {
	SignedIn     bool
	SelectedMail string
}

// @OnEvent({"replay":true})
func (layout *MailLayoutService) SessionChanged(session SessionChanged) {
	layout.SignedIn = session.Token != ""
}

// @OnEvent
func (layout *MailLayoutService) MailSelected(selected MailSelected) {
	layout.SelectedMail = selected.EmailID
}

// @OnEvent
func (layout *MailLayoutService) MailClosed(_ MailClosed) {
	layout.SelectedMail = ""
}

// FolderFor is the folder a /folder/<name> page shows, and "" elsewhere.
func (layout *MailLayoutService) FolderFor(path string) string {
	name, found := strings.CutPrefix(path, "/folder/")
	return layout.decoded(name, found)
}

func (MailLayoutService) decoded(name string, found bool) string {
	decoded, failure := url.PathUnescape(name)
	if !found || failure != nil {
		return ""
	}
	return decoded
}
