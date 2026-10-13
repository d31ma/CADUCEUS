import (
	"encoding/json"
	"net/http"
	"regexp"
	"slices"
)

// ExternalAccountsRoute answers /api/v1/external-accounts and the provider
// catalog and discovery routes. Every route authenticates its caller and
// forwards one accounts operation, with the caller's bearer token, to the Go
// owner; the token is always written last, so a body token can never select
// another owner.
type ExternalAccountsRoute struct {
	Access *AccessHelper
}

// Providers answers {providers}: the provider catalog.
func (route ExternalAccountsRoute) Providers(request *http.Request) *http.Response {
	return route.signed(request, nil, func(token string, _ any) *http.Response {
		result, err := route.accounts().Call("accounts.providers", map[string]any{"token": token})
		return route.under("providers", result, err)
	})
}

// Discover answers the provider settings discovered for an address.
func (route ExternalAccountsRoute) Discover(request *http.Request) *http.Response {
	return route.withBody(request, func(token string, body any) *http.Response {
		address := BodyValues{}.TrimmedText(BodyValues{}.Field(body, "address"))
		if address == "" {
			return Answer{}.BadRequest("address is required")
		}
		return route.call("accounts.discover", map[string]any{"token": token, "address": address})
	})
}

// List answers {accounts}: the caller's external accounts.
func (route ExternalAccountsRoute) List(request *http.Request) *http.Response {
	return route.signed(request, nil, func(token string, _ any) *http.Response {
		result, err := route.accounts().Call("accounts.list", map[string]any{"token": token})
		return route.under("accounts", result, err)
	})
}

// Spread forwards the body's fields with the caller's token to an operation
// such as accounts.create, accounts.validate or an OAuth begin.
func (route ExternalAccountsRoute) Spread(request *http.Request, operation string) *http.Response {
	return route.withBody(request, func(token string, body any) *http.Response {
		parameters := BodyValues{}.Spread(RecordValues{}.Or(body, nil))
		parameters["token"] = token
		return route.call(operation, parameters)
	})
}

// CompleteOAuth forwards the state and code an OAuth provider returned.
func (route ExternalAccountsRoute) CompleteOAuth(request *http.Request, operation string) *http.Response {
	return route.withBody(request, func(token string, body any) *http.Response {
		return route.call(operation, map[string]any{
			"token": token,
			"state": RecordValues{}.Or(BodyValues{}.Field(body, "state"), ""),
			"code":  RecordValues{}.Or(BodyValues{}.Field(body, "code"), ""),
		})
	})
}

// Get answers one external account of the caller.
func (route ExternalAccountsRoute) Get(id string, request *http.Request) *http.Response {
	return route.signed(request, nil, func(token string, _ any) *http.Response {
		id := route.normalizedId(id)
		if id == "" {
			return Answer{}.NotFound("External account not found")
		}
		return route.call("accounts.get", map[string]any{"token": token, "id": id})
	})
}

// Change pauses, resumes or updates the presentation of one account under
// its current revision.
func (route ExternalAccountsRoute) Change(id string, request *http.Request) *http.Response {
	return route.withBody(request, func(token string, body any) *http.Response {
		id := route.normalizedId(id)
		revision := BodyValues{}.PositiveRevision(BodyValues{}.Field(body, "revision"))
		action, _ := BodyValues{}.Field(body, "action").(string)
		if id == "" {
			return Answer{}.NotFound("External account not found")
		}
		if revision == 0 || !slices.Contains([]string{"pause", "resume", "update"}, action) {
			return Answer{}.BadRequest("action and positive revision are required")
		}
		parameters := map[string]any{}
		operation := "accounts." + action
		if action == "update" {
			operation = "accounts.update_presentation"
			for _, name := range []string{"label", "color", "order"} {
				if (BodyValues{}).Has(body, name) {
					parameters[name] = BodyValues{}.Field(body, name)
				}
			}
		}
		parameters["token"], parameters["id"], parameters["revision"] = token, id, revision
		return route.call(operation, parameters)
	})
}

// Remove deletes one account under its current revision.
func (route ExternalAccountsRoute) Remove(id string, request *http.Request) *http.Response {
	return route.withBody(request, func(token string, body any) *http.Response {
		id := route.normalizedId(id)
		revision := BodyValues{}.PositiveRevision(BodyValues{}.Field(body, "revision"))
		if id == "" {
			return Answer{}.NotFound("External account not found")
		}
		if revision == 0 {
			return Answer{}.BadRequest("positive revision is required")
		}
		return route.call("accounts.delete", map[string]any{"token": token, "id": id, "revision": revision})
	})
}

// Read answers an operation that reads one account by id, such as
// accounts.diagnostics or accounts.folders.
func (route ExternalAccountsRoute) Read(id string, request *http.Request, operation string) *http.Response {
	return route.signed(request, nil, func(token string, _ any) *http.Response {
		return route.call(operation, map[string]any{"token": token, "id": id})
	})
}

// Messages answers the indexed messages of one folder of an account.
func (route ExternalAccountsRoute) Messages(id string, request *http.Request) *http.Response {
	return route.signed(request, nil, func(token string, _ any) *http.Response {
		query := request.URL.Query()
		folder := ""
		if values := query["folder"]; len(values) > 0 {
			folder = values[len(values)-1]
		}
		limitValues, present := query["limit"]
		limit := ""
		if present && len(limitValues) > 0 {
			limit = limitValues[len(limitValues)-1]
		}
		return route.call("accounts.messages", map[string]any{
			"token": token, "id": id, "folder": folder, "limit": BodyValues{}.Limit(limit, present),
		})
	})
}

// Mutate queues one provider mutation (a message flag, move or delete, or a
// folder change) under the account's revision and an idempotency key.
func (route ExternalAccountsRoute) Mutate(id string, request *http.Request) *http.Response {
	return route.withBody(request, func(token string, body any) *http.Response {
		values := BodyValues{}
		revision := values.CoercedRevision(values.Field(body, "revision"))
		kind := values.Text(values.Field(body, "kind"))
		idempotencyKey := values.Trimmed(values.Field(body, "idempotencyKey"))
		stableIds, isList := values.Field(body, "messageStableIds").([]any)
		if !isList {
			stableIds = []any{}
		}
		messageKinds := []string{"set_read", "set_starred", "move", "delete"}
		folderKinds := []string{"folder_create", "folder_rename", "folder_delete"}
		known := slices.Contains(messageKinds, kind) || slices.Contains(folderKinds, kind)
		if revision == 0 || idempotencyKey == "" || !known || (slices.Contains(messageKinds, kind) && len(stableIds) == 0) {
			return Answer{}.BadRequest("revision, idempotencyKey, kind, and the required mutation target are required")
		}
		return route.call("accounts.mutate", map[string]any{
			"revision": revision, "idempotencyKey": idempotencyKey, "kind": kind, "messageStableIds": stableIds,
			"targetFolderId": values.Text(values.Field(body, "targetFolderId")),
			"value":          values.Field(body, "value") == true,
			"folderStableId": values.Text(values.Field(body, "folderStableId")),
			"parentStableId": values.Text(values.Field(body, "parentStableId")),
			"folderName":     values.Text(values.Field(body, "folderName")),
			"token":          token, "id": id,
		})
	})
}

// Mutation answers one queued provider mutation.
func (route ExternalAccountsRoute) Mutation(id string, mutationId string, request *http.Request) *http.Response {
	return route.signed(request, nil, func(token string, _ any) *http.Response {
		if id == "" || mutationId == "" {
			return Answer{}.NotFound("Provider mutation not found")
		}
		return route.call("accounts.mutation", map[string]any{"token": token, "id": id, "mutationId": mutationId})
	})
}

// Reconnect replaces an account's credentials under its current revision.
func (route ExternalAccountsRoute) Reconnect(id string, request *http.Request) *http.Response {
	return route.withBody(request, func(token string, body any) *http.Response {
		if !route.accountId().MatchString(id) {
			id = ""
		}
		revision := BodyValues{}.PositiveRevision(BodyValues{}.Field(body, "revision"))
		credentials, isObject := BodyValues{}.Field(body, "credentials").(map[string]any)
		if id == "" {
			return Answer{}.NotFound("External account not found")
		}
		if revision == 0 || !isObject {
			return Answer{}.BadRequest("credentials and positive revision are required")
		}
		return route.call("accounts.reconnect", map[string]any{"token": token, "id": id, "revision": revision, "credentials": credentials})
	})
}

// Retry schedules another synchronization of an account.
func (route ExternalAccountsRoute) Retry(id string, request *http.Request) *http.Response {
	return route.withBody(request, func(token string, body any) *http.Response {
		revision := BodyValues{}.CoercedRevision(BodyValues{}.Field(body, "revision"))
		if revision == 0 {
			return Answer{}.BadRequest("positive revision is required")
		}
		return route.call("accounts.retry", map[string]any{"token": token, "id": id, "revision": revision})
	})
}

// withBody reads the JSON body (malformed JSON is refused before the caller
// is authenticated) and runs answer for the signed-in caller.
func (route ExternalAccountsRoute) withBody(request *http.Request, answer func(token string, body any) *http.Response) *http.Response {
	body, refusal := RequestBody{}.Value(request)
	if refusal != nil {
		return refusal
	}
	return route.signed(request, body, answer)
}

func (route ExternalAccountsRoute) signed(request *http.Request, body any, answer func(token string, body any) *http.Response) *http.Response {
	caller, refusal := route.Access.Signed(request)
	if refusal != nil {
		return refusal
	}
	return answer(caller.Token, body)
}

func (route ExternalAccountsRoute) call(operation string, parameters map[string]any) *http.Response {
	result, err := route.accounts().Call(operation, parameters)
	if err != nil {
		return route.accounts().Failure(err)
	}
	return ResultAnswer{}.Of(result)
}

func (route ExternalAccountsRoute) under(field string, result json.RawMessage, err error) *http.Response {
	if err != nil {
		return route.accounts().Failure(err)
	}
	return ResultAnswer{}.Under(field, result)
}

func (ExternalAccountsRoute) accounts() ExternalAccounts {
	return ExternalAccounts{}.FromSettings()
}

func (route ExternalAccountsRoute) normalizedId(value string) string {
	id := BodyValues{}.TrimmedText(value)
	if !route.accountId().MatchString(id) {
		return ""
	}
	return id
}

func (ExternalAccountsRoute) accountId() *regexp.Regexp {
	return regexp.MustCompile(`^ext_[A-Za-z0-9_-]{20,80}$`)
}
