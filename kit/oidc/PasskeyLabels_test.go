package oidc

import (
	"strings"
	"testing"
)

func TestPasskeyLabelsKeepsAFreeLabel(t *testing.T) {
	realmEqual(t, "label", PasskeyLabels{}.Unique(" Mac ", []PasskeyRecord{{Label: "Phone"}}), "Mac")
}

func TestPasskeyLabelsNumbersATakenLabel(t *testing.T) {
	existing := []PasskeyRecord{{Label: "Mac"}, {Label: "Mac 2"}}
	realmEqual(t, "label", PasskeyLabels{}.Unique("Mac", existing), "Mac 3")
}

func TestPasskeyLabelsCutsTheBaseToFit64Characters(t *testing.T) {
	base := strings.Repeat("a", 64)
	label := PasskeyLabels{}.Unique(base, []PasskeyRecord{{Label: base}})
	realmEqual(t, "label", label, strings.Repeat("a", 62)+" 2")
}

func TestPasskeyLabelsCountsCharactersNotBytes(t *testing.T) {
	base := strings.Repeat("é", 64)
	label := PasskeyLabels{}.Unique(base, []PasskeyRecord{{Label: base}})
	realmEqual(t, "label", label, strings.Repeat("é", 62)+" 2")
	realmIs(t, PasskeyLabels{}.Checked(base), nil)
}

func TestPasskeyLabelsChecksTheLength(t *testing.T) {
	realmIs(t, PasskeyLabels{}.Checked("  "), InvalidLabel{})
	realmIs(t, PasskeyLabels{}.Checked(strings.Repeat("a", 65)), InvalidLabel{})
	realmIs(t, PasskeyLabels{}.Checked("Mac"), nil)
}
