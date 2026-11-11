import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// @TestSetup
type ComposerTestSetup struct{}

func (setup ComposerTestSetup) Configure(execution *BehaviorExecution) (*BehaviorConfiguration, error) {
	configuration := NewBehaviorConfiguration()
	for _, name := range []string{"identity", "IdentityClient", "directory", "DirectoryClient", "messagingSend", "MessagingSendClient", "attachments", "AttachmentsClient"} {
		configuration.Http[name] = setup
	}
	return configuration, nil
}

func (setup ComposerTestSetup) Exchange(request *BehaviorHttpRequest, execution *BehaviorExecution) (BehaviorValue, error) {
	switch request.Method + " " + request.Path {
	case "POST /api/v1/identity/session":
		return setup.session("access-1"), nil
	case "POST /api/v1/identity/session/refresh":
		return setup.session("access-2"), nil
	case "POST /api/v1/directory/recipients/resolve":
		return setup.resolve(request)
	case "POST /api/v1/messaging/messages":
		return setup.send(request)
	case "POST /api/v1/attachments":
		return setup.createUpload(request)
	}
	return setup.completeUpload(request)
}

func (setup ComposerTestSetup) session(accessToken string) Session {
	me := Me{MemberId: "m-1", DomainId: "example.com", Username: "ana", DisplayName: "Ana Ruiz", PrimaryAddress: "ana@example.com", Roles: []string{}}
	return Session{Me: me, Domains: []DomainChoice{{DomainId: me.DomainId, Default: true}}, AccessToken: accessToken, ExpiresIn: 60, SessionKey: "key-1"}
}

func (setup ComposerTestSetup) resolve(request *BehaviorHttpRequest) (BehaviorValue, error) {
	if !strings.HasPrefix(fmt.Sprint(request.Headers["Authorization"]), "Bearer access-") {
		return nil, errors.New("unauthenticated")
	}
	body := ResolveRequest{}
	setup.decode(request, &body)
	recipients := []ResolvedRecipient{}
	for _, address := range body.Addresses {
		recipients = append(recipients, setup.resolved(address))
	}
	return ResolveResponse{Recipients: recipients}, nil
}

func (setup ComposerTestSetup) resolved(address string) ResolvedRecipient {
	member := []string{"chat", "email", "both"}
	switch {
	case address == "bo@example.com":
		return ResolvedRecipient{Address: address, DisplayName: "Bo Lee", Kind: "member", MemberId: "m-2", DefaultDelivery: "chat", AllowedDeliveries: member}
	case address == "releases@example.com":
		return ResolvedRecipient{Address: address, DisplayName: "#releases", Kind: "channel", ChannelId: "c-1", DefaultDelivery: "chat", AllowedDeliveries: []string{"chat"}}
	case strings.HasSuffix(address, "@example.org"):
		return ResolvedRecipient{Address: address, Kind: "otherDomain", DefaultDelivery: "email", AllowedDeliveries: []string{"email"}}
	}
	return ResolvedRecipient{Address: address, Kind: "external", DefaultDelivery: "email", AllowedDeliveries: []string{"email"}}
}

func (setup ComposerTestSetup) send(request *BehaviorHttpRequest) (BehaviorValue, error) {
	body := SendRequest{}
	setup.decode(request, &body)
	if body.Body == "offline" {
		return nil, errors.New("offline")
	}
	message := SendResultMessage{MessageId: "msg-" + body.ClientMessageId, ConversationId: "conv-1"}
	return SendResult{Message: message, Deliveries: []SendDelivery{}}, nil
}

func (setup ComposerTestSetup) createUpload(request *BehaviorHttpRequest) (BehaviorValue, error) {
	body := UploadCreateRequest{}
	setup.decode(request, &body)
	if body.FileName == "broken.pdf" {
		return nil, errors.New("unavailable")
	}
	return Upload{AttachmentId: "att-" + body.FileName, UploadUrl: "/blobs/" + body.FileName, ExpiresAt: "2026-11-11T00:00:00Z"}, nil
}

func (setup ComposerTestSetup) completeUpload(request *BehaviorHttpRequest) (BehaviorValue, error) {
	if !strings.HasPrefix(request.Path, "/api/v1/attachments/") {
		return nil, errors.New("no fake answer for " + request.Method + " " + request.Path)
	}
	attachmentId := strings.TrimSuffix(strings.TrimPrefix(request.Path, "/api/v1/attachments/"), "/complete")
	return UploadComplete{AttachmentId: attachmentId, State: "ready"}, nil
}

func (setup ComposerTestSetup) decode(request *BehaviorHttpRequest, target any) {
	text, _ := json.Marshal(request.Body)
	_ = json.Unmarshal(text, target)
}

func (setup ComposerTestSetup) Auth(behaviors *BehaviorContext) *ComposerAuthService {
	BrowserFakes{}.Answer("clock", "now", 1000)
	tokens := &SessionTokenService{identity: NewIdentityClientWith(behaviors)}
	signedIn, _ := tokens.SignIn(SessionSignInRequest{Login: "ana", Password: "right"})
	accounts := &SessionAccountService{}
	accounts.Apply(signedIn)
	session := &SessionService{tokens: tokens, accounts: accounts, problems: &SessionProblemService{}}
	return &ComposerAuthService{session: session, accounts: accounts}
}

func (setup ComposerTestSetup) Resolve(behaviors *BehaviorContext) *ComposerResolveService {
	return &ComposerResolveService{directory: NewDirectoryClientWith(behaviors), auth: setup.Auth(behaviors), words: &ComposerWordsService{}}
}

func (setup ComposerTestSetup) Recipients(behaviors *BehaviorContext) *ComposerRecipientService {
	resolve := setup.Resolve(behaviors)
	return &ComposerRecipientService{resolve: resolve, words: resolve.words, auth: resolve.auth}
}

func (setup ComposerTestSetup) Sender(behaviors *BehaviorContext) *ComposerSendService {
	return &ComposerSendService{messaging: NewMessagingSendClientWith(behaviors), auth: setup.Auth(behaviors)}
}

func (setup ComposerTestSetup) Uploads(behaviors *BehaviorContext) *ComposerUploadService {
	return &ComposerUploadService{attachments: NewAttachmentsClientWith(behaviors), auth: setup.Auth(behaviors)}
}

func (setup ComposerTestSetup) Composer(behaviors *BehaviorContext) *ComposerService {
	BrowserFakes{}.Answer("secureRandom", "uuid", "id-1")
	composer := &ComposerService{recipients: setup.Recipients(behaviors), send: setup.Sender(behaviors), problems: &ComposerProblemService{}, uploads: setup.Uploads(behaviors)}
	composer.Start()
	return composer
}

func (setup ComposerTestSetup) Thread(behaviors *BehaviorContext) *ComposerThreadService {
	BrowserFakes{}.Answer("secureRandom", "uuid", "id-1")
	sender := setup.Sender(behaviors)
	return &ComposerThreadService{send: sender, problems: &ComposerProblemService{}, auth: sender.auth}
}
