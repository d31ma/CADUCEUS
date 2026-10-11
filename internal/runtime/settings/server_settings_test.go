package settings

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// repositoryRoot answers the checkout this package sits in.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

var (
	settingReads = []*regexp.Regexp{
		regexp.MustCompile(`\.Value\("([A-Z][A-Z0-9_]+)"\)`),
		regexp.MustCompile(`\.setting\("([A-Z][A-Z0-9_]+)"`),
		regexp.MustCompile(`\.(?:Body|Refusal)\(request, "([A-Z][A-Z0-9_]+)"\)`),
	}
	settingLists = regexp.MustCompile(`range \[\]string\{([^}]*)\} \{\s*if [^\n]*\n\s*[a-z]+ = settings\.Value\(name\)`)
	quoted       = regexp.MustCompile(`"([A-Z][A-Z0-9_]+)"`)
)

// reservedByTachyon are names the server reads that `.env` cannot carry:
// Tachyon refuses YON_PORT (TY2010) because it owns the endpoint, so a
// handler reads "" and falls back to its default.
var reservedByTachyon = map[string]bool{"PORT": true}

// The Go server's handlers read settings by name through Settings{}.Value,
// which answers "" for a name the entry point did not hand over. Every name
// they read must be in HandlerSettings.
func TestServerReadsOnlyHandedOverSettings(t *testing.T) {
	server := filepath.Join(repositoryRoot(t), "server")
	read := map[string]string{}
	err := filepath.WalkDir(server, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "tests" {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		for _, pattern := range settingReads {
			for _, match := range pattern.FindAllStringSubmatch(text, -1) {
				read[match[1]] = path
			}
		}
		for _, list := range settingLists.FindAllStringSubmatch(text, -1) {
			for _, match := range quoted.FindAllStringSubmatch(list[1], -1) {
				read[match[1]] = path
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(read) < 20 {
		t.Fatalf("found only %d settings reads under %s; the patterns no longer match the server", len(read), server)
	}
	for name, path := range read {
		if reservedByTachyon[name] {
			continue
		}
		if !slices.Contains(HandlerSettings, name) {
			t.Errorf("%s reads %s, which is not in HandlerSettings", path, name)
		}
	}
}

// tests/support/TestServer.mjs hands the suites' settings to `ty serve` the
// way the entry point does, so its list must be HandlerSettings.
func TestTestServerHandsOverTheSameSettings(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(repositoryRoot(t), "tests", "support", "TestServer.mjs"))
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)static SETTINGS = \[(.*?)\]`).FindStringSubmatch(string(data))
	if block == nil {
		t.Fatal("TestServer.mjs has no static SETTINGS list")
	}
	var listed []string
	for _, match := range regexp.MustCompile(`'([A-Z][A-Z0-9_]+)'`).FindAllStringSubmatch(block[1], -1) {
		listed = append(listed, match[1])
	}
	if !slices.Equal(listed, HandlerSettings) {
		t.Fatalf("TestServer.SETTINGS = %q\nHandlerSettings   = %q", listed, HandlerSettings)
	}
	marker := regexp.MustCompile(`static MARKER = '([^']*)'`).FindStringSubmatch(string(data))
	if marker == nil || marker[1] != GeneratedMarker {
		t.Fatalf("TestServer.MARKER = %q, want %q", marker, GeneratedMarker)
	}
}

// `ty serve` refuses a project .env that sets its own endpoint or profile
// (TY2010), so the handed-over settings must never include those names.
func TestHandlerSettingsLeaveTachyonNamesAlone(t *testing.T) {
	for _, name := range HandlerSettings {
		if reservedByTachyon[name] || name == "HOST" || name == "PROFILE" || strings.HasPrefix(name, "SERVER_") {
			t.Errorf("HandlerSettings hands over %s, which Tachyon reserves", name)
		}
	}
}
