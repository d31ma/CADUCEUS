import (
	"net/url"
	"slices"
	"strings"
	"time"
)

// ShareService is the standalone, unauthenticated view of one expiring
// conversation link. It reads the conversation again every 30 seconds and
// when the tab becomes visible, so a revoked link stops showing it.
// @Service
type ShareService struct {
	// @Needs
	threadSharesClient        *ThreadSharesClient
	applicationEventPublisher ApplicationEventPublisher
	Loading                   bool
	Error                     string
	Available                 bool
	Thread                    SharedThread
	DownloadError             string
	DownloadHref              string
	DownloadName              string
	token                     string
}

// @OnStart
func (share *ShareService) Start() {
	share.Loading = true
	share.Thread = SharedThread{Messages: []SharedMessage{}}
}

// HearLocation reads the link's token from /share/<token> and loads it.
// @OnEvent({"replay":true})
func (share *ShareService) HearLocation(place PageLocation) {
	token, _ := url.PathUnescape(strings.TrimPrefix(place.Path, "/share/"))
	share.token = ShareUtils{}.Token(place.Path, token)
	share.load()
	share.applicationEventPublisher.PublishEvent(TimerRequested{Name: share.timer(), DelayMs: 30000, Repeat: true}, false)
}

// HearTimer reads the conversation again.
// @OnEvent
func (share *ShareService) HearTimer(elapsed TimerElapsed) {
	FlowUtils{}.When(elapsed.Name == share.timer(), share.load)()
}

// HearVisibility reads the conversation again when the tab comes back.
// @OnEvent
func (share *ShareService) HearVisibility(changed VisibilityChanged) {
	FlowUtils{}.When(changed.Visible, share.load)()
}

// Download saves one attachment of the shared conversation.
func (share *ShareService) Download(id string) error {
	share.DownloadError = ""
	content, failure := share.threadSharesClient.Attachment(share.token, id)
	FlowUtils{}.Either(!HTTPUtils{}.Missed(failure, content.ContentBase64), func() { share.save(content, id) }, func() {
		share.DownloadError = "This attachment is unavailable. The link may have expired or been revoked."
	})()
	return failure
}

func (share *ShareService) load() {
	thread, failure := share.threadSharesClient.Thread(share.token)
	share.adopt(!slices.Contains([]bool{failure == nil, len(thread.Messages) > 0, share.token != ""}, false), thread, time.Now())
	share.Loading = false
	share.applicationEventPublisher.PublishEvent(ShareLoaded{Available: share.Available}, false)
}

func (share *ShareService) adopt(available bool, thread SharedThread, now time.Time) {
	share.Available = available
	share.Thread = ShareUtils{}.Decorated(thread, now)
	share.Error = ShareUtils{}.Unavailable(available, share.token)
}

// save points the hidden download link at the attachment and clicks it.
func (share *ShareService) save(content SharedAttachmentContent, id string) {
	share.DownloadName = TextUtils{}.FirstOf(content.Filename, ShareUtils{}.Filename(share.Thread, id))
	share.DownloadHref = "data:" + TextUtils{}.FirstOf(content.ContentType, "application/octet-stream") + ";base64," + content.ContentBase64
	share.applicationEventPublisher.PublishEvent(ClickRequested{Selector: "#shared-thread-download"}, false)
}

func (ShareService) timer() string { return "share" }
