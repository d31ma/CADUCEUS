import (
	"encoding/json"
	"time"
)

// FyloDatabase runs FYLO operations through the Go owner of the data root.
// Access, when set, is the POSIX identity every operation runs as.
type FyloDatabase struct {
	Boundary Boundary
	Root     string
	Access   *FyloAccess
}

// As answers the same database acting with another access descriptor.
func (database FyloDatabase) As(access *FyloAccess) FyloDatabase {
	database.Access = access
	return database
}

// Op runs one operation and answers its raw result.
func (database FyloDatabase) Op(name string, fields map[string]any) (json.RawMessage, error) {
	payload := map[string]any{"op": name}
	for key, value := range fields {
		if value != nil {
			payload[key] = value
		}
	}
	if database.Access != nil {
		payload["access"] = database.Access
	}
	return database.Boundary.Call(payload, database.responseLimit(), database.timeout())
}

func (FyloDatabase) responseLimit() int { return 8 * 1024 * 1024 }

func (FyloDatabase) timeout() time.Duration { return 10 * time.Second }

// PutData stores one document and answers its id.
func (database FyloDatabase) PutData(collection string, data any) (string, error) {
	raw, err := database.Op("putData", map[string]any{"collection": collection, "data": data})
	return database.identifier(raw, err)
}

// GetDoc answers the document with its id (none when it is missing).
func (database FyloDatabase) GetDoc(collection string, id string) ([]FyloDocument, error) {
	return database.documents(database.Op("getDoc", map[string]any{"collection": collection, "id": id}))
}

// FindDocs answers every document that matches a FYLO query, in FYLO's order.
func (database FyloDatabase) FindDocs(collection string, query any) ([]FyloDocument, error) {
	return database.documents(database.Op("findDocs", map[string]any{"collection": collection, "query": query}))
}

// PatchDoc merges fields into one document. FYLO reads the document id and
// its new fields from one {id: fields} object, as the JavaScript client sent.
func (database FyloDatabase) PatchDoc(collection string, id string, fields any) error {
	_, err := database.Op("patchDoc", map[string]any{"collection": collection, "id": map[string]any{id: fields}})
	return err
}

// DelDoc deletes one document.
func (database FyloDatabase) DelDoc(collection string, id string) error {
	_, err := database.Op("delDoc", map[string]any{"collection": collection, "id": id})
	return err
}

func (FyloDatabase) identifier(raw json.RawMessage, err error) (string, error) {
	if err != nil {
		return "", err
	}
	var id string
	return id, json.Unmarshal(raw, &id)
}

func (FyloDatabase) documents(raw json.RawMessage, err error) ([]FyloDocument, error) {
	if err != nil {
		return nil, err
	}
	return FyloDocuments{}.Ordered(raw)
}
