import (
	"cmp"
	"slices"
	"strings"
)

// InboxRulesService lists the account's inbox rules in processing order and
// builds, previews, saves, reorders, enables and removes them.
// @Service
type InboxRulesService struct {
	// @Needs
	rulesClient               *RulesClient
	applicationEventPublisher ApplicationEventPublisher
	senders                   []string
	authorization             string
	sequence                  float64
	Rules                     []InboxRule
	// @SessionState
	Open bool
	// @SessionState
	EditID string
	// @SessionState
	Draft InboxRule
	// @SessionState
	Conditions []DraftCondition
	// @SessionState
	Actions []DraftAction
	// @SessionState
	Builder map[string]string
	// @SessionState
	PreviewAttachment bool
	PreviewResult     string
}

// @OnStart
func (service *InboxRulesService) Start() {
	service.Rules = []InboxRule{}
	FlowUtils{}.When(service.Builder == nil, service.clearForm)()
}

// @OnEvent({"replay":true})
func (service *InboxRulesService) HearSession(changed SessionChanged) {
	service.authorization = "Bearer " + changed.Token
	refused := false
	FlowUtils{}.When(changed.SignedIn, func() { refused = service.reload() })()
	service.announce(refused)
}

// @OnEvent({"replay":true})
func (service *InboxRulesService) Learn(addresses SenderAddresses) {
	service.senders = addresses.Addresses
}

// Rows are the saved rules with their readable conditions and actions.
func (service *InboxRulesService) Rows() []RuleRow {
	return service.rows()
}

func (service *InboxRulesService) ToggleForm() {
	service.Open = !service.Open
	FlowUtils{}.Either(service.Open, func() {
		service.Draft.Mailbox = cmp.Or(service.Draft.Mailbox, PreferenceUtils{}.Joined(service.firstSender()))
	}, service.clearForm)()
}

// SetField records one builder field, such as the condition value or the folder.
func (service *InboxRulesService) SetField(name string, value string) {
	service.Builder[name] = value
}

func (service *InboxRulesService) SetName(name string) {
	service.Draft.Name = name
}

func (service *InboxRulesService) SetMailbox(mailbox string) {
	service.Draft.Mailbox = mailbox
}

func (service *InboxRulesService) SetMatch(match string) {
	service.Draft.ConditionMatch = match
}

func (service *InboxRulesService) SetStop(stop bool) {
	service.Draft.StopProcessing = stop
}

func (service *InboxRulesService) AddCondition() {
	value := strings.TrimSpace(service.Builder[service.names().Value])
	field := service.Builder[service.names().Field]
	flag := slices.Contains([]string{"true", "false"}, strings.ToLower(value))
	invalid := ChoiceUtils{}.All(field == "hasAttachment", !flag)
	FlowUtils{}.When(value != "", func() {
		FlowUtils{}.Either(invalid, func() { service.toast("Has attachment must be true or false.") }, func() { service.addCondition(field, value) })()
	})()
}

func (service *InboxRulesService) RemoveCondition(key float64) {
	service.Conditions = slices.DeleteFunc(slices.Clone(service.Conditions), func(condition DraftCondition) bool { return condition.Key == key })
}

func (service *InboxRulesService) AddAction() {
	action, problem := service.action(service.Builder[service.names().Action])
	FlowUtils{}.Either(problem == "", func() { service.addAction(action) }, func() { service.toast(problem) })()
}

func (service *InboxRulesService) RemoveAction(key float64) {
	service.Actions = slices.DeleteFunc(slices.Clone(service.Actions), func(action DraftAction) bool { return action.Key == key })
}

func (service *InboxRulesService) Reset() {
	service.clearForm()
}

func (service *InboxRulesService) Edit(id string) {
	rule := service.rule(id)
	service.EditID = rule.ID
	service.Draft = rule
	service.Draft.Mailbox = cmp.Or(rule.Mailbox, PreferenceUtils{}.Joined(service.firstSender()))
	any := "any"
	service.Draft.ConditionMatch = ChoiceUtils{}.Text(rule.ConditionMatch == any, any, service.names().Every)
	service.Conditions = service.draftConditions(rule.Conditions)
	service.Actions = service.draftActions(rule.Actions)
	service.Open = true
	service.PreviewResult = ""
}

func (service *InboxRulesService) Save() error {
	rule := service.built()
	complete := ChoiceUtils{}.All(ChoiceUtils{}.All(rule.Name != "", rule.Mailbox != ""), len(rule.Actions) > 0)
	FlowUtils{}.Either(complete, func() { service.save(rule) }, func() { service.toast("Name, mailbox, and at least one action are required.") })()
	return nil
}

// @OnEvent
func (service *InboxRulesService) Delete(given ConfirmationGiven) {
	FlowUtils{}.When(given.Action == "rule-remove", func() { service.remove(given.ID) })()
	service.announce(false)
}

func (service *InboxRulesService) Toggle(id string) error {
	rule := service.rule(id)
	_, _ = service.rulesClient.Update(service.authorization, id, map[string]any{service.names().Enabled: !rule.Enabled})
	service.reload()
	return nil
}

// Move puts one rule earlier (-1) or later (1) in processing order.
func (service *InboxRulesService) Move(id string, delta float64) error {
	index := slices.IndexFunc(service.Rules, func(rule InboxRule) bool { return rule.ID == id })
	target := index + int(delta)
	inside := ChoiceUtils{}.All(index >= 0, ChoiceUtils{}.All(target >= 0, target < len(service.Rules)))
	FlowUtils{}.When(inside, func() { service.move(index, target) })()
	return nil
}

func (service *InboxRulesService) Preview() error {
	rule := service.built()
	complete := ChoiceUtils{}.All(ChoiceUtils{}.All(rule.Name != "", rule.Mailbox != ""), len(rule.Actions) > 0)
	FlowUtils{}.Either(complete, func() { service.preview(rule) }, func() { service.PreviewResult = "Add a name, mailbox, and action before previewing." })()
	return nil
}

// reload reads the rules and says whether the server refused the read.
func (service *InboxRulesService) reload() bool {
	rules, failure := service.rulesClient.Rules(service.authorization)
	FlowUtils{}.When(failure == nil, func() { service.Rules = append([]InboxRule{}, rules...) })()
	return HTTPUtils{}.Refused(failure)
}

func (service *InboxRulesService) rows() []RuleRow {
	rows := []RuleRow{}
	for _, rule := range service.Rules {
		rows = append(rows, RuleRow{
			ID: rule.ID, Name: rule.Name, Mailbox: rule.Mailbox, Enabled: rule.Enabled,
			ConditionsLabel: RuleDescriptionUtils{}.Conditions(rule),
			ActionsLabel:    RuleDescriptionUtils{}.Actions(rule.Actions) + ChoiceUtils{}.Text(rule.StopProcessing, " · then stop", ""),
		})
	}
	return rows
}

func (service *InboxRulesService) clearForm() {
	service.EditID = ""
	service.Draft = InboxRule{ConditionMatch: service.names().Every}
	service.Conditions = []DraftCondition{}
	service.Actions = []DraftAction{}
	service.Builder = map[string]string{service.names().Field: "from", service.names().Operator: "contains", service.names().Value: "", service.names().Action: service.names().Folder, service.names().Folder: "", service.names().To: "", service.names().SampleFrom: "", service.names().SampleSubject: "", service.names().SampleBody: ""}
	service.PreviewAttachment = false
	service.PreviewResult = ""
	service.Open = false
}

func (service *InboxRulesService) addCondition(field string, value string) {
	service.sequence += 1
	condition := RuleCondition{Field: field, Op: service.Builder[service.names().Operator], Value: value}
	service.Conditions = append(slices.Clone(service.Conditions), DraftCondition{Key: service.sequence, Field: condition.Field, Op: condition.Op, Value: value, Label: RuleDescriptionUtils{}.Condition(condition)})
	service.Builder[service.names().Value] = ""
}

// action builds the chosen action, or names what is missing.
func (service *InboxRulesService) action(kind string) (RuleAction, string) {
	folder := RuleDescriptionUtils{}.FolderName(service.Builder[service.names().Folder])
	forward := PreferenceUtils{}.Lower(service.Builder[service.names().To])
	switch kind {
	case service.names().Folder:
		return RuleAction{Type: kind, Folder: folder}, ChoiceUtils{}.Text(folder == "", "Enter a folder name.", "")
	case "forward":
		return RuleAction{Type: kind, To: forward}, ChoiceUtils{}.Text(forward == "", "Enter a forwarding address.", "")
	case "markReadTrue", "markReadFalse", "starTrue", "starFalse":
		enabled := "True"
		value := strings.HasSuffix(kind, enabled)
		return RuleAction{Type: strings.TrimSuffix(strings.TrimSuffix(kind, enabled), "False"), Value: &value}, ""
	}
	return RuleAction{Type: "delete"}, ""
}

func (service *InboxRulesService) addAction(action RuleAction) {
	service.sequence += 1
	service.Actions = append(slices.Clone(service.Actions), DraftAction{Key: service.sequence, Action: action, Label: RuleDescriptionUtils{}.Action(action)})
	service.Builder[service.names().Folder] = ""
	service.Builder[service.names().To] = ""
}

func (service *InboxRulesService) built() InboxRule {
	rule := service.Draft
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Mailbox = PreferenceUtils{}.Lower(rule.Mailbox)
	rule.Enabled = true
	rule.Conditions = service.ruleConditions()
	rule.Actions = service.ruleActions()
	return rule
}

func (service *InboxRulesService) save(rule InboxRule) {
	service.Draft.Name = rule.Name
	service.Draft.Mailbox = rule.Mailbox
	body := service.ruleBody(rule)
	updated := service.EditID != ""
	_, failure := service.send(body)
	FlowUtils{}.Either(failure == nil, func() {
		service.reload()
		service.toast(ChoiceUtils{}.Text(updated, "Rule updated.", "Rule saved."))
		service.clearForm()
	}, func() { service.toast(HTTPUtils{}.Message(failure, "Failed to save rule.", "Network error.")) })()
}

func (service *InboxRulesService) send(body BehaviorValue) (BehaviorValue, error) {
	if service.EditID != "" {
		return service.rulesClient.Update(service.authorization, service.EditID, body)
	}
	return service.rulesClient.Create(service.authorization, body)
}

func (service *InboxRulesService) remove(id string) {
	_, failure := service.rulesClient.Delete(service.authorization, id)
	FlowUtils{}.When(failure == nil, func() {
		service.Rules = slices.DeleteFunc(slices.Clone(service.Rules), func(rule InboxRule) bool { return rule.ID == id })
		service.toast("Rule deleted.")
	})()
}

func (service *InboxRulesService) move(index int, target int) {
	next := slices.Clone(service.Rules)
	moved := next[index]
	next = slices.Insert(slices.Delete(next, index, index+1), target, moved)
	ids := []string{}
	for _, rule := range next {
		ids = append(ids, rule.ID)
	}
	_, failure := service.rulesClient.Order(service.authorization, map[string]any{"ids": ids})
	FlowUtils{}.Either(failure == nil, func() { service.Rules = next }, func() { service.toast("Could not reorder rules.") })()
}

func (service *InboxRulesService) preview(rule InboxRule) {
	message := map[string]any{
		"from": service.Builder[service.names().SampleFrom], service.names().To: rule.Mailbox, "subject": service.Builder[service.names().SampleSubject],
		"body": service.Builder[service.names().SampleBody], "hasAttachment": service.PreviewAttachment,
	}
	result, failure := service.rulesClient.Preview(service.authorization, map[string]any{"rule": service.ruleBody(rule), "message": message})
	matched := ChoiceUtils{}.Text(result.Matched, "Match · "+RuleDescriptionUtils{}.Actions(result.PlannedActions), "No match · no actions would run")
	service.PreviewResult = ChoiceUtils{}.Text(failure == nil, matched, "Preview failed.")
}

func (service *InboxRulesService) ruleBody(rule InboxRule) map[string]any {
	return map[string]any{
		"name": rule.Name, "mailbox": rule.Mailbox, service.names().Enabled: true, "stopProcessing": rule.StopProcessing,
		"conditionMatch": rule.ConditionMatch, "conditions": rule.Conditions, "actions": rule.Actions,
	}
}

func (service *InboxRulesService) ruleConditions() []RuleCondition {
	conditions := []RuleCondition{}
	for _, condition := range service.Conditions {
		conditions = append(conditions, RuleCondition{Field: condition.Field, Op: condition.Op, Value: condition.Value})
	}
	return conditions
}

func (service *InboxRulesService) ruleActions() []RuleAction {
	actions := []RuleAction{}
	for _, action := range service.Actions {
		actions = append(actions, action.Action)
	}
	return actions
}

func (service *InboxRulesService) draftConditions(conditions []RuleCondition) []DraftCondition {
	drafts := []DraftCondition{}
	for _, condition := range conditions {
		service.sequence += 1
		drafts = append(drafts, DraftCondition{Key: service.sequence, Field: condition.Field, Op: condition.Op, Value: condition.Value, Label: RuleDescriptionUtils{}.Condition(condition)})
	}
	return drafts
}

func (service *InboxRulesService) draftActions(actions []RuleAction) []DraftAction {
	drafts := []DraftAction{}
	for _, action := range actions {
		service.sequence += 1
		drafts = append(drafts, DraftAction{Key: service.sequence, Action: action, Label: RuleDescriptionUtils{}.Action(action)})
	}
	return drafts
}

func (service *InboxRulesService) rule(id string) InboxRule {
	index := slices.IndexFunc(service.Rules, func(rule InboxRule) bool { return rule.ID == id })
	return append(slices.Clone(service.Rules), InboxRule{})[cmp.Or(index+1, len(service.Rules)+1)-1]
}

func (service *InboxRulesService) firstSender() []string {
	return service.senders[:min(1, len(service.senders))]
}

func (service *InboxRulesService) toast(message string) {
	service.applicationEventPublisher.PublishEvent(ToastRequested{Message: message}, false)
}

func (service *InboxRulesService) announce(refused bool) {
	service.applicationEventPublisher.PublishEvent(SettingsActivity{Section: "inbox-rules", Refused: refused}, false)
}

func (service *InboxRulesService) names() RuleBuilderNames {
	return RuleBuilderNames{
		Field:         "field",
		Operator:      "op",
		Every:         "all",
		Enabled:       "enabled",
		SampleFrom:    "previewFrom",
		SampleSubject: "previewSubject",
		SampleBody:    "previewBody",
		Value:         "value",
		Action:        "action",
		Folder:        "folder",
		To:            "to",
	}
}
