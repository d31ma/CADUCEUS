import (
	"cmp"
	"slices"
	"strings"
)

// TemplateEditorService manages reusable rich composer templates with
// editable blanks, kept in name order.
// @Service
type TemplateEditorService struct {
	// @Needs
	templatesClient           *TemplatesClient
	applicationEventPublisher ApplicationEventPublisher
	authorization             string
	Templates                 []MessageTemplate
	// @SessionState
	EditID string
	// @SessionState
	Draft  MessageTemplate
	Error  string
	Saving bool
}

// @OnStart
func (service *TemplateEditorService) Start() {
	service.Templates = []MessageTemplate{}
}

// @OnEvent({"replay":true})
func (service *TemplateEditorService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	FlowUtils{}.When(changed.SignedIn, service.fetch)()
}

func (service *TemplateEditorService) Edit(id string) {
	index := slices.IndexFunc(service.Templates, func(template MessageTemplate) bool { return template.ID == id })
	service.load(append(slices.Clone(service.Templates), MessageTemplate{})[cmp.Or(index+1, len(service.Templates)+1)-1])
}

func (service *TemplateEditorService) New() {
	service.load(MessageTemplate{})
}

// SetField records one edited field of the template draft.
func (service *TemplateEditorService) SetField(name string, value string) {
	fields := map[string]any{}
	PreferenceUtils{}.Decode(service.Draft, &fields)
	fields[name] = value
	PreferenceUtils{}.Decode(fields, &service.Draft)
}

func (service *TemplateEditorService) Save() error {
	complete := ChoiceUtils{}.All(strings.TrimSpace(service.Draft.Name) != "", strings.TrimSpace(service.Draft.Text) != "")
	FlowUtils{}.Either(complete, service.save, func() { service.Error = "Name and template content are required." })()
	return nil
}

// @OnEvent
func (service *TemplateEditorService) Delete(given ConfirmationGiven) {
	FlowUtils{}.When(given.Action == "template-remove", func() { service.remove(given.ID) })()
	service.announce(false)
}

func (service *TemplateEditorService) fetch() {
	templates, failure := service.templatesClient.Templates(service.authorization)
	service.Templates = service.sorted(append([]MessageTemplate{}, templates...))
	service.announce(HTTPUtils{}.Refused(failure))
}

func (service *TemplateEditorService) load(template MessageTemplate) {
	service.EditID = template.ID
	service.Draft = template
	service.Error = ""
}

func (service *TemplateEditorService) save() {
	draft := service.Draft
	service.Saving = true
	service.Error = ""
	request := map[string]any{"name": strings.TrimSpace(draft.Name), "subject": draft.Subject, "to": draft.To, "cc": draft.Cc, "text": draft.Text}
	saved, failure := service.send(request)
	service.Saving = false
	service.showSaved(saved, failure)
}

func (service *TemplateEditorService) send(request BehaviorValue) (MessageTemplate, error) {
	if service.EditID == "" {
		return service.templatesClient.Create(service.authorization, request)
	}
	return service.templatesClient.Update(service.authorization, service.EditID, request)
}

func (service *TemplateEditorService) showSaved(saved MessageTemplate, failure error) {
	if failure != nil {
		service.Error = HTTPUtils{}.Message(failure, "Failed to save template.", service.network())
		return
	}
	others := slices.DeleteFunc(slices.Clone(service.Templates), func(template MessageTemplate) bool { return template.ID == saved.ID })
	service.Templates = service.sorted(append(others, saved))
	service.load(saved)
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Template “" + saved.Name + "” saved."}, false)
}

func (service *TemplateEditorService) remove(id string) {
	_, failure := service.templatesClient.Delete(service.authorization, id)
	FlowUtils{}.Either(failure == nil, func() {
		service.Templates = slices.DeleteFunc(slices.Clone(service.Templates), func(template MessageTemplate) bool { return template.ID == id })
		FlowUtils{}.When(service.EditID == id, service.New)()
		service.applicationEventPublisher.PublishEvent(ToastRequested{Message: "Template removed."}, false)
	}, func() { service.Error = HTTPUtils{}.Message(failure, "Failed to delete template.", service.network()) })()
}

func (service *TemplateEditorService) sorted(templates []MessageTemplate) []MessageTemplate {
	slices.SortStableFunc(templates, func(left MessageTemplate, right MessageTemplate) int { return strings.Compare(left.Name, right.Name) })
	return templates
}

func (service *TemplateEditorService) announce(refused bool) {
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "templates", Refused: refused}, false)
}

func (service *TemplateEditorService) network() string { return "Network error." }
