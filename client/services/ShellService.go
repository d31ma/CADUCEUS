import (
	"encoding/json"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ShellService is the app frame: who is signed in, which route and folder are
// active, the user menu, the mobile drawer, the sidebar disclosures,
// screenshot mode, the toast, the keyboard-command dialog and the presence
// heartbeat. Its view is components/app/shell. The sidebar data lives in
// MailboxService and ExternalMailboxService, the notification centre in
// NotificationCenterService and the theme in PreferenceService.
// @Service
type ShellService struct {
	// @Needs
	presenceClient *PresenceClient
	// @Needs
	snoozeClient *SnoozeClient
	// @Needs
	sendClient                *SendClient
	applicationEventPublisher ApplicationEventPublisher
	SignedIn                  bool
	Restored                  bool
	IsRootPath                bool
	BrandLogo                 string
	Email                     string
	DisplayName               string
	ActiveFolder              string
	SelectedDomain            string
	SelectedAccount           string
	UserMenuOpen              bool
	MobileSidebarOpen         bool
	ScreenshotMode            bool
	ShortcutHelpOpen          bool
	ShortcutGroups            []ShortcutGroup
	ToastVisible              bool
	ToastMessage              string
	ToastActionLabel          string
	toastAction               string
	toastSubject              string
	toastSerial               int
	token                     string
	locale                    string
	pending                   map[string]PendingUndo
	sections                  map[string]string
}

// @OnStart
func (shell *ShellService) Start() {
	shell.BrandLogo = "/shared/assets/favicon.svg"
	shell.IsRootPath = true
	shell.ActiveFolder = shell.names().Inbox
	shell.locale = LocaleUtils{}.Resolve("", "")
	shell.pending = map[string]PendingUndo{}
	shell.sections = map[string]string{}
}

// HearSession mirrors the signed-in user, sends the first presence heartbeat
// and keeps beating every 30 seconds.
// @OnEvent({"replay":true})
func (shell *ShellService) HearSession(changed SessionChanged) {
	shell.SignedIn = changed.SignedIn
	shell.Restored = true
	shell.Email = changed.Email
	shell.DisplayName = changed.DisplayName
	shell.token = changed.Token
	shell.ScreenshotMode = shell.keptScreenshotMode(changed.SignedIn)
	FlowUtils{}.When(changed.SignedIn, shell.beginBeating)()
	shell.redirectFromRoot()
}

// HearLocation reads the active folder and the selected mailbox from the
// address, and sends a signed-in visitor from / to the inbox.
// @OnEvent({"replay":true})
func (shell *ShellService) HearLocation(place PageLocation) {
	query, _ := url.ParseQuery(strings.TrimPrefix(place.Search, "?"))
	shell.IsRootPath = place.Path == shell.names().Root
	shell.ActiveFolder = FolderUtils{}.Active(place.Path, query.Get("folder"), shell.ActiveFolder)
	shell.SelectedDomain = strings.ToLower(query.Get("domain"))
	shell.SelectedAccount = query.Get("account")
	shell.redirectFromRoot()
}

// HearStorage restores screenshot mode, the sidebar disclosures and the
// pending undo actions that the reader and the composer left in this tab.
// @OnEvent({"replay":true})
func (shell *ShellService) HearStorage(stored StoredValues) {
	names := shell.names()
	shell.ScreenshotMode = stored.Session[names.Screenshot] == names.On
	shell.sections = stored.Local
	shell.locale = LocaleUtils{}.Resolve(stored.Local[names.Locale], "")
	shell.applyScreenshotMode()
	shell.offerPendingUndo(shell.pendingFrom(stored.Session[names.Snooze]), names.Snooze)
	shell.offerPendingUndo(shell.pendingFrom(stored.Session[names.Send]), names.Send)
}

// HearPreferences follows the interface locale of the signed-in user.
// @OnEvent({"replay":true})
func (shell *ShellService) HearPreferences(loaded PreferencesLoaded) {
	shell.locale = LocaleUtils{}.Resolve(loaded.InterfaceLocale, "")
}

// Text translates one fixed interface message in the active locale.
func (shell *ShellService) Text(id string) string {
	return LocaleUtils{}.Translate(shell.locale, id, nil)
}

// Encode is encodeURIComponent for the view's links.
func (shell *ShellService) Encode(value string) string {
	return strings.ReplaceAll(url.QueryEscape(value), "+", "%20")
}

// SectionExpanded is true unless the user closed this sidebar section on this
// device.
func (shell *ShellService) SectionExpanded(name string) bool {
	return shell.sections[shell.sectionKey(name)] != shell.names().Closed
}

// SectionClass is the class of a sidebar section, with "collapsed" when closed.
func (shell *ShellService) SectionClass(name string) string {
	return shell.collapsed("sidebar-section", shell.sections[shell.sectionKey(name)] == shell.names().Closed)
}

// ToggleSection opens or closes a sidebar section and keeps the choice.
func (shell *ShellService) ToggleSection(name string) {
	shell.remember(shell.sectionKey(name), shell.flipped(shell.sections[shell.sectionKey(name)]))
}

// FavoriteExpanded is true when a Favorites group shows its domains: All
// Inboxes starts open and All Sent starts closed.
func (shell *ShellService) FavoriteExpanded(name string) bool {
	return shell.favoriteState(name) == shell.names().Open
}

// FavoriteGroupClass is the class of a Favorites group.
func (shell *ShellService) FavoriteGroupClass(name string) string {
	return shell.collapsed("favorite-group", shell.favoriteState(name) == shell.names().Closed)
}

// ToggleFavorite opens or closes a Favorites group and keeps the choice.
func (shell *ShellService) ToggleFavorite(name string) {
	shell.remember(shell.favoriteKey(name), shell.flipped(shell.favoriteState(name)))
}

// ToggleUserMenu opens or closes the top-bar user menu.
func (shell *ShellService) ToggleUserMenu() {
	shell.MobileSidebarOpen = false
	shell.UserMenuOpen = !shell.UserMenuOpen
}

// CloseUserMenu closes the user menu after one of its items is chosen.
func (shell *ShellService) CloseUserMenu() {
	shell.UserMenuOpen = false
}

// ToggleMobileSidebar opens or closes the responsive navigation drawer.
func (shell *ShellService) ToggleMobileSidebar() {
	shell.UserMenuOpen = false
	shell.MobileSidebarOpen = !shell.MobileSidebarOpen
}

// CloseMobileSidebar closes the navigation drawer.
func (shell *ShellService) CloseMobileSidebar() {
	shell.MobileSidebarOpen = false
}

// ToggleScreenshotMode masks or restores every visible personal value.
func (shell *ShellService) ToggleScreenshotMode() {
	shell.ScreenshotMode = !shell.ScreenshotMode
	shell.UserMenuOpen = false
	shell.applyScreenshotMode()
	shell.applicationEventPublisher.PublishEvent(StorageWritten{Area: shell.names().Session, Values: map[string]string{shell.names().Screenshot: shell.onOff(shell.ScreenshotMode)}, Removed: []string{}}, false)
	shell.showToast(ToastRequested{Message: shell.screenshotNotice(shell.ScreenshotMode)})
}

// SignOut asks SessionService to end the session.
func (shell *ShellService) SignOut() {
	shell.applicationEventPublisher.PublishEvent(SignOutRequested{Reason: "user"}, false)
}

// CloseShortcutHelp hides the keyboard-command dialog and returns the focus.
func (shell *ShellService) CloseShortcutHelp() {
	shell.ShortcutHelpOpen = false
	shell.applicationEventPublisher.PublishEvent(FocusRequested{Restore: true}, false)
}

// OpenShortcutSettings leaves the dialog for the shortcut settings.
func (shell *ShellService) OpenShortcutSettings() {
	shell.ShortcutHelpOpen = false
	shell.navigate("/settings#keyboard-shortcuts")
}

// RunToastAction runs the toast's action once and hides the toast.
func (shell *ShellService) RunToastAction() {
	chosen := ToastActionChosen{Action: shell.toastAction, Subject: shell.toastSubject}
	shell.dismissToast()
	shell.applicationEventPublisher.PublishEvent(chosen, false)
}

// HearToast shows a toast that any service asked for.
// @OnEvent
func (shell *ShellService) HearToast(request ToastRequested) {
	shell.showToast(request)
}

// HearTimer hides an expired toast and sends the presence heartbeat.
// @OnEvent
func (shell *ShellService) HearTimer(elapsed TimerElapsed) {
	shell.timerAction(elapsed.Name)()
}

// HearVisibility beats again when a hidden tab comes back.
// @OnEvent
func (shell *ShellService) HearVisibility(changed VisibilityChanged) {
	FlowUtils{}.When(changed.Visible, shell.beatWhenSignedIn)()
}

// HearToastAction undoes a pending snooze or send.
// @OnEvent
func (shell *ShellService) HearToastAction(chosen ToastActionChosen) {
	shell.undoAction(chosen.Action)(shell.pending[chosen.Action])
}

// HearShortcut runs the keyboard commands that the shell owns.
// @OnEvent
func (shell *ShellService) HearShortcut(pressed ShortcutPressed) {
	shell.shortcutAction(pressed.Command)()
}

// HearShortcutGroups keeps the rows of the keyboard-command dialog.
// @OnEvent({"replay":true})
func (shell *ShellService) HearShortcutGroups(listed ShortcutGroupsListed) {
	shell.ShortcutGroups = listed.Groups
}

// HearEscape closes the drawer and the keyboard-command dialog.
// @OnEvent
func (shell *ShellService) HearEscape(pressed EscapePressed) {
	shell.MobileSidebarOpen = false
	FlowUtils{}.When(shell.ShortcutHelpOpen, shell.CloseShortcutHelp)()
}

// HearBackdrop closes the keyboard-command dialog on a click beside it.
// @OnEvent
func (shell *ShellService) HearBackdrop(clicked BackdropClicked) {
	FlowUtils{}.When(clicked.Dialog == "shortcut-help", shell.CloseShortcutHelp)()
}

// HearPageClick closes the user menu on a click outside it.
// @OnEvent
func (shell *ShellService) HearPageClick(clicked PageClicked) {
	shell.UserMenuOpen = shell.keptUserMenu(clicked.InsideUserMenu)
}

func (shell *ShellService) openShortcutHelp() {
	shell.ShortcutHelpOpen = true
	shell.applicationEventPublisher.PublishEvent(FocusRequested{Selector: "[data-shortcut-help-dialog]", Remember: true}, false)
}

func (shell *ShellService) redirectFromRoot() {
	FlowUtils{}.When(shell.IsRootPath && shell.SignedIn, func() {
		shell.applicationEventPublisher.PublishEvent(NavigationRequested{Target: shell.names().Root + shell.names().Inbox, Replace: true}, false)
	})()
}

func (shell *ShellService) beginBeating() {
	shell.beat()
	shell.applicationEventPublisher.PublishEvent(TimerRequested{Name: shell.names().Presence, DelayMs: 30000, Repeat: true}, false)
}

func (shell *ShellService) beatWhenSignedIn() {
	FlowUtils{}.When(shell.SignedIn, shell.beat)()
}

func (shell *ShellService) beat() {
	shell.presenceClient.Heartbeat(shell.bearer())
}

func (shell *ShellService) keptScreenshotMode(signedIn bool) bool {
	return shell.ScreenshotMode && signedIn
}

func (shell *ShellService) keptUserMenu(inside bool) bool {
	return shell.UserMenuOpen && inside
}

func (shell *ShellService) applyScreenshotMode() {
	shell.applicationEventPublisher.PublishEvent(DocumentAttributesChanged{
		Owner:      "screenshot",
		Attributes: map[string]string{"data-screenshot-mode": shell.trueOrEmpty(shell.ScreenshotMode)},
	}, false)
}

func (shell *ShellService) showToast(request ToastRequested) {
	shell.toastSerial++
	shell.ToastMessage = request.Message
	shell.ToastActionLabel = request.ActionLabel
	shell.toastAction = request.Action
	shell.toastSubject = request.Subject
	shell.ToastVisible = true
	shell.applicationEventPublisher.PublishEvent(TimerRequested{Name: shell.toastTimer(), DelayMs: shell.toastDuration(request.DurationMs), Repeat: false}, false)
}

func (shell *ShellService) dismissToast() {
	shell.ToastVisible = false
	shell.ToastActionLabel = ""
	shell.toastAction = ""
	shell.toastSubject = ""
}

func (shell *ShellService) toastTimer() string {
	return "toast-" + strconv.Itoa(shell.toastSerial)
}

func (shell *ShellService) timerAction(name string) func() {
	switch name {
	case shell.names().Presence:
		return shell.beatWhenSignedIn
	case shell.toastTimer():
		return shell.dismissToast
	}
	return func() {}
}

func (shell *ShellService) shortcutAction(command string) func() {
	actions := map[string]func(){
		"core:compose":                func() { shell.navigate("/compose") },
		"core:go-inbox":               func() { shell.navigate(shell.names().Root + shell.names().Inbox) },
		"core:go-drafts":              func() { shell.navigate("/drafts") },
		"core:go-sent":                func() { shell.navigate("/sent") },
		"core:go-archive":             func() { shell.navigate("/archive") },
		"core:go-spam":                func() { shell.navigate("/spam") },
		"core:go-trash":               func() { shell.navigate("/trash") },
		"core:go-settings":            func() { shell.navigate("/settings") },
		"core:toggle-screenshot-mode": shell.ToggleScreenshotMode,
		"core:show-shortcuts":         shell.openShortcutHelp,
		"core:refresh":                shell.refreshInbox,
		"core:search":                 shell.focusSearch,
	}
	if action, found := actions[command]; found {
		return action
	}
	return func() {}
}

func (shell *ShellService) refreshInbox() {
	shell.applicationEventPublisher.PublishEvent(InboxRefreshRequested{Reason: "shortcut"}, false)
}

func (shell *ShellService) focusSearch() {
	shell.applicationEventPublisher.PublishEvent(FocusRequested{Selector: "[data-thread-search-input]"}, false)
}

func (shell *ShellService) navigate(target string) {
	shell.applicationEventPublisher.PublishEvent(NavigationRequested{Target: target}, false)
}

func (shell *ShellService) remember(key string, value string) {
	shell.sections[key] = value
	shell.applicationEventPublisher.PublishEvent(StorageWritten{Area: shell.names().Local, Values: map[string]string{key: value}, Removed: []string{}}, false)
}

func (shell *ShellService) favoriteState(name string) string {
	return TextUtils{}.FirstOf(shell.sections[shell.favoriteKey(name)], shell.favoriteDefault(name))
}

func (shell *ShellService) pendingFrom(text string) PendingUndo {
	pending := PendingUndo{}
	json.Unmarshal([]byte(text), &pending)
	return pending
}

// offerPendingUndo shows the Undo of a snooze or send that is still owned by
// this user and has not expired, and forgets any other.
func (shell *ShellService) offerPendingUndo(pending PendingUndo, key string) {
	remaining := pending.ExpiresAt - time.Now().UnixMilli()
	FlowUtils{}.Either(!slices.Contains([]bool{pending.Owner != "", pending.Owner == shell.Email, remaining > 0}, false), func() {
		shell.pending[key] = pending
		shell.showToast(ToastRequested{Message: TextUtils{}.FirstOf(pending.Message, pending.Label), ActionLabel: "Undo", Action: key, DurationMs: int(max(250, remaining))})
	}, func() { shell.forgetPending(key) })()
}

func (shell *ShellService) forgetPending(key string) {
	shell.applicationEventPublisher.PublishEvent(StorageWritten{Area: shell.names().Session, Values: map[string]string{}, Removed: []string{key}}, false)
}

func (shell *ShellService) undoAction(action string) func(PendingUndo) {
	undos := map[string]func(PendingUndo){shell.names().Snooze: shell.undoSnooze, shell.names().Send: shell.undoSend}
	if undo, found := undos[action]; found {
		return undo
	}
	return func(PendingUndo) {}
}

func (shell *ShellService) undoSnooze(pending PendingUndo) {
	shell.forgetPending(shell.names().Snooze)
	_, failure := shell.snoozeClient.Cancel(shell.bearer(), pending.ID)
	shell.afterUndo(failure, "Snooze undone.", "Could not undo snooze.")()
}

func (shell *ShellService) undoSend(pending PendingUndo) {
	shell.forgetPending(shell.names().Send)
	_, failure := shell.sendClient.Undo(shell.bearer(), SendUndo{UndoID: pending.UndoID})
	shell.afterUndo(failure, "Send undone.", "Could not undo send.")()
}

func (shell *ShellService) afterUndo(failure error, done string, failed string) func() {
	return FlowUtils{}.Either(failure == nil, func() {
		shell.showToast(ToastRequested{Message: done})
		shell.applicationEventPublisher.PublishEvent(InboxRefreshRequested{Reason: "undo"}, false)
	}, func() { shell.showToast(ToastRequested{Message: failed}) })
}

func (shell *ShellService) bearer() string {
	return "Bearer " + shell.token
}

func (ShellService) toastDuration(duration int) int {
	if duration > 0 {
		return duration
	}
	return 2500
}

func (ShellService) screenshotNotice(enabled bool) string {
	if enabled {
		return "Screenshot mode enabled."
	}
	return "Screenshot mode disabled."
}

func (shell *ShellService) onOff(enabled bool) string {
	return map[bool]string{true: shell.names().On, false: "off"}[enabled]
}

func (ShellService) trueOrEmpty(enabled bool) string {
	if enabled {
		return "true"
	}
	return ""
}

func (shell *ShellService) favoriteDefault(name string) string {
	if name == "sent" {
		return shell.names().Closed
	}
	return shell.names().Open
}

func (shell *ShellService) flipped(state string) string {
	if state == shell.names().Closed {
		return shell.names().Open
	}
	return shell.names().Closed
}

func (ShellService) collapsed(base string, closed bool) string {
	if closed {
		return base + " collapsed"
	}
	return base
}

func (ShellService) sectionKey(name string) string { return "caduceus-sidebar-section-" + name }

func (ShellService) favoriteKey(name string) string { return "caduceus-sidebar-favorite-" + name }

func (ShellService) names() ShellNames {
	return ShellNames{
		Inbox: "inbox", Presence: "presence", Session: "session", Local: "local", Open: "open", Closed: "closed",
		Snooze: "caduceus_pending_snooze_undo", Root: "/", On: "on", Screenshot: "caduceus-screenshot-mode", Send: "caduceus_pending_send_undo", Locale: "caduceus-interface-locale-v1",
	}
}
