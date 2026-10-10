import (
	"regexp"
	"strings"
)

// TemplateService loads, filters, inserts and saves the mailbox's message
// templates. The editor inserts a template before the signature and quoted
// mail, and it answers the body of a template the person saves, because
// message HTML never enters Go.
//
// @Service
type TemplateService struct {
	// @Needs
	templatesClient           *TemplatesClient
	applicationEventPublisher ApplicationEventPublisher

	Templates []ComposeTemplate
	Visible   []ComposeTemplate

	token   string
	pending []TemplateInput
}

// @OnStart
func (template *TemplateService) Start() {
	template.Templates = []ComposeTemplate{}
	template.Visible = []ComposeTemplate{}
}

// SessionChanged loads the templates of the signed-in mailbox.
//
// @OnEvent({"replay":true})
func (template *TemplateService) SessionChanged(changed SessionChanged) {
	template.token = changed.Token
	template.reload()
}

// Filter prefix-filters the template picker.
func (template *TemplateService) Filter(query string) {
	template.Visible = template.startingWith(strings.ToLower(strings.TrimSpace(query)))
}

// Load asks the editor to insert a template. The editor asks for
// confirmation when the draft already has a recipient, a subject or text.
func (template *TemplateService) Load(id string, to string, subject string) {
	template.insert(template.named(id), to, subject)
}

// Save asks the editor for the template body and a name.
func (template *TemplateService) Save(subject string, to string, cc string) {
	template.pending = []TemplateInput{{Subject: subject, To: to, Cc: cc}}
	template.publish(TemplateDraftRequested{SuggestedName: template.suggestedName(subject)})
}

// TemplateDraftReady saves the editor's template body under its name.
//
// @OnEvent
func (template *TemplateService) TemplateDraftReady(ready TemplateDraftReady) {
	template.storePending(ready)
	template.pending = nil
	template.publish(ComposeChanged{Reason: "changed"})
}

// Manage opens settings, where templates are managed.
func (template *TemplateService) Manage() {
	template.publish(NavigationRequested{Route: "settings"})
}

func (template *TemplateService) publish(value any) {
	template.applicationEventPublisher.PublishEvent(value, false)
}

func (template *TemplateService) authorization() string {
	return "Bearer " + template.token
}

func (template *TemplateService) reload() {
	templates, problem := template.templatesClient.List(template.authorization())
	template.Templates = template.listedOr(templates, problem)
	template.Visible = append([]ComposeTemplate{}, template.Templates...)
}

func (template *TemplateService) listedOr(templates []ComposeTemplate, problem error) []ComposeTemplate {
	if problem == nil {
		return append([]ComposeTemplate{}, templates...)
	}
	return template.Templates
}

func (template *TemplateService) startingWith(query string) []ComposeTemplate {
	visible := []ComposeTemplate{}
	for _, candidate := range template.Templates {
		visible = append(visible, TemplateService{}.whenStartsWith(candidate, query)...)
	}
	return visible
}

func (TemplateService) whenStartsWith(candidate ComposeTemplate, query string) []ComposeTemplate {
	if strings.HasPrefix(strings.ToLower(candidate.Name), query) {
		return []ComposeTemplate{candidate}
	}
	return nil
}

func (template *TemplateService) named(id string) []ComposeTemplate {
	found := []ComposeTemplate{}
	for _, candidate := range template.Templates {
		found = append(found, TemplateService{}.whenNamed(candidate, id)...)
	}
	return found[:min(len(found), 1)]
}

func (TemplateService) whenNamed(candidate ComposeTemplate, id string) []ComposeTemplate {
	if candidate.ID == id && id != "" {
		return []ComposeTemplate{candidate}
	}
	return nil
}

func (template *TemplateService) insert(found []ComposeTemplate, to string, subject string) {
	for _, chosen := range found {
		template.publish(TemplateInsertRequested{TemplateID: chosen.ID, Name: chosen.Name, Text: chosen.Text, To: chosen.To, Cc: chosen.Cc, Subject: chosen.Subject, Confirm: TemplateService{}.hasHeaders(to, subject)})
	}
}

func (TemplateService) hasHeaders(to string, subject string) bool {
	if to != "" || subject != "" {
		return true
	}
	return false
}

func (TemplateService) suggestedName(subject string) string {
	name := []rune(strings.TrimSpace(regexp.MustCompile(`[^\p{L}\p{N}_\- ]+`).ReplaceAllString(subject, "")))
	return string(name[:min(len(name), 100)])
}

func (template *TemplateService) storePending(ready TemplateDraftReady) {
	for _, input := range template.pending {
		template.store(input, ready)
	}
}

func (template *TemplateService) store(input TemplateInput, ready TemplateDraftReady) {
	input.Name = strings.TrimSpace(ready.Name)
	input.Text = ready.Text
	saved, problem := template.templatesClient.Save(input, template.authorization())
	template.publish(ToastRequested{Message: TemplateService{}.savedMessage(saved, problem)})
	template.reload()
}

func (TemplateService) savedMessage(saved SavedTemplate, problem error) string {
	if problem == nil {
		return "Template “" + saved.Name + "” saved."
	} else if BehaviorErrorCode(problem) == "HTTP_STATUS" {
		return "Failed to save template."
	}
	return "Network error."
}
