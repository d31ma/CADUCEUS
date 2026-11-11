import (
	"errors"
	"strings"
)

// @Test(AttachmentsTestSetup, {"partition":"tests"})
type UploadCheckServiceTest struct {
	behaviors *BehaviorContext
}

func (test *UploadCheckServiceTest) valid() CreateUploadRequest {
	return CreateUploadRequest{FileName: "Q3 plan.pdf", MediaType: "application/pdf", Size: 10, Sha256: strings.Repeat("a", 64)}
}

func (test *UploadCheckServiceTest) field(err error) string {
	invalid := &InvalidUpload{}
	errors.As(err, &invalid)
	return invalid.Field
}

func (test *UploadCheckServiceTest) TestCheckAcceptsAValidRequest() {
	Assert.IsTrue((&UploadCheckService{}).Check(test.valid(), 100) == nil, "valid request refused")
}

func (test *UploadCheckServiceTest) TestCheckRefusesAFileNameWithASlash() {
	request := test.valid()
	request.FileName = "../plan.pdf"
	Assert.Equals("fileName", test.field((&UploadCheckService{}).Check(request, 100)))
}

func (test *UploadCheckServiceTest) TestCheckRefusesAFileNameWithAControlCharacter() {
	request := test.valid()
	request.FileName = "plan\r\n.pdf"
	Assert.Equals("fileName", test.field((&UploadCheckService{}).Check(request, 100)))
}

func (test *UploadCheckServiceTest) TestCheckRefusesAnEmptyOrLongFileName() {
	empty := test.valid()
	empty.FileName = ""
	long := test.valid()
	long.FileName = strings.Repeat("n", 256)
	Assert.Equals("fileName", test.field((&UploadCheckService{}).Check(empty, 100)))
	Assert.Equals("fileName", test.field((&UploadCheckService{}).Check(long, 100)))
}

func (test *UploadCheckServiceTest) TestCheckRefusesAMalformedMediaType() {
	request := test.valid()
	request.MediaType = "pdf"
	Assert.Equals("mediaType", test.field((&UploadCheckService{}).Check(request, 100)))
}

func (test *UploadCheckServiceTest) TestCheckRefusesAnEmptyFile() {
	request := test.valid()
	request.Size = 0
	Assert.Equals("size", test.field((&UploadCheckService{}).Check(request, 100)))
}

func (test *UploadCheckServiceTest) TestCheckRefusesAFileOverTheLimit() {
	large := &AttachmentTooLarge{}
	Assert.IsTrue(errors.As((&UploadCheckService{}).Check(test.valid(), 9), &large), "not too large")
	Assert.Equals(int64(9), large.Limit)
}

func (test *UploadCheckServiceTest) TestCheckRefusesABadSha256() {
	request := test.valid()
	request.Sha256 = strings.Repeat("A", 64)
	Assert.Equals("sha256", test.field((&UploadCheckService{}).Check(request, 100)))
}
