package realtime

import "testing"

func TestMemoryStorageScrollsByEqualityInPages(t *testing.T) {
	memory := newMemoryStorage()
	storage := memory.storage()
	memory.CreateCollection("things")
	for _, domain := range []string{"a.com", "a.com", "b.com"} {
		memory.Insert("things", DomainRecord{DomainId: domain})
	}
	found := []DomainRecord{}
	cursor, err := storage.Page(Query{Collection: "things", Where: map[string]any{"domainId": "a.com"}, Size: 1}, &found)
	if err != nil || len(found) != 1 || cursor == "" {
		t.Fatalf("first page %v %q %v", found, cursor, err)
	}
	cursor, _ = storage.Page(Query{Collection: "things", Where: map[string]any{"domainId": "a.com"}, Cursor: cursor, Size: 1}, &found)
	if len(found) != 1 || cursor != "" {
		t.Fatalf("second page %v %q", found, cursor)
	}
}
