import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
)

// ExternalMailboxService shows the connected (provider) mailboxes in the
// sidebar: their folder trees, colours, order, pause and sync controls, and
// the dialog that queues a folder create, rename or delete at the provider.
// @Service
type ExternalMailboxService struct {
	// @Needs
	externalAccountsClient *ExternalAccountsClient
	applicationEventPublisher ApplicationEventPublisher
	Accounts                  []ExternalAccount
	DialogOpen                bool
	DialogMode                string
	DialogName                string
	DialogError               string
	DialogBusy                bool
	DialogTitle               string
	DialogDescription         string
	DialogAction              string
	folders                   map[string][]ExternalFolder
	open                      map[string]bool
	dialogAccount             string
	dialogFolder              string
	busy                      bool
	token                     string
}

// @OnStart
func (external *ExternalMailboxService) Start() {
	external.Accounts = []ExternalAccount{}
	external.folders = map[string][]ExternalFolder{}
	external.open = map[string]bool{}
}

// HearStorage restores which provider folders are expanded on this device.
// @OnEvent({"replay":true})
func (external *ExternalMailboxService) HearStorage(stored StoredValues) {
	json.Unmarshal([]byte(stored.Local[external.openKey()]), &external.open)
}

// HearSession loads the connected mailboxes of the signed-in user.
// @OnEvent({"replay":true})
func (external *ExternalMailboxService) HearSession(changed SessionChanged) {
	external.token = changed.Token
	FlowUtils{}.When(changed.SignedIn, external.load)()
}

// HearToastAction refreshes the folders after a queued folder change.
// @OnEvent
func (external *ExternalMailboxService) HearToastAction(chosen ToastActionChosen) {
	FlowUtils{}.When(chosen.Action == external.refreshAction(), external.load)()
}

// HearEscape closes the folder dialog.
// @OnEvent
func (external *ExternalMailboxService) HearEscape(pressed EscapePressed) {
	external.CloseDialog()
}

// HearBackdrop closes the folder dialog on a click beside it.
// @OnEvent
func (external *ExternalMailboxService) HearBackdrop(clicked BackdropClicked) {
	FlowUtils{}.When(clicked.Dialog == external.folderForm(), external.CloseDialog)()
}

// HearSubmit submits the folder dialog's form.
// @OnEvent
func (external *ExternalMailboxService) HearSubmit(submitted FormSubmitted) {
	FlowUtils{}.When(submitted.Form == external.folderForm(), external.SubmitDialog)()
}

// FoldersFor lists the visible folder rows of one connected account.
func (external *ExternalMailboxService) FoldersFor(accountID string) []ExternalFolder {
	return ExternalFolderUtils{}.Visible(external.folders[accountID], external.open, external.account(accountID))
}

// ToggleFolder expands or collapses a provider folder that has children.
func (external *ExternalMailboxService) ToggleFolder(accountID string, stableID string) {
	key := ExternalFolderUtils{}.OpenKey(accountID, stableID)
	external.open[key] = !external.open[key]
	encoded, _ := json.Marshal(external.open)
	external.applicationEventPublisher.PublishEvent(StorageWritten{Area: "local", Values: map[string]string{external.openKey(): string(encoded)}, Removed: []string{}}, false)
}

// ChangeColor saves a new colour for a connected account. The picker reports
// a choice more than once (input, then change); the same colour again is not
// another update.
func (external *ExternalMailboxService) ChangeColor(accountID string, color string) {
	account := external.account(accountID)
	external.guarded(FlowUtils{}.When(!slices.Contains([]bool{regexp.MustCompile(`^#[0-9a-fA-F]{6}$`).MatchString(color), !strings.EqualFold(color, account.Color)}, false), func() {
		external.update(account, external.colored(account.Revision, color), "Account color updated.")
	}))
}

// MoveUp moves a connected account one place up.
func (external *ExternalMailboxService) MoveUp(accountID string) {
	external.guarded(external.mover(accountID, -1))
}

// MoveDown moves a connected account one place down.
func (external *ExternalMailboxService) MoveDown(accountID string) {
	external.guarded(external.mover(accountID, 1))
}

// TogglePause pauses or resumes a connected account.
func (external *ExternalMailboxService) TogglePause(accountID string) {
	external.guarded(func() { external.pause(external.account(accountID)) })
}

// Retry schedules an immediate synchronization of a connected account.
func (external *ExternalMailboxService) Retry(accountID string) {
	external.guarded(func() { external.retry(external.account(accountID)) })
}

// OpenDialog opens the folder dialog to create, rename or delete a folder.
func (external *ExternalMailboxService) OpenDialog(mode string, accountID string, stableID string) {
	folder := external.folder(accountID, stableID)
	account := external.account(TextUtils{}.FirstOf(folder.AccountID, accountID))
	FlowUtils{}.When(external.allowed(mode, account, folder), func() { external.openFor(mode, account.ID, folder) })()
}

// CloseDialog closes the folder dialog unless a change is being queued.
func (external *ExternalMailboxService) CloseDialog() {
	FlowUtils{}.When(external.closable(), external.closeDialog)()
}

// SubmitDialog queues the dialog's folder change at the provider.
func (external *ExternalMailboxService) SubmitDialog() {
	FlowUtils{}.When(!external.DialogBusy, external.submit)()
}

// load reads the redacted connected-account identities and the folder
// catalog of each readable account. A provider that is down leaves its
// account without folders; the hosted mailbox stays usable.
func (external *ExternalMailboxService) load() {
	list, failure := external.externalAccountsClient.Accounts(external.bearer())
	accounts := ExternalAccountUtils{}.Normalize(list.Accounts)
	FlowUtils{}.When(failure == nil, func() { external.adopt(accounts, external.foldersOf(accounts)) })()
}

func (external *ExternalMailboxService) foldersOf(accounts []ExternalAccount) map[string][]ExternalFolder {
	folders := map[string][]ExternalFolder{}
	for _, account := range accounts {
		folders[account.ID] = external.readableFolders(account, ExternalAccountUtils{}.Readable(account))
	}
	return folders
}

func (external *ExternalMailboxService) readableFolders(account ExternalAccount, readable bool) []ExternalFolder {
	list := ExternalFolderList{}
	FlowUtils{}.When(readable, func() { list, _ = external.externalAccountsClient.Folders(external.bearer(), account.ID) })()
	return ExternalFolderUtils{}.Normalize(list.Folders, account.ID)
}

func (external *ExternalMailboxService) adopt(accounts []ExternalAccount, folders map[string][]ExternalFolder) {
	external.Accounts = ExternalAccountUtils{}.Arranged(accounts)
	external.folders = folders
	external.applicationEventPublisher.PublishEvent(ExternalMailboxesChanged{Accounts: external.Accounts, Folders: external.folders}, true)
}

func (external *ExternalMailboxService) guarded(change func()) {
	FlowUtils{}.When(!external.busy, func() {
		external.busy = true
		change()
		external.busy = false
	})()
}

// update sends a map, not a model: Tac sends every field of a model, empty
// ones included, and a colour or order change must carry only its own field.
func (external *ExternalMailboxService) update(account ExternalAccount, update map[string]any, message string) {
	updated, failure := external.externalAccountsClient.Update(external.bearer(), account.ID, update)
	external.settled(failure, updated.ID, "Could not update connected account.", func() { external.replaced([]ExternalAccount{updated}, message) })()
}

// change is the body of an account change: its action and the revision it
// applies to. Colour and order changes add only their own field.
func (ExternalMailboxService) change(action string, revision int) map[string]any {
	return map[string]any{"action": action, "revision": revision}
}

func (external *ExternalMailboxService) colored(revision int, color string) map[string]any {
	body := external.change(external.updateAction(), revision)
	body["color"] = color
	return body
}

func (external *ExternalMailboxService) ordered(revision int, order int) map[string]any {
	body := external.change(external.updateAction(), revision)
	body["order"] = order
	return body
}

// mover swaps the order of two neighbouring accounts.
func (external *ExternalMailboxService) mover(accountID string, direction int) func() {
	index := slices.IndexFunc(external.Accounts, func(account ExternalAccount) bool { return account.ID == accountID })
	return FlowUtils{}.When(!slices.Contains([]bool{index >= 0, index+direction >= 0, index+direction < len(external.Accounts)}, false), func() {
		external.swap(external.Accounts[index], external.Accounts[index+direction])
	})
}

func (external *ExternalMailboxService) swap(account ExternalAccount, neighbor ExternalAccount) {
	moved, failure := external.externalAccountsClient.Update(external.bearer(), neighbor.ID, external.ordered(neighbor.Revision, account.Order))
	external.settled(failure, moved.ID, external.reorderFailure(), func() { external.secondMove(account, neighbor, moved) })()
}

// secondMove moves the account into its neighbour's place, and gives the
// neighbour its old order back when this second write fails.
func (external *ExternalMailboxService) secondMove(account ExternalAccount, neighbor ExternalAccount, movedNeighbor ExternalAccount) {
	moved, failure := external.externalAccountsClient.Update(external.bearer(), account.ID, external.ordered(account.Revision, neighbor.Order))
	FlowUtils{}.When(HTTPUtils{}.Missed(failure, moved.ID), func() {
		external.externalAccountsClient.Update(external.bearer(), movedNeighbor.ID, external.ordered(movedNeighbor.Revision, neighbor.Order))
	})()
	external.settled(failure, moved.ID, external.reorderFailure(), func() {
		external.replaced([]ExternalAccount{movedNeighbor, moved}, "Account order updated.")
	})()
}

func (external *ExternalMailboxService) pause(account ExternalAccount) {
	action := external.pauseAction(account.State)
	updated, failure := external.externalAccountsClient.Update(external.bearer(), account.ID, external.change(action, account.Revision))
	external.settled(failure, updated.ID, external.couldNot()+action+" connected account.", func() {
		external.replaced([]ExternalAccount{updated}, external.pauseNotice(action))
	})()
}

func (external *ExternalMailboxService) retry(account ExternalAccount) {
	answer, failure := external.externalAccountsClient.Retry(external.bearer(), account.ID, ExternalRetry{Revision: account.Revision})
	external.settled(failure, external.scheduled(answer.Scheduled), "Could not synchronize connected account.", func() {
		external.toast(ToastRequested{Message: "Account synchronization scheduled."})
	})()
}

// settled is the next step after a change: the change's own step when it
// landed, and otherwise a reload of the accounts with the failure shown.
func (external *ExternalMailboxService) settled(failure error, answered string, message string, next func()) func() {
	return FlowUtils{}.Either(HTTPUtils{}.Missed(failure, answered), func() {
		external.load()
		external.toast(ToastRequested{Message: message})
	}, next)
}

func (external *ExternalMailboxService) replaced(updates []ExternalAccount, message string) {
	external.adopt(ExternalAccountUtils{}.Replaced(external.Accounts, updates), external.folders)
	external.toast(ToastRequested{Message: message})
}

func (external *ExternalMailboxService) openFor(mode string, accountID string, folder ExternalFolder) {
	external.dialogAccount = accountID
	external.dialogFolder = folder.StableID
	external.DialogMode = mode
	external.DialogName = external.startingName(mode, folder)
	external.DialogError = ""
	external.DialogOpen = true
	external.describe(folder)
	external.applicationEventPublisher.PublishEvent(FocusRequested{Selector: external.focusTarget(mode), Remember: true}, false)
}

func (external *ExternalMailboxService) describe(folder ExternalFolder) {
	named := TextUtils{}.FirstOf(folder.DisplayName, TextUtils{}.FirstOf(folder.Name, "this folder"))
	texts := ExternalAccountUtils{}.DialogTexts(FolderDialog{Mode: external.DialogMode, Name: named}, folder.StableID != "", external.DialogBusy)
	external.DialogTitle, external.DialogDescription, external.DialogAction = texts[0], texts[1], texts[2]
}

func (external *ExternalMailboxService) closeDialog() {
	external.DialogOpen = false
	external.dialogAccount = ""
	external.dialogFolder = ""
	external.applicationEventPublisher.PublishEvent(FocusRequested{Restore: true}, false)
}

func (external *ExternalMailboxService) submit() {
	valid := external.DialogMode == ExternalAccountUtils{}.modes().Delete || ExternalAccountUtils{}.ValidFolderName(external.DialogName)
	FlowUtils{}.Either(valid, external.mutate, func() {
		external.DialogError = "Enter a folder name of 128 characters or fewer without leading or trailing spaces."
	})()
}

func (external *ExternalMailboxService) mutate() {
	account := external.account(external.dialogAccount)
	key := IdentityUtils{}.UUID(account.ID + external.dialogFolder + external.DialogName)
	external.DialogBusy = true
	external.DialogError = ""
	external.describe(external.folder(external.dialogAccount, external.dialogFolder))
	answer, failure := external.externalAccountsClient.Mutate(external.bearer(), account.ID, ExternalAccountUtils{}.Mutation(FolderDialog{Mode: external.DialogMode, Folder: external.dialogFolder, Name: external.DialogName}, account.Revision, key))
	external.DialogBusy = false
	FlowUtils{}.Either(HTTPUtils{}.Missed(failure, answer.ID), func() {
		external.DialogError = external.couldNot() + external.DialogMode + " the connected folder."
	}, external.queued)()
}

func (external *ExternalMailboxService) queued() {
	mode := external.DialogMode
	external.closeDialog()
	external.toast(ToastRequested{Message: "Folder " + mode + " queued. Provider folders are syncing.", ActionLabel: "Refresh folders", Action: external.refreshAction(), DurationMs: 10000})
}

// allowed checks a dialog mode against the account and, for a folder, the
// folder's provider actions.
func (external *ExternalMailboxService) allowed(mode string, account ExternalAccount, folder ExternalFolder) bool {
	rows := ExternalFolderUtils{}.Visible([]ExternalFolder{folder}, map[string]bool{}, account)
	actions := append(rows, ExternalFolder{})[0].Actions
	modes := ExternalAccountUtils{}.modes()
	permitted := map[string]bool{modes.Create: actions.CreateChild, modes.Rename: actions.Rename, modes.Delete: actions.Delete}
	root := map[string]bool{modes.Create: account.CanCreateFolder}
	return !slices.Contains([]bool{account.ID != "", map[bool]bool{true: root[mode], false: permitted[mode]}[folder.StableID == ""]}, false)
}

func (external *ExternalMailboxService) closable() bool {
	return external.DialogOpen && !external.DialogBusy
}

func (external *ExternalMailboxService) account(accountID string) ExternalAccount {
	index := slices.IndexFunc(external.Accounts, func(account ExternalAccount) bool { return account.ID == accountID })
	return append([]ExternalAccount{{}}, external.Accounts...)[index+1]
}

func (external *ExternalMailboxService) folder(accountID string, stableID string) ExternalFolder {
	folders := external.folders[accountID]
	index := slices.IndexFunc(folders, func(folder ExternalFolder) bool { return folder.StableID == stableID })
	return append([]ExternalFolder{{}}, folders...)[index+1]
}

func (ExternalMailboxService) startingName(mode string, folder ExternalFolder) string {
	if mode == "rename" {
		return TextUtils{}.FirstOf(folder.DisplayName, folder.Name)
	}
	return ""
}

func (ExternalMailboxService) focusTarget(mode string) string {
	if mode == (ExternalAccountUtils{}).modes().Delete {
		return "[data-external-folder-submit]"
	}
	return "[data-external-folder-name]"
}

// pauseAction is the action that toggles an account, with its notice.
func (ExternalMailboxService) pauseAction(state string) string {
	return map[bool]string{true: "resume", false: "pause"}[state == "paused"]
}

func (ExternalMailboxService) pauseNotice(action string) string {
	return "Account " + action + "d."
}

func (ExternalMailboxService) scheduled(scheduled bool) string {
	if scheduled {
		return "scheduled"
	}
	return ""
}

func (external *ExternalMailboxService) toast(request ToastRequested) {
	external.applicationEventPublisher.PublishEvent(request, false)
}

func (external *ExternalMailboxService) bearer() string {
	return "Bearer " + external.token
}

func (ExternalMailboxService) reorderFailure() string { return "Could not reorder connected accounts." }

func (ExternalMailboxService) couldNot() string { return "Could not " }

func (ExternalMailboxService) updateAction() string { return "update" }

func (ExternalMailboxService) refreshAction() string { return "refresh-external-folders" }

func (ExternalMailboxService) folderForm() string { return "external-folder" }

func (ExternalMailboxService) openKey() string { return "caduceus-external-folder-open" }
