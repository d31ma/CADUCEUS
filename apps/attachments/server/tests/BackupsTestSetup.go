import (
	"cmp"
	"crypto/rand"
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"time"

	"mesail.dev/packages/attachments"
	"mesail.dev/packages/backups"
	"mesail.dev/packages/blobs"
	"mesail.dev/packages/oidc"
)

// @TestSetup
type BackupsTestSetup struct{}

func (BackupsTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	return NewBehaviorConfiguration(), nil
}

func (setup BackupsTestSetup) Tokens(domainId string, people []string) map[string]string {
	realm := setup.started(domainId)
	tokens := map[string]string{}
	for _, person := range people {
		tokens[person] = setup.token(realm, person, "mesail-member", "mesail-web")
	}
	tokens["service"] = setup.token(realm, "directory", "mesail-service", "mesail-service")
	return tokens
}

func (setup BackupsTestSetup) started(domainId string) oidc.SigningRealm {
	base := cmp.Or(attachments.SettingsReader{Environment: os.Getenv}.Read().Folder, os.TempDir())
	folder, _ := os.MkdirTemp(base, "backups-")
	keys, _ := os.MkdirTemp("", "g4-keys-")
	os.Setenv("TMPDIR", keys)
	started, _ := oidc.SigningRealm{}.StartedMesail()
	realm := started.ForDomain(domainId)
	stored := attachments.Settings{Adapter: "local", Folder: folder, Messaging: "http://127.0.0.1:9", Link: attachments.LinkSettings{Base: "http://127.0.0.1:8100/blobs", Secret: "secret"}}
	section := map[string]any{}
	text, _ := json.Marshal(stored)
	json.Unmarshal(text, &section)
	section["backup"] = map[string]any{"link": map[string]any{"seconds": 600}, "limit": map[string]any{"bytes": 50}}
	snapshot := map[string]any{"yon": map[string]any{"attachments": section, "oidc": realm.Settings}}
	text, _ = json.Marshal(snapshot)
	os.Setenv("YON_APPLICATION_JSON", string(text))
	realm.Publish(time.Now())
	return realm
}

func (setup BackupsTestSetup) token(realm oidc.SigningRealm, memberId string, realmRole string, party string) string {
	request := oidc.TokenRequest{Subject: memberId, Email: memberId + "@" + realm.Domain, Name: memberId, Username: memberId, RealmRoles: []string{realmRole}, Audience: "mesail-api", AuthorizedParty: party, ExpiresAt: time.Now().Unix() + 60}
	token, _ := realm.Token(request)
	return "Bearer " + token
}

func (setup BackupsTestSetup) Domain() string {
	return "g4-" + strings.ToLower(rand.Text()[:10]) + ".test"
}

func (setup BackupsTestSetup) Start(version int64, content string, device string) BackupStart {
	inspection, _ := blobs.Inspector{}.Inspect(strings.NewReader(content), int64(len(content)))
	slot := BackupKeySlot{Kind: "recovery", Salt: "c2FsdC1zYWx0LXNhbHQ", Rounds: 600000, WrappedKey: "d3JhcHBlZC1rZXk"}
	return BackupStart{ExpectedVersion: version, Size: inspection.Size, Sha256: inspection.Sha256, KeySlots: []BackupKeySlot{slot}, DeviceId: device}
}

func (setup BackupsTestSetup) Put(uploadUrl string, content string) {
	settings := attachments.SettingsReader{Environment: os.Getenv}.Read()
	parsed, _ := url.Parse(uploadUrl)
	path := strings.TrimPrefix(parsed.Path, "/blobs/")
	folder := blobs.NewLocalFolder(settings.Folder, settings.Link.Base, blobs.NewLocalSigner([]byte(settings.Link.Secret)))
	folder.Write(backups.BlobNames{}.Key(path), strings.NewReader(content), int64(len(content)))
}

func (setup BackupsTestSetup) Exists(uploadUrl string) bool {
	settings := attachments.SettingsReader{Environment: os.Getenv}.Read()
	parsed, _ := url.Parse(uploadUrl)
	folder := blobs.NewLocalFolder(settings.Folder, settings.Link.Base, blobs.NewLocalSigner([]byte(settings.Link.Secret)))
	reader, err := folder.Read(backups.BlobNames{}.Key(strings.TrimPrefix(parsed.Path, "/blobs/")))
	if err != nil {
		return false
	}
	reader.Close()
	return true
}

func (setup BackupsTestSetup) Controller() *BackupsController {
	return &BackupsController{backups: &BackupService{desks: &BackupDeskService{}, failures: setup.Failures(), views: &BackupViewService{}}}
}

func (setup BackupsTestSetup) EventsController() *EventsController {
	return &EventsController{events: &BackupEventsService{desks: &BackupDeskService{}, failures: setup.Failures()}}
}

func (setup BackupsTestSetup) Failures() *BackupFailureService {
	return &BackupFailureService{failures: &FailureService{}}
}

func (setup BackupsTestSetup) MemberEvent(eventId string, domainId string, memberId string, state string) EventEnvelope {
	payload, _ := json.Marshal(map[string]string{"memberId": memberId, "displayName": memberId, "primaryAddress": memberId + "@" + domainId, "state": state})
	return EventEnvelope{EventId: eventId, Type: "member.upserted", DomainId: domainId, Producer: "directory", OccurredAt: time.Now().UTC().Format(time.RFC3339), Payload: payload}
}
