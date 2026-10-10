import (
	"errors"
	"time"
)

// MailActionProvider carries the connected-account side of mail actions:
// every provider message is reauthorized by the caller's token, every
// mutation is journaled before it is awaited, and a move's destination
// identity replaces the message's identity in the journal.
type MailActionProvider struct {
	Provider UnifiedMail
	Actions  *MailActionsHelper
}

func (MailActionProvider) mutationTimeout() time.Duration { return 60 * time.Second }

func (MailActionProvider) mutationPoll() time.Duration { return 100 * time.Millisecond }

func (MailActionProvider) unavailable() error {
	return &MailActionError{Message: "Connected account authorization is unavailable", Status: 503}
}

func (MailActionProvider) rejected() error {
	return &MailActionError{Message: "The connected account rejected this action. Refresh to review the provider version.", Status: 409}
}

// Journal merges fields into the caller's entry of an action.
func (provider MailActionProvider) Journal(caller RouteCaller, id string, patch map[string]any) error {
	found, err := provider.Actions.UpdateById(id, patch, caller.Email)
	if err != nil {
		return err
	}
	if !found {
		return &MailActionError{Message: "Mail action journal became unavailable", Status: 503}
	}
	return nil
}

// Message answers one provider message's summary, reauthorized by the
// caller's token.
func (provider MailActionProvider) Message(caller RouteCaller, id string) (map[string]any, error) {
	if caller.Token == "" {
		return nil, provider.unavailable()
	}
	loaded, err := provider.Provider.Load(caller.Token, id, false)
	if err != nil {
		return nil, err
	}
	var message map[string]any
	if loaded != nil {
		message = ExternalMessage{}.Summary(loaded.Indexed)
	}
	if message == nil {
		return nil, &MailActionError{Message: "Email not found", Status: 404}
	}
	return message, nil
}

// Written queues the provider mutations a change's target state needs and
// answers the message as it will read.
func (provider MailActionProvider) Written(caller RouteCaller, changes []map[string]any, index int, target string, actionId string, current map[string]any) (map[string]any, error) {
	change := changes[index]
	text := TextValue{}
	if caller.Token == "" {
		return nil, provider.unavailable()
	}
	loaded, err := provider.Provider.Load(caller.Token, text.Of(change["id"]), false)
	if err != nil {
		return nil, err
	}
	if loaded == nil || loaded.AccountId != change["accountId"] || loaded.StableId != change["stableId"] {
		return nil, &MailActionError{Message: "Email not found", Status: 404}
	}
	opposite := "before"
	if target == "before" {
		opposite = "after"
	}
	mutations, _ := change["providerMutations"].(map[string]any)
	state, _ := change[target].(map[string]any)
	if len(text.List(mutations[opposite])) == 0 && current != nil && (MailActionRules{}).MatchesState(current, state) {
		return current, nil
	}
	if loaded, err = provider.queued(caller, changes, index, target, actionId, loaded); err != nil {
		return nil, err
	}
	message := ExternalMessage{}.Summary(provider.Provider.Projected(loaded, state).Indexed)
	if message == nil {
		return nil, &MailActionError{Message: "Provider action projection failed", Status: 503}
	}
	return message, nil
}

// queued journals the target state's new mutations and, for a move, waits
// for its destination identity; it answers the message as it now loads.
func (provider MailActionProvider) queued(caller RouteCaller, changes []map[string]any, index int, target string, actionId string, loaded *ExternalLoaded) (*ExternalLoaded, error) {
	change := changes[index]
	text := TextValue{}
	state, _ := change[target].(map[string]any)
	generations, _ := change["providerGenerations"].(map[string]any)
	generation := int(text.Number(generations[target])) + 1
	prefix := "mail-action-" + actionId + "-" + target + "-" + text.Of(float64(generation)) + "-" + text.Of(float64(index))
	mutations, err := provider.Provider.QueuePatch(caller.Token, loaded, state, prefix)
	var folderError *ExternalFolderError
	if errors.As(err, &folderError) {
		return nil, &MailActionError{Message: folderError.Error(), Status: 400}
	}
	if err != nil {
		return nil, err
	}
	ids := []any{}
	var move map[string]any
	for _, mutation := range mutations {
		ids = append(ids, mutation["id"])
		if move == nil && mutation["kind"] == "move" {
			move = mutation
		}
	}
	provider.recorded(change, target, ids, generation)
	if err := provider.Journal(caller, actionId, map[string]any{"changesJson": text.Json(changes), "updatedAt": (EpochTime{}).Now()}); err != nil {
		return nil, err
	}
	if move == nil {
		return loaded, nil
	}
	return provider.moved(caller, changes, change, actionId, move)
}

func (MailActionProvider) recorded(change map[string]any, target string, ids []any, generation int) {
	mutations, isObject := change["providerMutations"].(map[string]any)
	if !isObject {
		mutations = map[string]any{"before": []any{}, "after": []any{}}
		change["providerMutations"] = mutations
	}
	generations, isObject := change["providerGenerations"].(map[string]any)
	if !isObject {
		generations = map[string]any{"before": float64(0), "after": float64(0)}
		change["providerGenerations"] = generations
	}
	mutations[target] = ids
	generations[target] = float64(generation)
}

func (provider MailActionProvider) moved(caller RouteCaller, changes []map[string]any, change map[string]any, actionId string, move map[string]any) (*ExternalLoaded, error) {
	text := TextValue{}
	completed, err := provider.awaited(caller, text.Of(change["accountId"]), text.Of(move["id"]))
	if err != nil {
		return nil, err
	}
	replacement := text.Of(text.Or(provider.first(completed["resultStableIds"]), ""))
	id := ExternalMessage{}.Id(change["accountId"], replacement)
	if id == "" {
		return nil, &MailActionError{Message: "The connected account did not return a safe destination identity", Status: 503}
	}
	change["stableId"], change["id"] = replacement, id
	if err := provider.Journal(caller, actionId, map[string]any{"changesJson": text.Json(changes), "updatedAt": (EpochTime{}).Now()}); err != nil {
		return nil, err
	}
	loaded, err := provider.Provider.Load(caller.Token, id, false)
	if err == nil && loaded == nil {
		err = &MailActionError{Message: "The moved provider message is not yet available", Status: 503}
	}
	return loaded, err
}

func (MailActionProvider) first(value any) any {
	if list := (TextValue{}).List(value); len(list) > 0 {
		return list[0]
	}
	return nil
}

// awaited polls one provider mutation until it applies, for at most a
// minute.
func (provider MailActionProvider) awaited(caller RouteCaller, accountId string, mutationId string) (map[string]any, error) {
	deadline := time.Now().Add(provider.mutationTimeout())
	for time.Now().Before(deadline) {
		mutation, err := provider.Provider.Accounts.Mutation(caller.Token, accountId, mutationId)
		if err != nil {
			return nil, err
		}
		switch mutation["state"] {
		case "applied":
			return mutation, nil
		case "failed":
			return nil, provider.rejected()
		}
		time.Sleep(provider.mutationPoll())
	}
	return nil, &MailActionError{Message: "The connected account action is still reconciling. Try again shortly.", Status: 503}
}

// Accepted reports whether a provider message holds its change's target
// state, judging queued mutations by their provider states.
func (provider MailActionProvider) Accepted(caller RouteCaller, message map[string]any, change map[string]any, target string) (bool, error) {
	if caller.Token == "" {
		return false, provider.unavailable()
	}
	state, _ := change[target].(map[string]any)
	mutations, _ := change["providerMutations"].(map[string]any)
	ids := TextValue{}.List(mutations[target])
	if len(ids) == 0 {
		return (MailActionRules{}).MatchesState(message, state), nil
	}
	applied := true
	for _, id := range ids {
		mutation, err := provider.Provider.Accounts.Mutation(caller.Token, TextValue{}.Of(change["accountId"]), TextValue{}.Of(id))
		if err != nil {
			return false, err
		}
		if mutation["state"] == "failed" {
			return false, provider.rejected()
		}
		applied = applied && mutation["state"] == "applied"
	}
	if applied && !(MailActionRules{}).MatchesState(message, state) {
		return false, &MailActionError{Message: "The connected account changed after this action. Refresh before choosing another result.", Status: 409}
	}
	return true, nil
}

// Refreshed replaces the identity of each provider message an applied move
// of the target state relocated, and journals the new identities.
func (provider MailActionProvider) Refreshed(caller RouteCaller, changes []map[string]any, target string, actionId string) error {
	if caller.Token == "" {
		return nil
	}
	changed := false
	for _, change := range changes {
		if change["source"] != "provider" {
			continue
		}
		mutations, _ := change["providerMutations"].(map[string]any)
		for _, mutationId := range (TextValue{}).List(mutations[target]) {
			mutation, err := provider.Provider.Accounts.Mutation(caller.Token, TextValue{}.Of(change["accountId"]), TextValue{}.Of(mutationId))
			if err != nil {
				return err
			}
			if mutation["kind"] != "move" || mutation["state"] != "applied" {
				continue
			}
			replacement := TextValue{}.Of(TextValue{}.Or(provider.first(mutation["resultStableIds"]), ""))
			id := ExternalMessage{}.Id(change["accountId"], replacement)
			if id == "" {
				return &MailActionError{Message: "The connected account move journal is missing its destination identity", Status: 503}
			}
			if change["stableId"] != replacement || change["id"] != id {
				change["stableId"], change["id"], changed = replacement, id, true
			}
		}
	}
	if !changed {
		return nil
	}
	return provider.Journal(caller, actionId, map[string]any{"changesJson": TextValue{}.Json(changes), "updatedAt": EpochTime{}.Now()})
}
