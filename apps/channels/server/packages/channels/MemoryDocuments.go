package channels

import (
	"encoding/json"
	"errors"
	"fmt"
)

type MemoryDocuments struct {
	collections map[string]*memoryCollection
	failures    []error
}

func NewMemoryDocuments() *MemoryDocuments {
	return &MemoryDocuments{collections: map[string]*memoryCollection{}, failures: []error{}}
}

func (memory *MemoryDocuments) FailNext(err error) {
	memory.failures = append(memory.failures, err)
}

func (memory *MemoryDocuments) Ensure(collection string) error {
	err := memory.takeFailure()
	if err != nil {
		return err
	}
	memory.ensure(collection)
	return nil
}

func (memory *MemoryDocuments) Insert(collection string, document any) (string, error) {
	fields, err := memory.open(collection, document)
	if err != nil {
		return "", err
	}
	return memory.collections[collection].insert(fields), nil
}

func (memory *MemoryDocuments) Find(collection string, id string, target any) (bool, error) {
	_, err := memory.open(collection, map[string]any{})
	if err != nil {
		return false, err
	}
	fields, found := memory.collections[collection].documents[id]
	if !found {
		return false, nil
	}
	return true, memory.decode(fields, target)
}

func (memory *MemoryDocuments) Replace(collection string, id string, document any) error {
	fields, err := memory.open(collection, document)
	if err != nil {
		return err
	}
	return memory.collections[collection].patch(id, fields)
}

func (memory *MemoryDocuments) All(collection string, where map[string]any, target any) error {
	_, err := memory.open(collection, map[string]any{})
	if err != nil {
		return err
	}
	return memory.decode(memory.collections[collection].matching(where), target)
}

func (memory *MemoryDocuments) Collections() []string {
	names := []string{}
	for name := range memory.collections {
		names = append(names, name)
	}
	return names
}

func (memory *MemoryDocuments) open(collection string, document any) (map[string]any, error) {
	err := memory.takeFailure()
	if err != nil {
		return nil, err
	}
	if memory.collections[collection] == nil {
		return nil, fmt.Errorf("FYLO collection %s does not exist", collection)
	}
	return memory.fieldsOf(document)
}

func (memory *MemoryDocuments) ensure(collection string) {
	if memory.collections[collection] == nil {
		memory.collections[collection] = &memoryCollection{documents: map[string]map[string]any{}, order: []string{}}
	}
}

func (memory *MemoryDocuments) takeFailure() error {
	if len(memory.failures) == 0 {
		return nil
	}
	failure := memory.failures[0]
	memory.failures = memory.failures[1:]
	return failure
}

func (*MemoryDocuments) fieldsOf(document any) (map[string]any, error) {
	encoded, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	fields := map[string]any{}
	err = json.Unmarshal(encoded, &fields)
	return fields, err
}

func (*MemoryDocuments) decode(value any, target any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return errors.Join(errors.New("cannot encode documents"), err)
	}
	return json.Unmarshal(encoded, target)
}
