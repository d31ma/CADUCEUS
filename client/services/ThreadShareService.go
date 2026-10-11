import (
	"strconv"
	"strings"
	"time"
)

// ThreadShareService manages the secret links that share the open
// conversation: it lists them, creates a new one (shown once), copies or
// opens it and revokes links. ShareService is the public page such a link
// opens.
//
// @Service
type ThreadShareService struct {
	// @Needs
	threadSharesClient        *ThreadSharesClient
	clipboard                 Clipboard
	applicationEventPublisher ApplicationEventPublisher

	Open        bool
	Busy        bool
	Error       string
	ExpiresDays string
	Shares      []ThreadShare
	HasShares   bool
	Created     CreatedShare
	HasCreated  bool

	emailID string
	origin  string
	token   string
}

// @OnStart
func (share *ThreadShareService) Start() {
	share.ExpiresDays = "7"
	share.Shares = []ThreadShare{}
}

// @OnEvent({"replay":true})
func (share *ThreadShareService) HearSession(changed SessionChanged) {
	share.token = changed.Token
}

// HearOrigin keeps the page origin, which a copied link starts with.
// @OnEvent({"replay":true})
func (share *ThreadShareService) HearOrigin(page PageOrigin) {
	share.origin = page.Origin
}

// @OnEvent({"replay":true})
func (share *ThreadShareService) HearMessage(opened OpenMessageChanged) {
	share.emailID = opened.EmailId
}

// HearBackdrop closes the dialog on a click outside it.
// @OnEvent
func (share *ThreadShareService) HearBackdrop(clicked BackdropClicked) {
	FlowUtils{}.When(clicked.Dialog == "reader-share", share.Close)()
}

func (share *ThreadShareService) OpenDialog() {
	share.Open = true
	share.Error = ""
	share.forgetCreated()
	share.load()
}

func (share *ThreadShareService) Close() {
	share.Open = share.Busy
	FlowUtils{}.When(!share.Busy, share.forgetCreated)()
}

// Create makes a new link; the server revokes the previous one.
func (share *ThreadShareService) Create() {
	FlowUtils{}.When(LogicUtils{}.All(share.emailID != "", !share.Busy), share.mint)()
}

func (share *ThreadShareService) Copy() {
	failure := share.clipboard.WriteText(share.Created.AbsoluteUrl)
	share.toast(TextUtils{}.Choose(failure == nil, "Sharing link copied.", "Could not copy the sharing link."))
}

// OpenLink opens the new link in a new tab through a hidden link.
func (share *ThreadShareService) OpenLink() {
	share.applicationEventPublisher.PublishEvent(ClickRequested{Selector: "[data-thread-share-link]"}, false)
}

func (share *ThreadShareService) Revoke(shareID string) {
	FlowUtils{}.When(LogicUtils{}.All(shareID != "", !share.Busy), func() { share.revoke(shareID) })()
}

func (share *ThreadShareService) bearer() string {
	return "Bearer " + share.token
}

func (share *ThreadShareService) forgetCreated() {
	share.Created = CreatedShare{}
	share.HasCreated = false
}

func (share *ThreadShareService) load() {
	share.Busy = true
	shares, failure := share.threadSharesClient.List(share.bearer(), share.emailID)
	share.Error = TextUtils{}.Choose(failure == nil, share.Error, "Could not load sharing links.")
	share.Shares = share.labelled(shares, time.Now())
	share.HasShares = len(share.Shares) > 0
	share.Busy = false
}

func (share *ThreadShareService) labelled(shares []ThreadShare, now time.Time) []ThreadShare {
	labelled := []ThreadShare{}
	for _, item := range shares {
		labelled = append(labelled, share.status(item, now))
	}
	return labelled
}

// status is "Revoked <date>", "Expired" or "Expires <date>".
func (ThreadShareService) status(item ThreadShare, now time.Time) ThreadShare {
	expires, failure := time.Parse(time.RFC3339Nano, item.ExpiresAt)
	expired := LogicUtils{}.Any(!item.Active, failure != nil, !expires.After(now))
	revoked := item.RevokedAt != ""
	item.Active = LogicUtils{}.All(!expired, !revoked)
	item.MessageLabel = TextUtils{}.Count(item.MessageCount, "message", "messages")
	item.StatusLabel = map[bool]string{true: "Revoked " + DateLabelUtils{}.Stamp(item.RevokedAt, now), false: TextUtils{}.Choose(expired, "Expired", "Expires "+DateLabelUtils{}.Stamp(item.ExpiresAt, now))}[revoked]
	return item
}

func (share *ThreadShareService) mint() {
	days, _ := strconv.Atoi(share.ExpiresDays)
	share.Busy = true
	share.Error = ""
	created, failure := share.threadSharesClient.Create(share.bearer(), map[string]any{"emailId": share.emailID, "expiresInDays": days})
	share.Busy = false
	FlowUtils{}.Either(HTTPUtils{}.Missed(failure, created.Url), func() { share.Error = "Could not create sharing link." }, func() { share.minted(created) })()
}

func (share *ThreadShareService) minted(created CreatedShare) {
	created.AbsoluteUrl = strings.TrimRight(share.origin, "/") + created.Url
	share.Created = created
	share.HasCreated = true
	share.load()
	share.toast("Private conversation link created.")
}

func (share *ThreadShareService) toast(message string) {
	share.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (share *ThreadShareService) revoke(shareID string) {
	share.Busy = true
	share.Error = ""
	_, failure := share.threadSharesClient.Revoke(share.bearer(), shareID)
	share.Busy = false
	FlowUtils{}.Either(failure == nil, func() { share.revoked(shareID) }, func() { share.Error = "Could not revoke sharing link." })()
}

func (share *ThreadShareService) revoked(shareID string) {
	FlowUtils{}.When(shareID == share.Created.Id, share.forgetCreated)()
	share.load()
	share.toast("Sharing link revoked.")
}
