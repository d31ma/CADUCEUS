package mailout

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"mesail.dev/packages/oidc"
)

type memoryDocuments struct {
	lock        sync.Mutex
	collections map[string][]map[string]any
	next        int
	failures    map[string]error
}

func newMemoryDocuments() *memoryDocuments {
	return &memoryDocuments{collections: map[string][]map[string]any{}, failures: map[string]error{}}
}

func (documents *memoryDocuments) fail(method string, err error) {
	documents.lock.Lock()
	defer documents.lock.Unlock()
	documents.failures[method] = err
}

func (documents *memoryDocuments) CreateCollection(name string) error {
	documents.lock.Lock()
	defer documents.lock.Unlock()
	if err := documents.failures["CreateCollection"]; err != nil {
		return err
	}
	if _, found := documents.collections[name]; !found {
		documents.collections[name] = []map[string]any{}
	}
	return nil
}

func (documents *memoryDocuments) Insert(collection string, document any) (string, error) {
	documents.lock.Lock()
	defer documents.lock.Unlock()
	if err := documents.failures["Insert"]; err != nil {
		return "", err
	}
	fields := toMap(document)
	documents.next++
	id := fmt.Sprintf("doc%06d", documents.next)
	fields["id"] = id
	documents.collections[collection] = append(documents.collections[collection], fields)
	return id, nil
}

func (documents *memoryDocuments) FindById(collection string, id string, target any) (bool, error) {
	documents.lock.Lock()
	defer documents.lock.Unlock()
	if err := documents.failures["FindById"]; err != nil {
		return false, err
	}
	for _, fields := range documents.collections[collection] {
		if fields["id"] == id {
			return true, fromMap(fields, target)
		}
	}
	return false, nil
}

func (documents *memoryDocuments) Patch(collection string, id string, change any, target any) (bool, error) {
	documents.lock.Lock()
	defer documents.lock.Unlock()
	if err := documents.failures["Patch"]; err != nil {
		return false, err
	}
	for _, fields := range documents.collections[collection] {
		if fields["id"] == id {
			for key, value := range toMap(change) {
				if key != "id" {
					fields[key] = value
				}
			}
			return true, fromMap(fields, target)
		}
	}
	return false, nil
}

func (documents *memoryDocuments) Command(command map[string]any) (any, error) {
	documents.lock.Lock()
	defer documents.lock.Unlock()
	if err := documents.failures["Command"]; err != nil {
		return nil, err
	}
	collection, _ := command["collection"].(string)
	where, _ := command["where"].(map[string]any)
	size, _ := command["size"].(int)
	cursor, _ := command["cursor"].(string)
	start, _ := strconv.Atoi(cursor)
	matches := []map[string]any{}
	for _, fields := range documents.collections[collection] {
		if matchesWhere(fields, where) {
			matches = append(matches, toMap(fields))
		}
	}
	end := min(len(matches), start+size)
	next := ""
	if end < len(matches) {
		next = strconv.Itoa(end)
	}
	return map[string]any{"content": matches[start:end], "nextCursor": next}, nil
}

func (documents *memoryDocuments) count(collection string) int {
	documents.lock.Lock()
	defer documents.lock.Unlock()
	return len(documents.collections[collection])
}

func matchesWhere(fields map[string]any, where map[string]any) bool {
	for key, value := range where {
		wanted, _ := json.Marshal(value)
		actual, _ := json.Marshal(fields[key])
		if string(wanted) != string(actual) {
			return false
		}
	}
	return true
}

func toMap(value any) map[string]any {
	raw, _ := json.Marshal(value)
	fields := map[string]any{}
	_ = json.Unmarshal(raw, &fields)
	return fields
}

func fromMap(fields map[string]any, target any) error {
	if target == nil {
		return nil
	}
	raw, _ := json.Marshal(fields)
	return json.Unmarshal(raw, target)
}

type tokenGate struct{}

func (tokenGate) Identify(authorization string) (oidc.Caller, error) {
	switch {
	case strings.HasPrefix(authorization, "Bearer token-"):
		return oidc.Caller{DomainId: strings.TrimPrefix(authorization, "Bearer token-"), Roles: []string{"Service"}}, nil
	case strings.HasPrefix(authorization, "Bearer member-"):
		return oidc.Caller{DomainId: strings.TrimPrefix(authorization, "Bearer member-"), Roles: []string{"Member"}}, nil
	default:
		return oidc.Caller{}, errors.New("no token")
	}
}

func tokenRealm(authorization string) string {
	return strings.TrimPrefix(authorization, "Bearer token-")
}

func fixedTokens(realm string) (string, error) {
	return "Bearer token-" + realm, nil
}
