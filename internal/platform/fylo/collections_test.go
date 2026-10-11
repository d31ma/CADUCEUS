package fylo

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

func readRepositoryFile(t *testing.T, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", relative))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func captured(pattern, text string) []string {
	var found []string
	for _, match := range regexp.MustCompile(pattern).FindAllStringSubmatch(text, -1) {
		found = append(found, match[1])
	}
	return found
}

// Every collection the Go server's routes name is created by the entry point
// and allowed by the bridge.
func TestServerCollectionsAreListed(t *testing.T) {
	named := captured(`(?m)^\s+[A-Z]\w*: "([a-z-]+)",$`, readRepositoryFile(t, "server/shared/persistence/FyloCollections.go"))
	resetSource := regexp.MustCompile(`(?s)func \(TestResetRoute\) collections\(\) \[\]string \{(.*?)\n\}`).FindStringSubmatch(readRepositoryFile(t, "server/shared/routes/TestResetRoute.go"))
	if resetSource == nil {
		t.Fatal("TestResetRoute.go has no collections()")
	}
	reset := captured(`"([a-z-]+)"`, resetSource[1])
	if len(reset) != 7 {
		t.Fatalf("TestResetRoute.collections() names %q; expected the seven retired authentication collections", reset)
	}
	if len(named) < 25 {
		t.Fatalf("read only %d names from FyloCollections.go", len(named))
	}
	all := AllCollections()
	for _, name := range append(named, reset...) {
		if !slices.Contains(all, name) {
			t.Errorf("the server uses collection %q, which internal/platform/fylo does not list", name)
		}
	}
}

// The legacy JavaScript container, which the suites seed records through,
// creates the same collections in the same order.
func TestLegacyContainerCreatesTheSameCollections(t *testing.T) {
	source := readRepositoryFile(t, "tests/support/legacy-server/shared/persistence/Fylo.js")
	block := func(name string) []string {
		body := regexp.MustCompile(`(?s)static ` + name + ` = \{(.*?)\}`).FindStringSubmatch(source)
		if body == nil {
			t.Fatalf("Fylo.js has no static %s", name)
		}
		return captured(`'([a-z-]+)'`, body[1])
	}
	if got := block("Collections"); !slices.Equal(got, DocumentCollections()) {
		t.Errorf("Fylo.Collections = %q\nDocumentCollections = %q", got, DocumentCollections())
	}
	if got := block("Buckets"); !slices.Equal(got, FileCollections()) {
		t.Errorf("Fylo.Buckets = %q\nFileCollections = %q", got, FileCollections())
	}
}
