import (
	"cmp"
	"slices"
	"strings"
)

// SignatureEditorService manages the account's rich signatures and the
// exact sender addresses each one is the default for. The preview shows
// only sanitized markup.
// @Service
type SignatureEditorService struct {
	// @Needs
	signaturesClient          *SignaturesClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	Signatures                []MailSignature
	// @SessionState
	EditID string
	// @SessionState
	Name string
	// @SessionState
	Text string
	// @SessionState
	Defaults []string
	Error    string
	Saving   bool
}

// @OnStart
func (service *SignatureEditorService) Start() {
	service.Signatures = []MailSignature{}
}

// @OnEvent({"replay":true})
func (service *SignatureEditorService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	FlowUtils{}.When(changed.SignedIn, service.fetch)()
}

func (service *SignatureEditorService) Edit(id string) {
	service.load(service.signature(id))
}

func (service *SignatureEditorService) New() {
	service.load(MailSignature{})
}

func (service *SignatureEditorService) ToggleDefault(address string) {
	service.Defaults = PreferenceUtils{}.Toggle(service.Defaults, address)
}

// Preview is the sanitized signature markup that the preview frame shows.
func (service *SignatureEditorService) Preview() string {
	return SignatureUtils{}.Sanitize(service.Text)
}

func (service *SignatureEditorService) Save() error {
	name := strings.TrimSpace(service.Name)
	complete := ChoiceUtils{}.All(name != "", strings.TrimSpace(service.Text) != "")
	FlowUtils{}.Either(complete, func() { service.save(name) }, func() { service.Error = "Name and signature content are required." })()
	return nil
}

// @OnEvent
func (service *SignatureEditorService) Delete(given ConfirmationGiven) {
	FlowUtils{}.When(given.Action == "signature-remove", func() { service.remove(given.ID) })()
	service.announce(false)
}

func (service *SignatureEditorService) fetch() {
	signatures, failure := service.signaturesClient.Signatures(service.authorization)
	service.Signatures = append([]MailSignature{}, signatures...)
	service.announce(HTTPUtils{}.Refused(failure))
}

func (service *SignatureEditorService) load(signature MailSignature) {
	service.EditID = signature.ID
	service.Name = signature.Name
	service.Text = signature.Text
	service.Defaults = append([]string{}, signature.DefaultFor...)
	service.Error = ""
}

func (service *SignatureEditorService) save(name string) {
	service.Name = name
	service.Saving = true
	service.Error = ""
	body := map[string]any{"name": name, "text": service.Text, "defaultFor": service.Defaults}
	updated := service.EditID != ""
	saved, failure := service.send(body)
	service.Saving = false
	FlowUtils{}.When(failure == nil, func() { service.Signatures = service.replaced(slices.Clone(service.Signatures), saved) })()
	service.showSaved(saved, failure, updated)
}

func (service *SignatureEditorService) send(body BehaviorValue) (MailSignature, error) {
	if service.EditID != "" {
		return service.signaturesClient.Update(service.authorization, service.EditID, body)
	}
	return service.signaturesClient.Create(service.authorization, body)
}

func (service *SignatureEditorService) showSaved(saved MailSignature, failure error, updated bool) {
	if failure != nil {
		service.Error = HTTPUtils{}.Message(failure, "Failed to save signature.", "Network error.")
		return
	}
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: ChoiceUtils{}.Text(updated, "Signature updated.", "Signature saved.")}, false)
	service.load(saved)
}

func (service *SignatureEditorService) replaced(signatures []MailSignature, saved MailSignature) []MailSignature {
	index := slices.IndexFunc(signatures, func(signature MailSignature) bool { return signature.ID == saved.ID })
	if index < 0 {
		return append(signatures, saved)
	}
	signatures[index] = saved
	return signatures
}

func (service *SignatureEditorService) remove(id string) {
	_, failure := service.signaturesClient.Delete(service.authorization, id)
	FlowUtils{}.When(failure == nil, func() {
		service.Signatures = slices.DeleteFunc(slices.Clone(service.Signatures), func(signature MailSignature) bool { return signature.ID == id })
		FlowUtils{}.When(service.EditID == id, service.New)()
		service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Signature removed."}, false)
	})()
}

func (service *SignatureEditorService) signature(id string) MailSignature {
	index := slices.IndexFunc(service.Signatures, func(signature MailSignature) bool { return signature.ID == id })
	return append(slices.Clone(service.Signatures), MailSignature{})[cmp.Or(index+1, len(service.Signatures)+1)-1]
}

func (service *SignatureEditorService) announce(refused bool) {
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "signatures", Refused: refused}, false)
}
