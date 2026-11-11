package messaging

import (
	"errors"
	"regexp"
	"testing"
)

func TestEveryListedNameIsAccepted(t *testing.T) {
	for _, name := range (ReactionNames{}).All() {
		if err := (ReactionNames{}).Check(name); err != nil {
			t.Fatalf("%s refused: %v", name, err)
		}
	}
}

func TestTheListHasTwelvePathSafeNames(t *testing.T) {
	names := ReactionNames{}.All()
	pattern := regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
	if len(names) != 12 {
		t.Fatalf("want 12 names, got %d", len(names))
	}
	for _, name := range names {
		if !pattern.MatchString(name) {
			t.Fatalf("%q is not a path-safe name", name)
		}
	}
}

func TestAnUnknownNameIsRefused(t *testing.T) {
	err := ReactionNames{}.Check("👍")
	var unknown UnknownReaction
	if !errors.As(err, &unknown) || unknown.Name != "👍" || unknown.Error() != "unknown reaction name: 👍" {
		t.Fatalf("got %v", err)
	}
}
