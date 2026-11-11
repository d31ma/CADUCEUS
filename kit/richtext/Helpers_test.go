package richtext

import (
	"encoding/json"
	"testing"
)

func blockTexts(result Result) []string {
	texts := []string{}
	for _, block := range result.Blocks {
		texts = append(texts, block.plainText())
	}
	return texts
}

func blockKinds(result Result) []string {
	kinds := []string{}
	for _, block := range result.Blocks {
		kinds = append(kinds, block.Kind)
	}
	return kinds
}

func encoded(t *testing.T, value any) string {
	t.Helper()
	bytes, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(bytes)
}

func expectStrings(t *testing.T, name string, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %q, want %q", name, got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("%s: got %q, want %q", name, got, want)
		}
	}
}
