package notify

import (
	"strings"
	"testing"
)

func TestSettingsAcceptZeroDelay(t *testing.T) {
	expectNoError(t, SettingsCheck{}.Check(0, nil))
}

func TestSettingsAcceptShortestDelay(t *testing.T) {
	expectNoError(t, SettingsCheck{}.Check(15, []string{"c1"}))
}

func TestSettingsAcceptLongestDelay(t *testing.T) {
	expectNoError(t, SettingsCheck{}.Check(1440, []string{strings.Repeat("a", 64)}))
}

func TestSettingsRefuseDelayTooShort(t *testing.T) {
	expectInvalidSettings(t, SettingsCheck{}.Check(14, nil), "afterMinutes")
}

func TestSettingsRefuseDelayTooLong(t *testing.T) {
	expectInvalidSettings(t, SettingsCheck{}.Check(1441, nil), "afterMinutes")
}

func TestSettingsRefuseNegativeDelay(t *testing.T) {
	expectInvalidSettings(t, SettingsCheck{}.Check(-1, nil), "afterMinutes")
}

func TestSettingsAcceptFiveHundredMuted(t *testing.T) {
	expectNoError(t, SettingsCheck{}.Check(0, strings.Split(strings.Repeat("c,", 500), ",")[:500]))
}

func TestSettingsRefuseTooManyMuted(t *testing.T) {
	ids := strings.Split(strings.Repeat("c,", 501), ",")[:501]
	expectInvalidSettings(t, SettingsCheck{}.Check(0, ids), "mutedConversationIds")
}

func TestSettingsRefuseEmptyMutedId(t *testing.T) {
	expectInvalidSettings(t, SettingsCheck{}.Check(0, []string{"c1", ""}), "mutedConversationIds")
}

func TestSettingsRefuseLongMutedId(t *testing.T) {
	expectInvalidSettings(t, SettingsCheck{}.Check(0, []string{strings.Repeat("a", 65)}), "mutedConversationIds")
}

func TestInvalidSettingsMessage(t *testing.T) {
	expectSame(t, InvalidSettings{Field: "afterMinutes", Reason: "is wrong"}.Error(), "the notification settings are not valid: afterMinutes is wrong")
}
