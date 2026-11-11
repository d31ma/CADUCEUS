import (
	"cmp"
	"crypto/rand"
	"encoding/json"
	"os"
	"strings"
	"time"

	"mesail.dev/packages/attachments"
	"mesail.dev/packages/blobs"
	"mesail.dev/packages/oidc"
)

// @TestSetup
type AttachmentsTestSetup struct{}

func (AttachmentsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (setup AttachmentsTestSetup) SignedIn(domainId string, memberId string, secret string) string {
	base := cmp.Or(attachments.SettingsReader{Environment: os.Getenv}.Read().Folder, os.TempDir())
	folder, _ := os.MkdirTemp(base, "test-")
	keys, _ := os.MkdirTemp("", "a1-keys-")
	os.Setenv("TMPDIR", keys)
	realm, _ := oidc.SigningRealm{}.Started(domainId)
	snapshot := struct {
		Yon struct {
			Attachments attachments.Settings `json:"attachments"`
			Oidc        oidc.Settings        `json:"oidc"`
		} `json:"yon"`
	}{}
	snapshot.Yon.Attachments = attachments.Settings{Adapter: "local", Folder: folder, Messaging: "http://127.0.0.1:9", Link: attachments.LinkSettings{Base: "http://127.0.0.1:8100/blobs", Secret: secret}, Limit: attachments.LimitSettings{Bytes: 100}}
	snapshot.Yon.Oidc = realm.Settings
	text, _ := json.Marshal(snapshot)
	os.Setenv("YON_APPLICATION_JSON", string(text))
	realm.Publish(time.Now())
	return setup.token(realm, memberId)
}

func (setup AttachmentsTestSetup) token(realm oidc.SigningRealm, memberId string) string {
	request := oidc.TokenRequest{Subject: memberId, Email: memberId + "@" + realm.Name, Name: memberId, RealmRoles: []string{"mesail-member"}, Audience: "mesail-api", AuthorizedParty: "mesail-identity", ExpiresAt: time.Now().Unix() + 60}
	token, _ := realm.Token(request)
	return "Bearer " + token
}

func (setup AttachmentsTestSetup) Domain() string {
	return "a1-" + strings.ToLower(rand.Text()[:10]) + ".test"
}

func (setup AttachmentsTestSetup) Request(content string, mediaType string) CreateUploadRequest {
	inspection, _ := blobs.Inspector{}.Inspect(strings.NewReader(content), int64(len(content)))
	return CreateUploadRequest{FileName: "plan.txt", MediaType: mediaType, Size: inspection.Size, Sha256: inspection.Sha256}
}

func (setup AttachmentsTestSetup) Store(domainId string, attachmentId string, content string) {
	settings := attachments.SettingsReader{Environment: os.Getenv}.Read()
	book := attachments.RecordBook{Documents: DataSource{}, Now: time.Now}
	record, _ := book.Lookup(domainId, attachmentId)
	folder := blobs.NewLocalFolder(settings.Folder, settings.Link.Base, blobs.NewLocalSigner([]byte(settings.Link.Secret)))
	folder.Write(book.Key(record), strings.NewReader(content), int64(len(content)))
}
