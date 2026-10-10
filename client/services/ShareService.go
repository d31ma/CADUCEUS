import (
	"strconv"
	"strings"
	"time"
)

// ShareService manages the secret links that share the open conversation:
// it lists them, creates a new one (shown once), copies or opens it and
// revokes links.
//
// @Service
type ShareService struct {
	// @Needs
	threadSharesClient        *ThreadSharesClient
	clipboard                 Clipboard
	clock                     Clock
	applicationEventPublisher ApplicationEventPublisher

	Open        bool
	Busy        bool
	Error       string
	ExpiresDays string
	Shares      []ThreadShare
	Created     CreatedShare
	HasCreated  bool

	emailId string
	origin  string
	token   string
}

// @OnStart
func (service *ShareService) Start() {
	service.ExpiresDays = "7"
	service.Shares = []ThreadShare{}
}

// @OnEvent({"replay":true})
func (service *ShareService) SessionChanged(changed SessionChanged) {
	service.token = changed.Token
}

// @OnEvent({"replay":true})
func (service *ShareService) ReaderOpened(opened ReaderOpened) {
	service.origin = opened.Origin
}

// @OnEvent({"replay":true})
func (service *ShareService) MessageOpened(opened OpenMessageChanged) {
	service.emailId = opened.EmailId
}

func (service *ShareService) OpenDialog() {
	service.Open = true
	service.Error = ""
	service.forgetCreated()
	service.load()
}

func (ShareService) backdrop() string { return "share-backdrop" }

func (service *ShareService) Close() {
	service.close(service.backdrop())
}

func (service *ShareService) Dismiss(className string) {
	service.close(className)
}

func (service *ShareService) Create() {
	service.create()
}

func (service *ShareService) Copy() {
	failure := service.clipboard.WriteText(service.Created.AbsoluteUrl)
	service.copied(failure == nil)
}

func (service *ShareService) OpenLink() {
	service.applicationEventPublisher.PublishEvent(ExternalLinkRequested{Url: service.Created.AbsoluteUrl}, false)
}

func (service *ShareService) Revoke(share ThreadShare) {
	service.revoke(share.Id)
}

func (service *ShareService) authorization() string {
	return "Bearer " + service.token
}

func (service *ShareService) forgetCreated() {
	service.Created = CreatedShare{}
	service.HasCreated = false
}

func (service *ShareService) close(className string) {
	if !service.Busy && className == service.backdrop() {
		service.Open = false
		service.forgetCreated()
	}
}

func (service *ShareService) load() {
	service.Busy = true
	shares, failure := service.threadSharesClient.List(service.emailId, service.authorization())
	now, _ := service.clock.NowMs()
	service.listed(shares, failure, now)
	service.Busy = false
}

func (service *ShareService) listed(shares []ThreadShare, failure error, now float64) {
	if failure != nil {
		service.Error = "Could not load sharing links."
		return
	}
	service.Shares = service.labelled(shares, now)
}

func (service *ShareService) labelled(shares []ThreadShare, now float64) []ThreadShare {
	labelled := []ThreadShare{}
	for _, share := range shares {
		labelled = append(labelled, service.status(share, service.expired(share, now)))
	}
	return labelled
}

func (service *ShareService) expired(share ThreadShare, now float64) bool {
	expires, failure := time.Parse(time.RFC3339Nano, share.ExpiresAt)
	if !share.Active || failure != nil || float64(expires.UnixMilli()) <= now {
		return true
	}
	return false
}

func (service *ShareService) status(share ThreadShare, expired bool) ThreadShare {
	switch {
	case share.RevokedAt != "":
		share.StatusPrefix, share.StatusDate = "Revoked ", share.RevokedAt
	case expired:
		share.StatusPrefix, share.StatusDate = "Expired", ""
	default:
		share.StatusPrefix, share.StatusDate = "Expires ", share.ExpiresAt
	}
	return share
}

func (service *ShareService) create() {
	if service.emailId != "" && !service.Busy {
		service.mint()
	}
}

func (service *ShareService) mint() {
	days, _ := strconv.Atoi(service.ExpiresDays)
	service.Busy = true
	service.Error = ""
	created, failure := service.threadSharesClient.Create(map[string]any{"emailId": service.emailId, "expiresInDays": days}, service.authorization())
	service.minted(created, failure)
	service.Busy = false
}

func (service *ShareService) minted(created CreatedShare, failure error) {
	if failure != nil || created.Url == "" {
		service.Error = "Could not create sharing link."
		return
	}
	created.AbsoluteUrl = strings.TrimRight(service.origin, "/") + created.Url
	service.Created = created
	service.HasCreated = true
	service.load()
	service.toast("Private conversation link created.")
}

func (service *ShareService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (service *ShareService) copied(written bool) {
	if written {
		service.toast("Sharing link copied.")
		return
	}
	service.toast("Could not copy the sharing link.")
}

func (service *ShareService) revoke(shareId string) {
	if shareId != "" && !service.Busy {
		service.revokeNow(shareId)
	}
}

func (service *ShareService) revokeNow(shareId string) {
	service.Busy = true
	service.Error = ""
	_, failure := service.threadSharesClient.Revoke(shareId, service.authorization())
	service.revoked(failure == nil, shareId)
	service.Busy = false
}

func (service *ShareService) revoked(removed bool, shareId string) {
	if !removed {
		service.Error = "Could not revoke sharing link."
		return
	}
	service.afterRevoke(shareId)
}

func (service *ShareService) afterRevoke(shareId string) {
	service.forgetRevoked(shareId)
	service.load()
	service.toast("Sharing link revoked.")
}

func (service *ShareService) forgetRevoked(shareId string) {
	if shareId == service.Created.Id {
		service.forgetCreated()
	}
}
