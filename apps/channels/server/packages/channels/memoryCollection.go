package channels

import (
	"fmt"
	"maps"
)

type memoryCollection struct {
	documents map[string]map[string]any
	order     []string
}

func (collection *memoryCollection) insert(fields map[string]any) string {
	id := fmt.Sprintf("%014d", len(collection.order)+1)
	stored := maps.Clone(fields)
	stored["id"] = id
	collection.documents[id] = stored
	collection.order = append(collection.order, id)
	return id
}

func (collection *memoryCollection) patch(id string, fields map[string]any) error {
	stored, found := collection.documents[id]
	if !found {
		return fmt.Errorf("document %s does not exist", id)
	}
	maps.Copy(stored, fields)
	stored["id"] = id
	return nil
}

func (collection *memoryCollection) matching(where map[string]any) []map[string]any {
	found := []map[string]any{}
	for _, id := range collection.order {
		found = append(found, collection.ifMatching(collection.documents[id], where)...)
	}
	return found
}

func (collection *memoryCollection) ifMatching(document map[string]any, where map[string]any) []map[string]any {
	for field, value := range where {
		if fmt.Sprint(document[field]) != fmt.Sprint(value) {
			return nil
		}
	}
	return []map[string]any{document}
}
