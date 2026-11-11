package compose_test

import (
	"fmt"
	"strings"
	"testing"

	"mesail.dev/packages/compose"
)

func TestReplyReferencesAppendsTheParent(t *testing.T) {
	got := compose.Threading{}.ReplyReferences([]string{"root@example.com"}, "parent@example.com")
	if strings.Join(got, " ") != "root@example.com parent@example.com" {
		t.Fatalf("references = %v", got)
	}
}

func TestReplyReferencesRemovesBracketsAndDuplicates(t *testing.T) {
	got := compose.Threading{}.ReplyReferences([]string{"<root@example.com>", "root@example.com", "<parent@example.com>"}, "<parent@example.com>")
	if strings.Join(got, " ") != "root@example.com parent@example.com" {
		t.Fatalf("references = %v", got)
	}
}

func TestReplyReferencesSkipsAnEmptyParent(t *testing.T) {
	got := compose.Threading{}.ReplyReferences(nil, "")
	if len(got) != 0 {
		t.Fatalf("references = %v", got)
	}
}

func TestReplyReferencesKeepsTheFirstAndTheLatest(t *testing.T) {
	parents := []string{}
	for index := 0; index < 25; index++ {
		parents = append(parents, fmt.Sprintf("r%02d@example.com", index))
	}
	got := compose.Threading{}.ReplyReferences(parents, "parent@example.com")
	if len(got) != 20 || got[0] != "r00@example.com" || got[1] != "r07@example.com" || got[19] != "parent@example.com" {
		t.Fatalf("references = %v", got)
	}
}
