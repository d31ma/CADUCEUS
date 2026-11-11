package channels

import (
	"errors"
	"fmt"
	"strconv"
	"testing"

	"mesail.dev/packages/outbox"
)

type memoryCommander struct {
	memory  *MemoryDocuments
	failure error
	answer  any
}

func newMemoryCommander() *memoryCommander {
	return &memoryCommander{memory: NewMemoryDocuments()}
}

func (commander *memoryCommander) Command(command map[string]any) (any, error) {
	if commander.failure != nil {
		return nil, commander.failure
	}
	if commander.answer != nil {
		return commander.answer, nil
	}
	collection := fmt.Sprint(command["collection"])
	switch command["command"] {
	case "collectionExists":
		return commander.memory.collections[collection] != nil, nil
	case "createCollection":
		commander.memory.ensure(collection)
		return true, nil
	case "insert":
		id, err := commander.memory.Insert(collection, command["value"])
		return map[string]any{"id": id}, err
	case "findById":
		document := map[string]any{}
		found, err := commander.memory.Find(collection, fmt.Sprint(command["id"]), &document)
		if !found {
			return nil, err
		}
		return document, err
	case "patch":
		return nil, commander.memory.Replace(collection, fmt.Sprint(command["id"]), command["value"])
	case "scroll":
		return commander.scroll(collection, command)
	}
	return nil, fmt.Errorf("unknown command %v", command["command"])
}

func (commander *memoryCommander) scroll(collection string, command map[string]any) (any, error) {
	rows := []map[string]any{}
	err := commander.memory.All(collection, command["where"].(map[string]any), &rows)
	start, _ := strconv.Atoi(fmt.Sprint(command["cursor"]))
	end := min(start+command["size"].(int), len(rows))
	next := ""
	if end < len(rows) {
		next = strconv.Itoa(end)
	}
	return map[string]any{"content": rows[start:end], "nextCursor": next}, err
}

func TestCommandDocumentsCreateACollectionOnlyOnce(t *testing.T) {
	commander := newMemoryCommander()
	documents := NewCommandDocuments(commander)
	if err := documents.Ensure("channels-x"); err != nil {
		t.Fatal(err)
	}
	id, _ := documents.Insert("channels-x", ChannelDocument{Name: "releases"})
	if err := documents.Ensure("channels-x"); err != nil {
		t.Fatal(err)
	}
	found, err := documents.Find("channels-x", id, &ChannelDocument{})
	if !found || err != nil {
		t.Fatalf("the second Ensure lost the document: %v %v", found, err)
	}
}

func TestCommandDocumentsInsertFindAndReplace(t *testing.T) {
	documents := NewCommandDocuments(newMemoryCommander())
	documents.Ensure("channels-x")
	id, err := documents.Insert("channels-x", ChannelDocument{Name: "releases", Topic: "old"})
	if err != nil || id == "" {
		t.Fatalf("insert failed: %q %v", id, err)
	}
	if err := documents.Replace("channels-x", id, map[string]any{"topic": "new"}); err != nil {
		t.Fatal(err)
	}
	document := ChannelDocument{}
	found, err := documents.Find("channels-x", id, &document)
	if !found || err != nil || document.Topic != "new" || document.Name != "releases" {
		t.Fatalf("unexpected document %+v %v %v", document, found, err)
	}
}

func TestCommandDocumentsFindOfAMissingDocumentIsNotFound(t *testing.T) {
	documents := NewCommandDocuments(newMemoryCommander())
	documents.Ensure("channels-x")
	found, err := documents.Find("channels-x", "00000000000099", &ChannelDocument{})
	if found || err != nil {
		t.Fatalf("want not found without an error, got %v %v", found, err)
	}
}

func TestCommandDocumentsAllReadsEveryPage(t *testing.T) {
	documents := NewCommandDocuments(newMemoryCommander())
	documents.Ensure("channels-x")
	for index := 0; index < 230; index++ {
		documents.Insert("channels-x", ChannelDocument{Name: "c" + strconv.Itoa(index), Visibility: "public"})
	}
	documents.Insert("channels-x", ChannelDocument{Name: "hidden", Visibility: "private"})
	public := []ChannelDocument{}
	if err := documents.All("channels-x", map[string]any{"visibility": "public"}, &public); err != nil {
		t.Fatal(err)
	}
	if len(public) != 230 || public[229].Name != "c229" {
		t.Fatalf("want 230 public channels in order, got %d", len(public))
	}
}

func TestCommandDocumentsReportAFailingStoreAsUnavailable(t *testing.T) {
	commander := newMemoryCommander()
	commander.failure = errors.New("fylo stopped")
	documents := NewCommandDocuments(commander)
	_, insertError := documents.Insert("channels-x", ChannelDocument{})
	_, findError := documents.Find("channels-x", "1", &ChannelDocument{})
	failures := []error{documents.Ensure("channels-x"), insertError, findError, documents.Replace("channels-x", "1", ChannelDocument{}), documents.All("channels-x", map[string]any{}, &[]ChannelDocument{})}
	for index, failure := range failures {
		if !errors.As(failure, new(*Unavailable)) {
			t.Fatalf("operation %d: want Unavailable, got %v", index, failure)
		}
	}
}

func TestCommandDocumentsReportAnUnreadableAnswer(t *testing.T) {
	commander := newMemoryCommander()
	commander.answer = "not a document"
	documents := NewCommandDocuments(commander)
	_, insertError := documents.Insert("channels-x", ChannelDocument{})
	if !errors.As(insertError, new(*Unavailable)) {
		t.Fatalf("want Unavailable, got %v", insertError)
	}
	commander.answer = map[string]any{"bad": make(chan int)}
	_, findError := documents.Find("channels-x", "1", &ChannelDocument{})
	if !errors.As(findError, new(*Unavailable)) {
		t.Fatalf("want Unavailable, got %v", findError)
	}
}

func TestCommandDocumentsRefuseADocumentThatIsNotAnObject(t *testing.T) {
	documents := NewCommandDocuments(newMemoryCommander())
	documents.Ensure("channels-x")
	if _, err := documents.Insert("channels-x", "text"); err == nil {
		t.Fatal("want an error for a document that is not an object")
	}
	if err := documents.Replace("channels-x", "1", "text"); err == nil {
		t.Fatal("want an error for fields that are not an object")
	}
}

func TestAssemblyBuildsAWorkingWorkflowAndDesk(t *testing.T) {
	t.Setenv("YON_APPLICATION_JSON", `{"yon":{"outbox":{"producer":"channels","subscribers":[{"name":"search","url":"http://127.0.0.1:1/s"}]},"channels":{"directoryurl":"http://127.0.0.1:1"}}}`)
	assembly := Assembly{Source: newMemoryCommander()}
	_, err := assembly.Workflow().List("Bearer forged")
	if !errors.As(err, new(*Unauthenticated)) {
		t.Fatalf("want Unauthenticated, got %v", err)
	}
	waiting, err := assembly.Desk().Waiting()
	if waiting != 0 || err != nil {
		t.Fatalf("want an empty outbox, got %d %v", waiting, err)
	}
}

func TestAssemblyUsesTheGivenPusherAndDirectory(t *testing.T) {
	directory := NewMemoryDirectory()
	assembly := Assembly{Source: newMemoryCommander(), Directory: directory, Pusher: nil}
	if assembly.directory(Settings{}) != Directory(directory) {
		t.Fatal("the given directory was not used")
	}
	if _, isRealm := assembly.pusher().(RealmPusher); !isRealm {
		t.Fatal("without a pusher the realm pusher is used")
	}
	recording := outbox.NewRecordingPusher()
	if (Assembly{Pusher: recording}).pusher() != outbox.Pusher(recording) {
		t.Fatal("the given pusher was not used")
	}
	if _, isClient := (Assembly{}).directory(Settings{DirectoryUrl: "http://127.0.0.1:1"}).(DirectoryClient); !isClient {
		t.Fatal("without a directory the HTTP client is used")
	}
}
