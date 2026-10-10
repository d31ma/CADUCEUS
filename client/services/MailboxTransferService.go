import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// MailboxTransferService decides what the folder export and the EML import
// may do and words their progress. The bytes move in MailboxArchiveService
// (JavaScript): an export is a binary stream with no size bound written to a
// file, and an import uploads files of up to 50 MiB, which the HTTPClient
// transport (JSON, 64 KiB requests, 1 MiB answers) cannot carry.
//
// @Service
type MailboxTransferService struct {
	applicationEventPublisher ApplicationEventPublisher

	Exporting           bool
	Importing           bool
	ExportProgressLabel string
	ImportProgressLabel string
	ExportError         string
	ExportNotice        string
	ImportError         string
	ImportNotice        string

	scope         MailboxScopeChanged
	authorization string
	format        string
	folder        string
	exported      int
	total         int
	bytesLabel    string
	imported      int
	importTotal   int
	importCurrent string
}

// @OnEvent({"replay":true})
func (transfer *MailboxTransferService) SessionChanged(session SessionChanged) {
	transfer.authorization = "Bearer " + session.Token
}

// @OnEvent({"replay":true})
func (transfer *MailboxTransferService) MailboxScopeChanged(scope MailboxScopeChanged) {
	transfer.scope = scope
}

// ChooseImportFiles opens the file picker once one mailbox is chosen.
func (transfer *MailboxTransferService) ChooseImportFiles() {
	mailbox := transfer.importMailbox()
	transfer.ImportError = transfer.importRefusal(mailbox)
	transfer.pickWhenAllowed(mailbox)
}

// CancelImport stops the file being read or uploaded and skips the rest.
func (transfer *MailboxTransferService) CancelImport() {
	transfer.applicationEventPublisher.PublishEvent(ImportCancelRequested{Reason: transfer.personAsked()}, false)
}

func (MailboxTransferService) personAsked() string { return "person" }

// ExportFolder streams the current folder into an mbox file, or with "eml"
// into an archive of the exact EML sources. A second click cancels.
func (transfer *MailboxTransferService) ExportFolder(format string) {
	transfer.cancelWhenExporting()
	scopes := transfer.exportScopes()
	transfer.ExportError = transfer.exportRefusal(format, scopes)
	transfer.exportWhenAllowed(format, scopes)
}

// CancelExport stops the export and drops what it had written.
func (transfer *MailboxTransferService) CancelExport() {
	transfer.applicationEventPublisher.PublishEvent(ExportCancelRequested{Reason: transfer.personAsked()}, false)
}

// @OnEvent
func (transfer *MailboxTransferService) ExportProgressed(progress ExportProgressed) {
	transfer.exported = progress.Messages
	transfer.bytesLabel = progress.BytesLabel
	transfer.ExportProgressLabel = transfer.exportLabel()
}

// @OnEvent
func (transfer *MailboxTransferService) ExportFinished(finished ExportFinished) {
	transfer.Exporting = false
	transfer.ExportError = finished.Failure
	transfer.ExportNotice = transfer.exportNotice(finished)
}

// @OnEvent
func (transfer *MailboxTransferService) ImportProgressed(progress ImportProgressed) {
	transfer.Importing = true
	transfer.ImportError = ""
	transfer.ImportNotice = ""
	transfer.imported = progress.Imported
	transfer.importTotal = progress.Total
	transfer.importCurrent = progress.Current
	transfer.ImportProgressLabel = transfer.importLabel()
}

// @OnEvent
func (transfer *MailboxTransferService) ImportFinished(finished ImportFinished) {
	transfer.Importing = false
	transfer.importCurrent = ""
	transfer.ImportError = transfer.importFailure(finished)
	transfer.ImportNotice = transfer.importNotice(finished)
	transfer.reloadWhenImported(finished.Imported)
}

func (transfer *MailboxTransferService) importMailbox() string {
	domain := transfer.importDomain()
	at := "@"
	local, ownDomain, _ := strings.Cut(transfer.scope.Owner, at)
	mailbox := map[bool]string{true: transfer.scope.Owner, false: cmp.Or(local, "owner") + at + domain}[ownDomain == domain]
	return map[bool]string{true: "", false: mailbox}[domain == ""]
}

func (transfer *MailboxTransferService) importDomain() string {
	switch {
	case transfer.scope.Account != "":
		return ""
	case transfer.scope.Domain != transfer.everyMailbox():
		return transfer.scope.Domain
	case len(transfer.scope.Domains) == 1:
		return transfer.scope.Domains[0]
	default:
		return ""
	}
}

func (MailboxTransferService) everyMailbox() string { return "all" }

func (transfer *MailboxTransferService) importRefusal(mailbox string) string {
	if mailbox == "" && !transfer.Importing && !transfer.Exporting {
		return "Choose one mailbox domain before importing EML files."
	}
	return transfer.ImportError
}

func (transfer *MailboxTransferService) pickWhenAllowed(mailbox string) {
	if mailbox != "" && !transfer.Importing && !transfer.Exporting {
		transfer.applicationEventPublisher.PublishEvent(ImportPickRequested{Mailbox: mailbox, Folder: transfer.scope.Folder, Authorization: transfer.authorization}, false)
	}
}

func (transfer *MailboxTransferService) cancelWhenExporting() {
	if transfer.Exporting {
		transfer.CancelExport()
	}
}

func (transfer *MailboxTransferService) exportScopes() []ExportScope {
	domains := transfer.exportDomains()
	scopes := []ExportScope{}
	for _, domain := range domains {
		scopes = append(scopes, ExportScope{Domain: domain})
	}
	return map[bool][]ExportScope{true: {{Account: transfer.scope.Account}}, false: scopes}[transfer.scope.Account != ""]
}

func (transfer *MailboxTransferService) exportDomains() []string {
	own := transfer.scope.Owner[strings.LastIndex(transfer.scope.Owner, "@")+1:]
	selected := strings.ToLower(cmp.Or(transfer.scope.Domain, transfer.everyMailbox()))
	domains := map[bool][]string{true: transfer.scope.Domains, false: {selected}}[selected == transfer.everyMailbox()]
	domains = slices.Compact(slices.DeleteFunc(slices.Clone(domains), func(domain string) bool { return domain == "" }))
	return map[bool][]string{true: slices.DeleteFunc([]string{strings.ToLower(own)}, func(domain string) bool { return domain == "" }), false: domains}[len(domains) == 0]
}

func (transfer *MailboxTransferService) exportRefusal(format string, scopes []ExportScope) string {
	if transfer.Exporting {
		return transfer.ExportError
	} else if len(scopes) == 0 {
		return "No mailbox domain is available to export."
	} else if format == transfer.archiveFormat() && transfer.scope.Account == "" && len(scopes) != 1 {
		return "Choose one mailbox domain before exporting individual EML files."
	}
	return ""
}

func (MailboxTransferService) archiveFormat() string { return "eml" }

func (transfer *MailboxTransferService) exportWhenAllowed(format string, scopes []ExportScope) {
	if transfer.Exporting || transfer.ExportError != "" {
		return
	}
	transfer.format = map[bool]string{true: transfer.archiveFormat(), false: "mbox"}[format == transfer.archiveFormat()]
	transfer.folder = strings.ToLower(cmp.Or(transfer.scope.Folder, "inbox"))
	transfer.Exporting = true
	transfer.exported = 0
	transfer.bytesLabel = "0 B"
	transfer.ExportNotice = ""
	transfer.total = transfer.scope.Total
	transfer.ExportProgressLabel = transfer.exportLabel()
	transfer.applicationEventPublisher.PublishEvent(ExportRequested{
		Format: transfer.format, Folder: transfer.folder, Scopes: scopes, Authorization: transfer.authorization,
		FilenameDomain: cmp.Or(transfer.scope.Account, map[bool]string{true: "all-domains", false: scopes[0].Domain}[len(scopes) > 1]),
	}, false)
}

func (transfer *MailboxTransferService) exportLabel() string {
	count := map[bool]string{
		true:  fmt.Sprintf("%d of %d messages", transfer.exported, transfer.total),
		false: transfer.messages(transfer.exported),
	}[transfer.total > 0]
	return fmt.Sprintf("Exporting %s: %s · %s", map[bool]string{true: "EML archive", false: "mbox"}[transfer.format == transfer.archiveFormat()], count, transfer.bytesLabel)
}

func (transfer *MailboxTransferService) exportNotice(finished ExportFinished) string {
	switch {
	case finished.Canceled:
		return "Mailbox export canceled."
	case finished.Failure != "":
		return ""
	case finished.Messages == 0:
		return fmt.Sprintf("There are no messages in %s.", transfer.folder)
	default:
		as := map[bool]string{true: " as individual EML files", false: ""}[transfer.format == transfer.archiveFormat()]
		return fmt.Sprintf("Exported %s%s (%s source).", transfer.messages(finished.Messages), as, finished.SourceBytesLabel)
	}
}

func (transfer *MailboxTransferService) importLabel() string {
	progress := fmt.Sprintf("%d of %d imported", transfer.imported, transfer.importTotal)
	return progress + map[bool]string{true: "", false: " · " + transfer.importCurrent}[transfer.importCurrent == ""]
}

func (transfer *MailboxTransferService) importFailure(finished ImportFinished) string {
	switch {
	case finished.Canceled:
		return ""
	case finished.Failure != "":
		return finished.Failure
	case finished.Failures > 0:
		failed := map[bool]string{true: "file failed", false: "files failed"}[finished.Failures == 1]
		return fmt.Sprintf("Imported %s; %d %s. %s", transfer.messages(finished.Imported), finished.Failures, failed, finished.FirstFailure)
	default:
		return ""
	}
}

func (transfer *MailboxTransferService) importNotice(finished ImportFinished) string {
	if finished.Canceled {
		return fmt.Sprintf("EML import canceled after %s.", transfer.messages(finished.Imported))
	} else if finished.Failure != "" || finished.Failures > 0 {
		return ""
	}
	return fmt.Sprintf("Imported %s into %s.", transfer.messages(finished.Imported), transfer.scope.FolderTitle)
}

func (transfer *MailboxTransferService) reloadWhenImported(imported int) {
	if imported > 0 {
		transfer.applicationEventPublisher.PublishEvent(InboxRefreshRequested{Reason: "import"}, false)
	}
}

func (MailboxTransferService) messages(count int) string {
	return fmt.Sprintf("%d %s", count, map[bool]string{true: "message", false: "messages"}[count == 1])
}
