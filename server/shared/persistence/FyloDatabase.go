import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// PatchDoc merges fields into one document. The FYLO machine reads the
// document id as text and its new fields from newDoc, as fylo.mjs sends them.
func (database FyloDatabase) PatchDoc(collection string, id string, fields any) error {
	_, err := database.Op("patchDoc", map[string]any{"collection": collection, "id": id, "newDoc": fields})
	return err
}

// DelDoc deletes one document.
func (database FyloDatabase) DelDoc(collection string, id string) error {
	_, err := database.Op("delDoc", map[string]any{"collection": collection, "id": id})
	return err
}

// FindDocsPage answers one bounded page of a FYLO query snapshot and the
// cursor of the next page ("" after the last page).
func (database FyloDatabase) FindDocsPage(collection string, query any, limit int, cursor string) (FyloPage, error) {
	page := map[string]any{"limit": limit}
	if cursor != "" {
		page["cursor"] = cursor
	}
	raw, err := database.Op("findDocs", map[string]any{"collection": collection, "query": query, "page": page})
	if err != nil {
		return FyloPage{}, err
	}
	var answer struct {
		Items      json.RawMessage `json:"items"`
		NextCursor any             `json:"nextCursor"`
	}
	if err := json.Unmarshal(raw, &answer); err != nil {
		return FyloPage{}, err
	}
	documents, err := FyloDocuments{}.Ordered(answer.Items)
	next, _ := answer.NextCursor.(string)
	return FyloPage{Documents: documents, NextCursor: next}, err
}

// PutFile stores one staged file in a file collection and answers its id.
func (database FyloDatabase) PutFile(collection string, file FyloFile) (string, error) {
	if !filepath.IsAbs(file.Path) {
		return "", errors.New("Fylo bucket source path must be absolute")
	}
	fields := map[string]any{"collection": collection, "file": map[string]any{"path": file.Path, "key": file.Key}}
	if file.MaxBytes != nil {
		fields["fileOptions"] = map[string]any{"maxBytes": *file.MaxBytes}
	}
	if file.Meta != nil {
		fields["meta"] = file.Meta
	}
	return database.identifier(database.Op("putData", fields))
}

// GetFileBytes answers the content of one stored file, checked against the
// length and checksum FYLO reports. A file too large for one response frame
// is read from the bucket's own object, as fylo.mjs does.
func (database FyloDatabase) GetFileBytes(collection string, id string) ([]byte, error) {
	raw, err := database.Op("getFileData", map[string]any{"collection": collection, "id": id})
	if err != nil {
		if database.oversized(err) {
			return database.bucketObject(collection, id)
		}
		return nil, err
	}
	var answer struct {
		Encoding       string `json:"encoding"`
		Data           *string `json:"data"`
		ContentLength  int    `json:"contentLength"`
		ChecksumSHA256 string `json:"checksumSHA256"`
	}
	if json.Unmarshal(raw, &answer) != nil || answer.Encoding != "base64" || answer.Data == nil {
		return nil, errors.New("Invalid Fylo bucket response for " + collection + "/" + id)
	}
	content, err := base64.StdEncoding.DecodeString(*answer.Data)
	if err != nil || len(content) != answer.ContentLength {
		return nil, errors.New("Fylo bucket length mismatch")
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != answer.ChecksumSHA256 {
		return nil, errors.New("Fylo bucket checksum mismatch")
	}
	return content, nil
}

func (FyloDatabase) oversized(err error) bool {
	var failure *BoundaryError
	return errors.As(err, &failure) && (failure.Code == "EFRAME_RESPONSE_TOO_LARGE" || failure.Code == "EUNSUPPORTEDOP")
}

// bucketObject finds one bucket object by its file name under the bucket's
// docs tree. The shard layout is FYLO's own, so it is discovered, not built.
func (database FyloDatabase) bucketObject(collection string, id string) ([]byte, error) {
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`).MatchString(collection) {
		return nil, errors.New("Invalid Fylo bucket name")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(id) {
		return nil, errors.New("Invalid Fylo bucket file ID")
	}
	documents, err := database.GetDoc(collection, id)
	if err != nil {
		return nil, err
	}
	name := database.objectName(documents, id)
	if !strings.HasPrefix(name, id+".") || strings.ContainsAny(name, "/\\") {
		return nil, errors.New("Invalid Fylo bucket metadata for " + collection + "/" + id)
	}
	bucketRoot, _ := filepath.Abs(filepath.Join(database.Root, ".buckets", collection))
	target := database.findObject(filepath.Join(bucketRoot, "docs"), name)
	if target == "" {
		return nil, errors.New("Fylo bucket object " + collection + "/" + name + " is not present")
	}
	if relative, err := filepath.Rel(bucketRoot, target); err != nil || strings.HasPrefix(relative, "..") {
		return nil, errors.New("Fylo bucket path escapes its root")
	}
	return os.ReadFile(target)
}

func (FyloDatabase) objectName(documents []FyloDocument, id string) string {
	for _, document := range documents {
		if document.Id != id {
			continue
		}
		var manifest struct {
			Name any `json:"name"`
		}
		_ = json.Unmarshal(document.Data, &manifest)
		name, _ := manifest.Name.(string)
		return name
	}
	return ""
}

func (FyloDatabase) findObject(docsRoot string, name string) string {
	shards, err := os.ReadDir(docsRoot)
	if err != nil {
		return ""
	}
	for _, shard := range shards {
		if !shard.IsDir() {
			if shard.Name() == name {
				return filepath.Join(docsRoot, name)
			}
			continue
		}
		candidate := filepath.Join(docsRoot, shard.Name(), name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
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
