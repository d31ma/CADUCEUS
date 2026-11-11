import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"
)

// @Test(MailOutTestSetup)
type MailOutPartsTest struct {
	behaviors *BehaviorContext
}

func (test *MailOutPartsTest) settings(address string) *SettingsDelegate {
	folder, _ := os.MkdirTemp("", "mail-out-parts-")
	MailOutTestSetup{}.Use(MailOutSettings{BlobFolder: filepath.Join(folder, "blobs"), DirectoryUrl: address, MessagingUrl: address}, address, "http://127.0.0.1:1")
	return &SettingsDelegate{}
}

func (test *MailOutPartsTest) failing() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusInternalServerError)
	}))
}

func (test *MailOutPartsTest) TestSettingsFillDefaults() {
	MailOutTestSetup{}.Use(MailOutSettings{}, "http://127.0.0.1:1/events", "http://127.0.0.1:1")
	settings := &SettingsDelegate{}
	Assert.Equals(7200, settings.Settings().MaxAgeMinutes)
	Assert.Equals(5*24*time.Hour, settings.MaxAge())
	Assert.Equals(time.Minute, settings.FirstDelay())
	Assert.Equals(time.Hour, settings.MaxDelay())
	Assert.Equals(5*time.Minute, settings.Lease())
	Assert.Equals(30*time.Second, settings.Timeout())
	Assert.Equals("mail-out.localhost", settings.Settings().HeloName)
}

func (test *MailOutPartsTest) TestSettingsReadTheOutboxBlock() {
	MailOutTestSetup{}.Use(MailOutSettings{MaxAgeMinutes: 15}, "http://127.0.0.1:1/events", "http://127.0.0.1:1")
	outbox, err := (&SettingsDelegate{}).Outbox()
	Assert.Equals(nil, err)
	Assert.Equals("mail-out", outbox.Producer)
	Assert.Equals("messaging", outbox.Subscribers[0].Name)
	Assert.Equals(15, (&SettingsDelegate{}).Settings().MaxAgeMinutes)
}

func (test *MailOutPartsTest) TestDirectoryAndSourceReportServerErrors() {
	server := test.failing()
	defer server.Close()
	settings := test.settings(server.URL)
	client := &ServiceClientDelegate{settings: settings}
	authorize := func(realm string) (string, error) { return "Bearer " + realm, nil }
	_, found, directoryErr := (&DirectoryDelegate{settings: settings, client: client}).Domain("example.com", "example.com", authorize)
	_, sourceErr := (&MessageSourceDelegate{settings: settings, client: client}).Source(OutboundItem{DomainId: "example.com", Producer: "messaging", MessageId: "msg_1"}, authorize)
	Assert.IsTrue(!found)
	Assert.Equals("directory answered 500", directoryErr.Error())
	Assert.Equals("the producer answered 500 for the message source", sourceErr.Error())
}

func (test *MailOutPartsTest) TestBlobStoreKeepsEachDomainInsideItsPrefix() {
	blobs := &BlobStoreDelegate{settings: test.settings("http://127.0.0.1:1")}
	putErr := blobs.Put("example.com", "example.com/outbound-a1", []byte("raw"))
	data, getErr := blobs.Get("example.com", "example.com/outbound-a1")
	_, slashErr := blobs.Get("example.com", "example.com/outbound/a1")
	_, dotErr := blobs.Get("example.com", "example.com/outbound-a1.eml")
	_, otherDomainErr := blobs.Get("example.com", "example.org/outbound-a1")
	_, traversalErr := blobs.Get("example.com", "example.com/..")
	Assert.Equals(nil, putErr)
	Assert.Equals(nil, getErr)
	Assert.Equals("raw", string(data))
	Assert.Equals("the blob key is not inside the domain folder", otherDomainErr.Error())
	Assert.Equals("the blob key is not inside the domain folder", traversalErr.Error())
	Assert.IsTrue(slashErr != nil, "a key has one slash")
	Assert.IsTrue(dotErr != nil, "a key name has no dot")
}

func (test *MailOutPartsTest) TestBlobStoreNeedsAFolder() {
	MailOutTestSetup{}.Use(MailOutSettings{}, "http://127.0.0.1:1/events", "http://127.0.0.1:1")
	_, err := (&BlobStoreDelegate{settings: &SettingsDelegate{}}).Get("example.com", "example.com/a")
	Assert.Equals("mail-out has no blob folder", err.Error())
}

func (test *MailOutPartsTest) TestAttachmentsAreReadFromTheDomainFolder() {
	blobs := &BlobStoreDelegate{settings: test.settings("http://127.0.0.1:1")}
	blobs.Put("example.com", "example.com/attachment-att_1", []byte("pdf"))
	attachments, err := (&AttachmentSourceDelegate{blobs: blobs}).Attachments("example.com", []string{"att_1"})
	Assert.Equals(nil, err)
	Assert.Equals("pdf", string(attachments[0].Content))
	Assert.Equals("att_1", attachments[0].FileName)
}

func (test *MailOutPartsTest) TestErrorHandlerAnswersWithApiErrors() {
	handler := &MailOutErrorHandler{}
	Assert.Equals(ApiError{Code: "invalid_request", Message: "bad"}, handler.Rejected(&RejectedEvent{Reason: "bad"}))
	Assert.Equals(ApiError{Code: "forbidden", Message: "only a caller with the Service role may deliver events"}, handler.Forbidden(&ForbiddenCaller{}))
	Assert.Equals(ApiError{Code: "not_found", Message: "not found"}, handler.Unknown(&UnknownDomain{}))
}

func (test *MailOutPartsTest) TestRetryDelayDoublesUpToTheCap() {
	Assert.Equals(time.Minute, RetryUtils{}.Delay(1, time.Minute, time.Hour))
	Assert.Equals(4*time.Minute, RetryUtils{}.Delay(3, time.Minute, time.Hour))
	Assert.Equals(time.Hour, RetryUtils{}.Delay(40, time.Minute, time.Hour))
}

func (test *MailOutPartsTest) TestOnlyMailboxGoneCodesSuppress() {
	Assert.IsTrue(ReplyUtils{}.MailboxGone("5.1.1"))
	Assert.IsTrue(!ReplyUtils{}.MailboxGone("5.7.1"))
}

func (test *MailOutPartsTest) TestCollectionNamesFollowFyloRules() {
	Assert.Equals("suppressions-", StorageUtils{}.CollectionName("suppressions", "example.com")[:13])
	Assert.Equals(29, len(StorageUtils{}.CollectionName("suppressions", "a-very-long-sub-domain-name-that-goes-on.example.com")))
}

func (test *MailOutPartsTest) TestHandOffMessageIdsHaveNoAngleBrackets() {
	envelope := HandOffUtils{}.Envelope(OutboundItem{DomainId: "example.com", MessageId: "<msg_1>", InReplyTo: " <parent@example.org> ", References: []string{"<root@example.org>"}})
	Assert.Equals("msg_1@example.com", envelope.Headers.MessageId)
	Assert.Equals("parent@example.org", envelope.Headers.InReplyTo)
	Assert.Equals([]string{"root@example.org"}, envelope.Headers.References)
}

func (test *MailOutPartsTest) TestAnOperatorsRealmTokenIsAServiceCaller() {
	realms := MailOutTestSetup{}.Realms("mesail-operators")
	MailOutTestSetup{}.Use(MailOutSettings{}, "http://127.0.0.1:1/events", "http://127.0.0.1:1")
	request, _ := http.NewRequest(http.MethodPost, "/api/v1/mail-out/jobs/deliver", nil)
	request.Header.Set("Authorization", "Bearer "+MailOutTestSetup{}.ServiceToken(realms["mesail-operators"]))
	identity := (&TokenIdentityProvider{}).Identify(request)
	Assert.IsTrue(identity != nil, "the token is accepted")
	Assert.Equals([]string{"Service"}, identity.Roles)
	Assert.Equals("mesail-operators/service-account", identity.User)
}
