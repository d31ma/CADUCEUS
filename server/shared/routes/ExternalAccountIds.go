import (
	"encoding/json"
	"net/http"
)

// ExternalAccountIds lists the ids of the caller's connected accounts
// through the external account boundary. It is the one accounts.list call
// the preference route needs; the external accounts routes own the rest.
type ExternalAccountIds struct{}

// Of answers the account ids the token's caller has connected, or the answer
// that reports the boundary's failure.
func (ExternalAccountIds) Of(token string) ([]string, *http.Response) {
	accounts := ExternalAccounts{}.FromSettings()
	raw, err := accounts.Call("accounts.list", map[string]any{"token": token})
	if err != nil {
		return nil, accounts.Failure(err)
	}
	var listed any
	_ = json.Unmarshal(raw, &listed)
	entries, isList := RecordValues{}.List(listed)
	if !isList {
		object, _ := RecordValues{}.Object(listed)
		entries, _ = RecordValues{}.List(object["accounts"])
	}
	ids := []string{}
	for _, entry := range entries {
		account, _ := RecordValues{}.Object(entry)
		if id, isText := account["id"].(string); isText && id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
