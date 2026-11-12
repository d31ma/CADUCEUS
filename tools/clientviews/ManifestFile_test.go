package main

import (
	"strings"
	"testing"
)

func TestAManifestWithoutTheSettingIsFine(t *testing.T) {
	problems, err := ManifestFile{Name: "manifest.json", Source: []byte(`{"name": "Mesail", "tac": {"styles": []}}`)}.Problems()
	if err != nil || len(problems) != 0 {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestAManifestThatKeepsTheCheckOnIsFine(t *testing.T) {
	problems, err := ManifestFile{Name: "manifest.json", Source: []byte(`{"tac": {"dependencies": {"check": true}}}`)}.Problems()
	if err != nil || len(problems) != 0 {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestAManifestThatTurnsTheCheckOffIsAProblem(t *testing.T) {
	source := "{\n  \"tac\": {\n    \"dependencies\": {\"check\": false}\n  }\n}\n"
	problems, err := ManifestFile{Name: "manifest.json", Source: []byte(source)}.Problems()
	if err != nil || len(problems) != 1 || problems[0].Line != 3 || !strings.Contains(problems[0].Reason, "tac.dependencies.check") {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestAManifestThatIsNotJsonIsAnError(t *testing.T) {
	_, err := ManifestFile{Name: "manifest.json", Source: []byte(`{"tac":`)}.Problems()
	if err == nil || !strings.HasPrefix(err.Error(), "manifest.json: ") {
		t.Fatalf("got %v", err)
	}
}

func TestAManifestWithACheckThatIsNotABooleanIsAnError(t *testing.T) {
	_, err := ManifestFile{Name: "manifest.json", Source: []byte(`{"tac": {"dependencies": {"check": "no"}}}`)}.Problems()
	if err == nil {
		t.Fatal("want an error")
	}
}
