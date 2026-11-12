package nocloud

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type allowedUse struct {
	file   string
	text   string
	reason string
}

type finding struct {
	file string
	host string
}

func publicHosts() []string {
	return []string{"cloudflare-dns", "amazonaws.com", "8.8.8.8", "8.8.4.4", "1.1.1.1", "1.0.0.1", "dns.google", "dns.quad9.net", "9.9.9.9"}
}

func allowedUses() []allowedUse {
	return []allowedUse{
		{file: "scripts/floci/deploy.sh", text: "lambda.amazonaws.com", reason: "an IAM service principal name in the Floci dev role policy, not an address Mesail connects to"},
	}
}

func skippedFolders() []string {
	return []string{".git", ".tachyon", "dist", "node_modules", "testdata", "vendor"}
}

func checkedExtensions() []string {
	return []string{".go", ".json", ".sh", ".yml", ".yaml", ".toml", ".conf", ".env", ".example", ".html", ".js", ".css"}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func isChecked(name string) bool {
	if strings.HasSuffix(name, "_test.go") {
		return false
	}
	return name == "Dockerfile" || slices.Contains(checkedExtensions(), filepath.Ext(name))
}

func isAllowed(file string, line string) bool {
	return slices.ContainsFunc(allowedUses(), func(use allowedUse) bool {
		return use.file == file && strings.Contains(line, use.text)
	})
}

func lineFindings(file string, content string) []finding {
	found := []finding{}
	for _, line := range strings.Split(content, "\n") {
		found = append(found, hostsInLine(file, line)...)
	}
	return found
}

func hostsInLine(file string, line string) []finding {
	found := []finding{}
	for _, host := range publicHosts() {
		if strings.Contains(line, host) && !isAllowed(file, line) {
			found = append(found, finding{file: file, host: host})
		}
	}
	return found
}

func scan(t *testing.T, root string) []finding {
	t.Helper()
	found := []finding{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if entry.IsDir() && slices.Contains(skippedFolders(), entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() || !isChecked(entry.Name()) {
			return nil
		}
		content, err := os.ReadFile(path)
		relative, _ := filepath.Rel(root, path)
		found = append(found, lineFindings(filepath.ToSlash(relative), string(content))...)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func applicationSettings(t *testing.T, app string) map[string]any {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(repositoryRoot(t), "apps", app, "server", "application.json"))
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]any{}
	if err := json.Unmarshal(content, &settings); err != nil {
		t.Fatal(err)
	}
	return settings
}

func section(t *testing.T, settings map[string]any, name string) map[string]any {
	t.Helper()
	value, ok := settings[name].(map[string]any)
	if !ok {
		t.Fatalf("settings have no %q section", name)
	}
	return value
}

func outgoingHosts(t *testing.T, settings map[string]any) []string {
	t.Helper()
	hosts := []string{}
	for _, host := range section(t, settings, "outgoing")["hosts"].([]any) {
		hosts = append(hosts, host.(string))
	}
	return hosts
}

func TestNoCodeOrSettingNamesAPublicOrCloudHost(t *testing.T) {
	found := scan(t, repositoryRoot(t))
	if len(found) != 0 {
		t.Fatalf("public or cloud hosts in code or settings (ADR 0007): %+v", found)
	}
}

func TestTheScanFindsAPublicHost(t *testing.T) {
	folder := t.TempDir()
	os.WriteFile(filepath.Join(folder, "application.json"), []byte(`{"dnsresolver": "https://cloudflare-dns.com/dns-query"}`), 0o600)
	found := scan(t, folder)
	if !slices.Equal(found, []finding{{file: "application.json", host: "cloudflare-dns"}}) {
		t.Fatalf("got %+v", found)
	}
}

func TestTheScanSkipsTestsDocsAndVendoredCode(t *testing.T) {
	folder := t.TempDir()
	os.MkdirAll(filepath.Join(folder, "vendor"), 0o700)
	os.WriteFile(filepath.Join(folder, "vendor", "client.go"), []byte(`const host = "dns.google"`), 0o600)
	os.WriteFile(filepath.Join(folder, "Lookup_test.go"), []byte(`const host = "1.1.1.1"`), 0o600)
	os.WriteFile(filepath.Join(folder, "README.md"), []byte("not `8.8.8.8`"), 0o600)
	found := scan(t, folder)
	if len(found) != 0 {
		t.Fatalf("got %+v", found)
	}
}

func TestTheDirectoryUsesTheHostResolverByDefault(t *testing.T) {
	settings := applicationSettings(t, "directory")
	resolver := section(t, settings, "directory")["dnsresolver"]
	if resolver != "" {
		t.Fatalf("dnsresolver must default to the host's resolver (empty), got %q", resolver)
	}
}

func TestTheDirectoryAllowsNoPublicResolverHost(t *testing.T) {
	hosts := outgoingHosts(t, applicationSettings(t, "directory"))
	if slices.Contains(hosts, "cloudflare-dns.com") {
		t.Fatalf("outgoing hosts name a public resolver: %v", hosts)
	}
}
