import (
	"errors"
	"net/http"
	"net/http/httptest"

	"mesail.dev/packages/oidc"
)

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type ReaderAccessServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ReaderAccessServiceTest) request() AccessRequest {
	return AccessRequest{Record: AttachmentRecord{UploaderId: "ana"}, AttachmentId: "att_1", Authorization: "Bearer t", Messaging: "http://127.0.0.1:9"}
}

func (test *ReaderAccessServiceTest) TestCheckAllowsTheUploader() {
	Assert.IsTrue((&ReaderAccessService{}).Check(AttachmentsTestSetup{}.Member("example.com", "ana"), test.request()) == nil, "uploader refused")
}

func (test *ReaderAccessServiceTest) TestCheckAllowsAServiceOfTheDomain() {
	caller := oidc.Caller{DomainId: "example.com", MemberId: "mail-out", Roles: []string{"Service"}}
	Assert.IsTrue((&ReaderAccessService{}).Check(caller, test.request()) == nil, "service refused")
}

func (test *ReaderAccessServiceTest) TestCheckRefusesAnotherMemberWhenMessagingDoesNotAnswer() {
	err := (&ReaderAccessService{}).Check(AttachmentsTestSetup{}.Member("example.com", "bo"), test.request())
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(err, &missing), "another member was allowed")
}

func (test *ReaderAccessServiceTest) TestCheckRefusesAnotherMemberWithoutAMessagingAddress() {
	request := test.request()
	request.Messaging = "::not a url"
	err := (&ReaderAccessService{}).Check(AttachmentsTestSetup{}.Member("example.com", "bo"), request)
	missing := &AttachmentNotFound{}
	Assert.IsTrue(errors.As(err, &missing), "another member was allowed")
}

func (test *ReaderAccessServiceTest) TestCheckAllowsAnotherMemberWhenMessagingSaysYes() {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	request := test.request()
	request.Messaging = server.URL
	Assert.IsTrue((&ReaderAccessService{}).Check(AttachmentsTestSetup{}.Member("example.com", "bo"), request) == nil, "messaging said yes but access was refused")
}
