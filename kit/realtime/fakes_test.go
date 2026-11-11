package realtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"
)

type memoryStorage struct {
	mutex       sync.Mutex
	collections map[string]map[string]map[string]any
	nextId      int
	writes      int
	failAll     bool
}

func newMemoryStorage() *memoryStorage {
	return &memoryStorage{collections: map[string]map[string]map[string]any{}}
}

func (memory *memoryStorage) storage() Storage {
	return Storage{Documents: memory, Scroll: memory.scroll}
}

func (memory *memoryStorage) failure() error {
	if memory.failAll {
		return errors.New("storage is down")
	}
	return nil
}

func (memory *memoryStorage) CollectionExists(name string) (bool, error) {
	memory.mutex.Lock()
	defer memory.mutex.Unlock()
	_, found := memory.collections[name]
	return found, memory.failure()
}

func (memory *memoryStorage) CreateCollection(name string) error {
	memory.mutex.Lock()
	defer memory.mutex.Unlock()
	if _, found := memory.collections[name]; !found {
		memory.collections[name] = map[string]map[string]any{}
	}
	return memory.failure()
}

func toFields(document any) map[string]any {
	raw, _ := json.Marshal(document)
	fields := map[string]any{}
	json.Unmarshal(raw, &fields)
	return fields
}

func (memory *memoryStorage) Insert(collection string, document any) (string, error) {
	memory.mutex.Lock()
	defer memory.mutex.Unlock()
	if err := memory.failure(); err != nil {
		return "", err
	}
	documents, found := memory.collections[collection]
	if !found {
		return "", fmt.Errorf("collection %s does not exist", collection)
	}
	memory.nextId++
	memory.writes++
	id := "doc" + strconv.Itoa(memory.nextId)
	fields := toFields(document)
	fields["id"] = id
	documents[id] = fields
	return id, nil
}

func (memory *memoryStorage) FindById(collection string, id string, target any) (bool, error) {
	memory.mutex.Lock()
	defer memory.mutex.Unlock()
	fields, found := memory.collections[collection][id]
	if !found {
		return false, memory.failure()
	}
	raw, _ := json.Marshal(fields)
	return true, errors.Join(memory.failure(), json.Unmarshal(raw, target))
}

func (memory *memoryStorage) Patch(collection string, id string, changes any, target any) (bool, error) {
	memory.mutex.Lock()
	defer memory.mutex.Unlock()
	fields, found := memory.collections[collection][id]
	if !found {
		return false, memory.failure()
	}
	memory.writes++
	for key, value := range toFields(changes) {
		fields[key] = value
	}
	return true, memory.failure()
}

func (memory *memoryStorage) DeleteById(collection string, id string) (bool, error) {
	memory.mutex.Lock()
	defer memory.mutex.Unlock()
	_, found := memory.collections[collection][id]
	delete(memory.collections[collection], id)
	if found {
		memory.writes++
	}
	return found, memory.failure()
}

func (memory *memoryStorage) scroll(collection string, where map[string]any, cursor string, size int) (Window, error) {
	memory.mutex.Lock()
	defer memory.mutex.Unlock()
	if err := memory.failure(); err != nil {
		return Window{}, err
	}
	documents, found := memory.collections[collection]
	if !found {
		return Window{}, fmt.Errorf("collection %s does not exist", collection)
	}
	if len(where) > 8 || size < 1 || size > 100 {
		return Window{}, errors.New("scroll accepts at most 8 equality fields and a size of 1 to 100")
	}
	ids := []string{}
	for id, fields := range documents {
		if matches(fields, where) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	start, _ := strconv.Atoi(cursor)
	end := min(start+size, len(ids))
	page := []map[string]any{}
	for _, id := range ids[min(start, len(ids)):end] {
		page = append(page, documents[id])
	}
	next := ""
	if end < len(ids) {
		next = strconv.Itoa(end)
	}
	raw, _ := json.Marshal(page)
	return Window{Content: raw, NextCursor: next}, nil
}

func matches(fields map[string]any, where map[string]any) bool {
	for key, wanted := range where {
		if fmt.Sprint(fields[key]) != fmt.Sprint(wanted) {
			return false
		}
	}
	return true
}

type recordingGateway struct {
	mutex  sync.Mutex
	pushes map[string][]Event
	closes map[string]int
	gone   map[string]bool
	down   bool
}

func newRecordingGateway() *recordingGateway {
	return &recordingGateway{pushes: map[string][]Event{}, closes: map[string]int{}, gone: map[string]bool{}}
}

func (gateway *recordingGateway) Push(connectionId string, event Event) error {
	gateway.mutex.Lock()
	defer gateway.mutex.Unlock()
	if gateway.down {
		return Unavailable{Operation: "push"}
	}
	if gateway.gone[connectionId] {
		return Gone{ConnectionId: connectionId}
	}
	gateway.pushes[connectionId] = append(gateway.pushes[connectionId], event)
	return nil
}

func (gateway *recordingGateway) Close(connectionId string, code int) error {
	gateway.mutex.Lock()
	defer gateway.mutex.Unlock()
	if gateway.gone[connectionId] {
		return Gone{ConnectionId: connectionId}
	}
	gateway.closes[connectionId] = code
	return nil
}

func (gateway *recordingGateway) eventsOf(connectionId string) []Event {
	gateway.mutex.Lock()
	defer gateway.mutex.Unlock()
	return append([]Event{}, gateway.pushes[connectionId]...)
}

type fixedTokens struct {
	grants map[string]Grant
}

func (tokens fixedTokens) Grant(token string, now time.Time) (Grant, error) {
	grant, found := tokens.grants[token]
	if !found {
		return Grant{}, Unauthenticated{}
	}
	return grant, nil
}

func fixedClock(at time.Time) func() time.Time {
	return func() time.Time { return at }
}
