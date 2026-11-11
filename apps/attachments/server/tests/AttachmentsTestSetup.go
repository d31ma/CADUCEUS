import (
	"cmp"
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"

	"mesail.dev/packages/blobs"
	"mesail.dev/packages/oidc"
)

// @TestSetup
type AttachmentsTestSetup struct{}

func (AttachmentsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (setup AttachmentsTestSetup) Settings(messaging string) AttachmentSettings {
	base := cmp.Or((&SettingsService{}).Settings().Folder, os.TempDir())
	folder, _ := os.MkdirTemp(base, "test-")
	attachments := AttachmentSettings{Adapter: "local", Folder: folder, Messaging: messaging, Link: LinkSettings{Base: "http://127.0.0.1:8100/blobs", Secret: "test secret"}}
	text, _ := json.Marshal(ApplicationSettings{Yon: YonSettings{Attachments: attachments}})
	os.Setenv("YON_APPLICATION_JSON", string(text))
	return (&SettingsService{}).Settings()
}

func (setup AttachmentsTestSetup) Domain() string {
	return "a1-" + strings.ToLower(rand.Text()[:10]) + ".test"
}

func (setup AttachmentsTestSetup) Member(domainId string, memberId string) oidc.Caller {
	return oidc.Caller{DomainId: domainId, MemberId: memberId, Roles: []string{"Member"}}
}

func (setup AttachmentsTestSetup) Folder(settings AttachmentSettings) blobs.LocalFolder {
	return blobs.NewLocalFolder(settings.Folder, settings.Link.Base, blobs.NewLocalSigner([]byte(settings.Link.Secret)))
}

func (setup AttachmentsTestSetup) Request(content string, mediaType string) CreateUploadRequest {
	inspection, _ := blobs.Inspector{}.Inspect(strings.NewReader(content), int64(len(content)))
	return CreateUploadRequest{FileName: "plan.txt", MediaType: mediaType, Size: inspection.Size, Sha256: inspection.Sha256}
}

func (setup AttachmentsTestSetup) Records() *AttachmentRecordService {
	return &AttachmentRecordService{}
}

func (setup AttachmentsTestSetup) Creator() *AttachmentCreateService {
	return &AttachmentCreateService{callerService: &CallerService{}, settingsService: &SettingsService{}, uploadCheckService: &UploadCheckService{}, blobStoreService: &BlobStoreService{}, attachmentRecordService: setup.Records()}
}

func (setup AttachmentsTestSetup) Completer() *AttachmentCompleteService {
	return &AttachmentCompleteService{callerService: &CallerService{}, settingsService: &SettingsService{}, blobStoreService: &BlobStoreService{}, attachmentRecordService: setup.Records(), uploadVerifyService: &UploadVerifyService{}}
}

func (setup AttachmentsTestSetup) Downloader() *AttachmentDownloadService {
	return &AttachmentDownloadService{callerService: &CallerService{}, settingsService: &SettingsService{}, blobStoreService: &BlobStoreService{}, attachmentRecordService: setup.Records(), readerAccessService: &ReaderAccessService{}}
}

func (setup AttachmentsTestSetup) Uploaded(caller oidc.Caller, content string, mediaType string) (Upload, AttachmentSettings) {
	settings := setup.Settings("http://127.0.0.1:9")
	upload, _ := setup.Creator().create(caller, setup.Request(content, mediaType))
	record, _ := setup.Records().Lookup(caller.DomainId, upload.AttachmentId)
	setup.Folder(settings).Write(setup.Records().Key(record), strings.NewReader(content), int64(len(content)))
	return upload, settings
}
