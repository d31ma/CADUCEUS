import (
	"regexp"
	"strings"
)

// MailActions applies, undoes and redoes reversible bulk mail actions. Each
// action is journaled before it changes mail, so an interrupted transition
// is finished (or failed) before the owner's next action.
type MailActions struct {
	Actions    *MailActionsHelper
	Emails     *EmailsHelper
	Categories *CategoryMappingService
	Provider   UnifiedMail
}

// Apply journals and applies one action, or replays the action an earlier
// request with the same client request id made.
func (actions MailActions) Apply(caller RouteCaller, input MailActionInput, now int64) (map[string]any, error) {
	rules := MailActionRules{}
	if _, err := actions.reconcile(caller); err != nil {
		return nil, err
	}
	fingerprint := rules.Fingerprint(input.Ids, input.Patch)
	if input.ClientRequestId != "" {
		existingId, existing, err := actions.Actions.FindByClientRequestId(caller.Email, input.ClientRequestId)
		if err != nil {
			return nil, err
		}
		if existingId != "" && existing != nil {
			return actions.replayed(caller, existingId, existing, fingerprint)
		}
	}
	messages, err := actions.authorized(caller, input.Ids)
	if err != nil {
		return nil, err
	}
	changes, err := actions.built(caller, messages, input.Patch)
	if err != nil {
		return nil, err
	}
	if len(changes) == 0 {
		return nil, &MailActionError{Message: "Messages already match this action", Status: 409}
	}
	owner := strings.ToLower(caller.Email)
	record := map[string]any{
		"id": "mail-action-" + Security{}.RandomUuid(), "ownerEmail": owner, "domain": strings.SplitN(owner+"@", "@", 3)[1],
		"kind": rules.Kind(input.Patch, len(changes)), "status": "prepared", "changesJson": TextValue{}.Json(changes),
		"createdAt": now, "updatedAt": now, "expiresAt": now + rules.TimeToLive(), "lastError": "",
	}
	if input.ClientRequestId != "" {
		record["clientRequestId"] = input.ClientRequestId
		record["requestFingerprint"] = fingerprint
	}
	actionId, err := actions.Actions.Put(record)
	if err != nil {
		return nil, err
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(actionId) {
		return nil, &MailActionError{Message: "FYLO did not return a mail action identifier", Status: 503}
	}
	return actions.applied(caller, actionId, record, changes)
}

func (actions MailActions) replayed(caller RouteCaller, existingId string, existing map[string]any, fingerprint string) (map[string]any, error) {
	rules := MailActionRules{}
	if existing["requestFingerprint"] != fingerprint {
		return nil, &MailActionError{Message: "clientRequestId was already used for another action", Status: 409}
	}
	changes, err := rules.Changes(existing)
	if err == nil {
		err = rules.EnsureDomains(changes, caller.Domains)
	}
	if err != nil {
		return nil, err
	}
	messages, err := actions.authorized(caller, actions.ids(changes))
	if err != nil {
		return nil, err
	}
	target := "after"
	if existing["status"] == "undone" {
		target = "before"
	}
	accepted, err := actions.accepted(caller, messages, changes, target)
	if err != nil {
		return nil, err
	}
	if !accepted {
		return nil, &MailActionError{Message: "Messages changed after this action; no changes were overwritten", Status: 409}
	}
	return rules.Result(existingId, existing, TextValue{}.Of(existing["status"]), messages), nil
}

func (actions MailActions) applied(caller RouteCaller, actionId string, record map[string]any, changes []map[string]any) (map[string]any, error) {
	updated, err := actions.written(caller, changes, "after", actionId)
	if err == nil {
		err = actions.provider().Journal(caller, actionId, map[string]any{
			"status": "applied", "changesJson": TextValue{}.Json(changes), "updatedAt": EpochTime{}.Now(), "lastError": "",
		})
	}
	if err == nil {
		return MailActionRules{}.Result(actionId, record, "applied", updated), nil
	}
	if actions.rolledBack(caller, changes, actionId) {
		_ = actions.provider().Journal(caller, actionId, map[string]any{
			"status": "failed", "updatedAt": EpochTime{}.Now(), "lastError": TextValue{}.Prefix(err.Error(), 200),
		})
	}
	return nil, err
}

// Undo restores the states an applied action replaced.
func (actions MailActions) Undo(caller RouteCaller, id string, now int64) (map[string]any, error) {
	return actions.transitioned(caller, id, MailActionTransition{From: "applied", Intermediate: "undoing", To: "undone", Source: "after", Target: "before", Now: now})
}

// Redo applies an undone action again.
func (actions MailActions) Redo(caller RouteCaller, id string, now int64) (map[string]any, error) {
	return actions.transitioned(caller, id, MailActionTransition{From: "undone", Intermediate: "redoing", To: "applied", Source: "before", Target: "after", Now: now})
}

func (actions MailActions) transitioned(caller RouteCaller, id string, transition MailActionTransition) (map[string]any, error) {
	records, err := actions.reconcile(caller)
	if err != nil {
		return nil, err
	}
	documentId, record, err := actions.find(caller, id, records)
	if err != nil {
		return nil, err
	}
	rules := MailActionRules{}
	changes, err := rules.Changes(record)
	if err == nil {
		err = rules.EnsureDomains(changes, caller.Domains)
	}
	if err != nil {
		return nil, err
	}
	if (RecordValues{}).Numeric(record["expiresAt"]) < float64(transition.Now) {
		return nil, &MailActionError{Message: "The undo window has expired", Status: 409}
	}
	if record["status"] == transition.To {
		current, err := actions.authorized(caller, actions.ids(changes))
		if err != nil {
			return nil, err
		}
		if accepted, err := actions.accepted(caller, current, changes, transition.Target); err != nil || accepted {
			return rules.Result(id, record, transition.To, current), err
		}
	}
	if record["status"] != transition.From {
		return nil, &MailActionError{Message: "Mail action cannot be changed from " + TextValue{}.Of(record["status"]), Status: 409}
	}
	return actions.moved(caller, id, documentId, record, changes, transition)
}

func (actions MailActions) moved(caller RouteCaller, id string, documentId string, record map[string]any, changes []map[string]any, transition MailActionTransition) (map[string]any, error) {
	messages, err := actions.authorized(caller, actions.ids(changes))
	if err != nil {
		return nil, err
	}
	accepted, err := actions.accepted(caller, messages, changes, transition.Source)
	if err != nil {
		return nil, err
	}
	if !accepted {
		return nil, &MailActionError{Message: "Messages changed after this action; no changes were overwritten", Status: 409}
	}
	if err := actions.Actions.Update(documentId, map[string]any{"status": transition.Intermediate, "updatedAt": transition.Now}); err != nil {
		return nil, err
	}
	updated, err := actions.written(caller, changes, transition.Target, id)
	if err != nil {
		return nil, err
	}
	err = actions.Actions.Update(documentId, map[string]any{
		"status": transition.To, "changesJson": TextValue{}.Json(changes), "updatedAt": EpochTime{}.Now(), "lastError": "",
	})
	return MailActionRules{}.Result(id, record, transition.To, updated), err
}

// find answers the owner's journal entry with the document or logical id,
// from the records just reconciled or from the journal.
func (actions MailActions) find(caller RouteCaller, id string, records []FyloRecord) (string, map[string]any, error) {
	documentId, record := "", map[string]any(nil)
	for _, candidate := range records {
		if candidate.Id == id || candidate.Data["id"] == id {
			documentId, record = candidate.Id, candidate.Data
			break
		}
	}
	if documentId == "" {
		var err error
		if documentId, record, err = actions.Actions.FindById(id, caller.Email); err != nil {
			return "", nil, err
		}
	}
	owner, isText := record["ownerEmail"].(string)
	if documentId == "" || record == nil || !isText || !strings.EqualFold(owner, caller.Email) {
		return "", nil, &MailActionError{Message: "Mail action not found", Status: 404}
	}
	return documentId, record, nil
}

// reconcile finishes, or fails, every interrupted action of the owner and
// answers the owner's journal as it now stands.
func (actions MailActions) reconcile(caller RouteCaller) ([]FyloRecord, error) {
	owner := strings.ToLower(caller.Email)
	records, err := actions.Actions.ListForOwner(owner)
	if err != nil {
		return nil, err
	}
	rules := MailActionRules{}
	for _, record := range records {
		if !(TextValue{}).Includes(rules.IncompleteStatuses(), record.Data["status"]) {
			continue
		}
		status, changes, err := actions.recovered(caller, record)
		if err == nil {
			err = actions.Actions.Update(record.Id, map[string]any{
				"status": status, "changesJson": TextValue{}.Json(changes), "updatedAt": EpochTime{}.Now(), "lastError": "",
			})
		}
		if err != nil {
			_ = actions.Actions.Update(record.Id, map[string]any{"status": "failed", "updatedAt": EpochTime{}.Now(), "lastError": TextValue{}.Prefix(err.Error(), 200)})
			record.Data["status"] = "failed"
			continue
		}
		record.Data["status"] = status
		record.Data["changesJson"] = TextValue{}.Json(changes)
	}
	return records, nil
}

func (actions MailActions) recovered(caller RouteCaller, record FyloRecord) (string, []map[string]any, error) {
	rules := MailActionRules{}
	changes, err := rules.Changes(record.Data)
	if err == nil {
		err = rules.EnsureDomains(changes, caller.Domains)
	}
	if err != nil {
		return "", nil, err
	}
	target, source, next := "after", "before", "applied"
	if record.Data["status"] == "undoing" {
		target, source, next = "before", "after", "undone"
	}
	if err := actions.provider().Refreshed(caller, changes, target, record.Id); err != nil {
		return "", nil, err
	}
	messages, err := actions.authorized(caller, actions.ids(changes))
	if err != nil {
		return "", nil, err
	}
	accepted, err := actions.accepted(caller, messages, changes, target)
	if err == nil && !accepted {
		accepted, err = actions.accepted(caller, messages, changes, source)
	}
	if err == nil && !accepted {
		err = &MailActionError{Message: "Messages changed while an action was interrupted", Status: 409}
	}
	if err == nil {
		_, err = actions.written(caller, changes, target, record.Id)
	}
	return next, changes, err
}

func (actions MailActions) provider() MailActionProvider {
	return MailActionProvider{Provider: actions.Provider, Actions: actions.Actions}
}

func (MailActions) ids(changes []map[string]any) []string {
	ids := []string{}
	for _, change := range changes {
		ids = append(ids, TextValue{}.Of(change["id"]))
	}
	return ids
}

// authorized answers the current emails of the ids, refusing the whole set
// when one is missing or not the caller's.
func (actions MailActions) authorized(caller RouteCaller, ids []string) ([]map[string]any, error) {
	messages := []map[string]any{}
	for _, id := range ids {
		if (ExternalMessage{}).IsRoute(id) {
			message, err := actions.provider().Message(caller, id)
			if err != nil {
				return nil, err
			}
			messages = append(messages, message)
			continue
		}
		_, email, err := actions.owned(caller, id)
		if err != nil {
			return nil, err
		}
		messages = append(messages, email)
	}
	return messages, nil
}

func (actions MailActions) owned(caller RouteCaller, id string) (string, map[string]any, error) {
	documentId, email, err := actions.Emails.FindById(id)
	if err != nil {
		return "", nil, err
	}
	if documentId == "" || email == nil || !(TextValue{}).Includes(caller.Domains, email["domain"]) || !(EmailsRecords{}).CanAccessEmail(email, caller.Email) {
		return "", nil, &MailActionError{Message: "Email not found", Status: 404}
	}
	return documentId, email, nil
}

// built answers the change each message needs; messages already in the
// patched state are left out.
func (actions MailActions) built(caller RouteCaller, messages []map[string]any, patch map[string]any) ([]map[string]any, error) {
	rules := MailActionRules{}
	mappings, err := actions.Categories.ForOwner(caller.Email)
	if err != nil {
		return nil, err
	}
	changes := []map[string]any{}
	for _, email := range messages {
		external := TextValue{}.Truthy(email["external"])
		if external && patch["labels"] != nil {
			return nil, &MailActionError{Message: "Connected accounts do not expose portable label mutations", Status: 400}
		}
		before, after := map[string]any{}, map[string]any{}
		for _, field := range rules.MutableFields() {
			if value, given := patch[field]; given {
				before[field] = rules.FieldValue(email, field)
				after[field] = value
				if field == "labels" {
					after[field] = MailActionLabels{}.Applied(before[field], value)
				}
			}
		}
		if folder, isText := after["folder"].(string); isText && !external {
			if path := mappings.PathFor(TextValue{}.Of(email["domain"]), folder); path != "" {
				after["folder"] = path
			}
		}
		if rules.SameState(before, after) {
			continue
		}
		change := map[string]any{"id": email["id"], "domain": email["domain"], "before": before, "after": after}
		if external {
			change["source"] = "provider"
			change["accountId"] = email["providerAccountId"]
			change["stableId"] = email["providerStableId"]
			change["providerMutations"] = map[string]any{"before": []any{}, "after": []any{}}
			change["providerGenerations"] = map[string]any{"before": 0, "after": 0}
		}
		changes = append(changes, change)
	}
	return rules.Decoded(changes), nil
}

// written writes the target state of every change and answers the emails as
// they then read.
func (actions MailActions) written(caller RouteCaller, changes []map[string]any, target string, actionId string) ([]map[string]any, error) {
	authorized, err := actions.authorized(caller, actions.ids(changes))
	if err != nil {
		return nil, err
	}
	current := map[string]map[string]any{}
	for _, message := range authorized {
		current[TextValue{}.Of(message["id"])] = message
	}
	updated := []map[string]any{}
	for index, change := range changes {
		state, _ := change[target].(map[string]any)
		if change["source"] == "provider" {
			message, err := actions.provider().Written(caller, changes, index, target, actionId, current[TextValue{}.Of(change["id"])])
			if err != nil {
				return nil, err
			}
			updated = append(updated, message)
			continue
		}
		documentId, email, err := actions.owned(caller, TextValue{}.Of(change["id"]))
		if err != nil {
			return nil, err
		}
		if !(MailActionRules{}).MatchesState(email, state) {
			if err := actions.Emails.Update(documentId, state); err != nil {
				return nil, err
			}
		}
		merged := TextValue{}.Copy(email)
		for field, value := range state {
			merged[field] = value
		}
		updated = append(updated, merged)
	}
	return updated, nil
}

// rolledBack restores a prepared action's before states, and reports
// whether every message was restored or never changed.
func (actions MailActions) rolledBack(caller RouteCaller, changes []map[string]any, actionId string) bool {
	for _, change := range changes {
		if change["source"] == "provider" {
			_, err := actions.written(caller, changes, "before", actionId+"-rollback")
			return err == nil
		}
	}
	rules := MailActionRules{}
	for _, change := range changes {
		documentId, email, err := actions.owned(caller, TextValue{}.Of(change["id"]))
		if err != nil {
			return false
		}
		before, _ := change["before"].(map[string]any)
		after, _ := change["after"].(map[string]any)
		if rules.MatchesState(email, after) {
			if actions.Emails.Update(documentId, before) != nil {
				return false
			}
		} else if !rules.MatchesState(email, before) {
			return false
		}
	}
	return true
}

// accepted reports whether every message holds its change's target state;
// a provider change with queued mutations is judged by their states.
func (actions MailActions) accepted(caller RouteCaller, messages []map[string]any, changes []map[string]any, target string) (bool, error) {
	rules := MailActionRules{}
	for index, change := range changes {
		state, _ := change[target].(map[string]any)
		if index >= len(messages) {
			return false, nil
		}
		if change["source"] != "provider" {
			if !rules.MatchesState(messages[index], state) {
				return false, nil
			}
			continue
		}
		accepted, err := actions.provider().Accepted(caller, messages[index], change, target)
		if err != nil || !accepted {
			return false, err
		}
	}
	return true, nil
}
