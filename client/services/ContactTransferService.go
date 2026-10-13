import (
	"encoding/json"
	"strconv"
	"strings"
)

// ContactTransferService imports and exports vCards. ContactsBrowserService
// reads the chosen file; the export downloads through a hidden link that this
// service fills and BrowserPageService clicks. The browser transport sends
// at most 64 KiB, so a larger vCard file cannot be imported yet.
// @Service
type ContactTransferService struct {
	// @Needs
	contactsClient            *ContactsClient
	applicationEventPublisher ApplicationEventPublisher
	DownloadHref              string
	DownloadName              string
	token                     string
	checked                   []string
	groupFilter               string
}

// HearSession keeps the bearer token.
// @OnEvent({"replay":true})
func (transfer *ContactTransferService) HearSession(changed SessionChanged) {
	transfer.token = changed.Token
}

// HearShown follows the checked contacts and the chosen group.
// @OnEvent({"replay":true})
func (transfer *ContactTransferService) HearShown(shown ContactsShown) {
	transfer.checked = shown.Checked
	transfer.groupFilter = shown.GroupFilter
}

// HearFile imports the chosen vCard file into the chosen group.
// @OnEvent
func (transfer *ContactTransferService) HearFile(chosen ContactsFileChosen) {
	body := map[string]any{"content": chosen.Content, "groupIds": transfer.importGroups()}
	encoded, _ := json.Marshal(body)
	FlowUtils{}.Either(len(encoded) <= transfer.largestRequest(), func() { transfer.importBody(body) }, transfer.tooLarge)()
}

// PickImport opens the file picker.
func (transfer *ContactTransferService) PickImport() {
	transfer.applicationEventPublisher.PublishEvent(ClickRequested{Selector: "[data-contact-import]"}, false)
}

// Export downloads the checked contacts, or every contact when none is
// checked, as one vCard file.
func (transfer *ContactTransferService) Export() error {
	answer, failure := transfer.contactsClient.Export(transfer.bearer(), map[string][]string{"ids": append([]string{}, transfer.checked...)})
	transfer.afterExport(answer, failure)
	return nil
}

func (transfer *ContactTransferService) bearer() string {
	return "Bearer " + transfer.token
}

func (transfer *ContactTransferService) importGroups() []string {
	if transfer.groupFilter == "" {
		return []string{}
	}
	return []string{transfer.groupFilter}
}

func (transfer *ContactTransferService) importBody(body map[string]any) {
	answer, failure := transfer.contactsClient.Import(transfer.bearer(), body)
	transfer.afterImport(answer, failure)
}

func (transfer *ContactTransferService) afterImport(answer ContactImport, failure error) {
	if failure != nil {
		transfer.toast(HTTPUtils{}.Message(failure, "vCard import failed.", transfer.offline()))
		return
	}
	transfer.applicationEventPublisher.PublishEvent(ContactsImported{Imported: answer.Imported}, false)
	transfer.toast(transfer.importedLabel(answer))
}

func (ContactTransferService) importedLabel(answer ContactImport) string {
	skipped := map[bool]string{false: "; skipped " + strconv.Itoa(answer.Skipped)}[answer.Skipped == 0]
	return ContactTransferService{}.sentence("Imported " + ContactViewUtils{}.CountLabel(answer.Imported) + skipped)
}

func (transfer *ContactTransferService) tooLarge() {
	transfer.toast("This vCard file is larger than 64 KiB, which cannot be imported here yet.")
}

// afterExport fills the hidden download link and asks the page to click it.
func (transfer *ContactTransferService) afterExport(answer ContactExport, failure error) {
	if (HTTPUtils{}).Missed(failure, answer.ContentBase64) {
		transfer.toast(HTTPUtils{}.Message(failure, "vCard export failed.", transfer.offline()))
		return
	}
	contentType := strings.ReplaceAll(TextUtils{}.FirstOf(answer.ContentType, "text/vcard;charset=utf-8"), " ", "")
	transfer.DownloadHref = "data:" + contentType + ";base64," + answer.ContentBase64
	transfer.DownloadName = TextUtils{}.FirstOf(answer.Filename, "contacts.vcf")
	transfer.applicationEventPublisher.PublishEvent(ClickRequested{Selector: "[data-contact-download]"}, false)
	transfer.toast(transfer.sentence("Exported " + ContactViewUtils{}.CountLabel(answer.Count)))
}

func (transfer *ContactTransferService) toast(message string) {
	transfer.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (ContactTransferService) sentence(text string) string {
	return text + "."
}

func (ContactTransferService) offline() string {
	return "The server could not be reached. Check your connection and try again."
}

// largestRequest is the most the browser transport sends in one request.
func (ContactTransferService) largestRequest() int { return 64 * 1024 }
