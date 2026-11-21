package devicebox

import (
	"fmt"
	"os"
	"testing"
)

type commandLog struct {
	inner Storage
	seen  map[string]int
}

func (log commandLog) Command(command map[string]any) (any, error) {
	log.seen[fmt.Sprint(command["command"], " ", command["collection"])]++
	return log.inner.Command(command)
}

func (log commandLog) Query(sql string, parameters []any) (any, error) {
	return log.inner.Query(sql, parameters)
}

func TestMain(m *testing.M) {
	folder, _ := os.MkdirTemp("", "devicebox-known-collections-")
	os.Setenv("TMPDIR", folder)
	code := m.Run()
	os.RemoveAll(folder)
	os.Exit(code)
}

func TestEachCollectionIsCheckedAndCreatedOnceForManyPuts(t *testing.T) {
	w := newWorld(t)
	log := commandLog{inner: w.storage, seen: map[string]int{}}
	w.wire(log)
	w.directory.Owns(w.domain, "ana", "a1")
	w.directory.Owns(w.domain, "bo", "b1")
	putMany(w, 5)
	_, pendingErr := w.desk.store.Waiting()
	names := Names{}
	collections := []string{names.Slips(w.domain), names.Bodies(w.domain), names.Counts(w.domain), names.Registry()}
	if pendingErr != nil || len(w.list("bo", "b1").Envelopes) != 5 {
		t.Fatalf("pending %v", pendingErr)
	}
	if got := callsOf(log, "collectionExists", collections); fmt.Sprint(got) != "[1 1 1 1]" {
		t.Fatalf("collectionExists calls %v", got)
	}
	if got := callsOf(log, "createCollection", collections); fmt.Sprint(got) != "[1 1 1 1]" {
		t.Fatalf("createCollection calls %v", got)
	}
}

func TestTheDomainListOfAnEmptyStoreIsEmptyAndCreatesNothing(t *testing.T) {
	w := newWorld(t)
	log := commandLog{inner: w.storage, seen: map[string]int{}}
	records, err := NewStore(NewDocuments(log)).Domains()
	if err != nil || len(records) != 0 || log.seen["createCollection "+Names{}.Registry()] != 0 {
		t.Fatalf("got %v %v %v", records, err, log.seen)
	}
}

func putMany(w *world, count int) {
	for range count {
		w.mustPut("ana", "a1", "b1")
	}
}

func callsOf(log commandLog, command string, collections []string) []int {
	counts := []int{}
	for _, collection := range collections {
		counts = append(counts, log.seen[command+" "+collection])
	}
	return counts
}
