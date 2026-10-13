import (
	"errors"
	"time"
)

// TextUtilsTest covers the small pure utilities of the shell.
// @Test(ShellTestSetup, {"partition":"tests"})
type TextUtilsTest struct {
	behaviors *BehaviorContext
}

func (test *TextUtilsTest) TestFirstOfPrefersTheFirstText() {
	Assert.Equals("a", TextUtils{}.FirstOf("a", "b"))
	Assert.Equals("b", TextUtils{}.FirstOf("", "b"))
}

func (test *TextUtilsTest) TestDisplayNamePrefersTheStoredName() {
	Assert.Equals("Ada Lovelace", NameUtils{}.DisplayName("  Ada Lovelace ", "x@example.com"))
}

func (test *TextUtilsTest) TestDisplayNameTitleCasesTheLocalPart() {
	Assert.Equals("Jane Doe", NameUtils{}.DisplayName("", "jane.doe@example.com"))
	Assert.Equals("Admin", NameUtils{}.DisplayName("", "admin@caduceus.com"))
}

func (test *TextUtilsTest) TestRefusedReadsAnErrorStatus() {
	Assert.IsTrue(HTTPUtils{}.Refused(errors.New("HTTP_STATUS: The browser HTTP request failed.")))
	Assert.Equals(false, HTTPUtils{}.Refused(errors.New("HTTP_NETWORK: The browser HTTP request failed.")))
}

func (test *TextUtilsTest) TestMessageTellsARefusalFromANetworkFailure() {
	Assert.Equals("Unable to sign in.", HTTPUtils{}.Message(errors.New("HTTP_STATUS: x"), "Unable to sign in.", "Network error."))
	Assert.Equals("Network error.", HTTPUtils{}.Message(errors.New("HTTP_NETWORK: x"), "Unable to sign in.", "Network error."))
}

func (test *TextUtilsTest) TestMissedNeedsTheProofField() {
	Assert.IsTrue(HTTPUtils{}.Missed(nil, ""))
	Assert.Equals(false, HTTPUtils{}.Missed(nil, "act_1"))
}

func (test *TextUtilsTest) TestPasskeyFailuresName() {
	Assert.Equals("Passkey authentication was cancelled.", PasskeyUtils{}.Failure("cancelled", "get"))
	Assert.Equals("Passkey registration was cancelled.", PasskeyUtils{}.Failure("cancelled", "create"))
	Assert.Equals("Passkey error: boom", PasskeyUtils{}.Failure("error:boom", "get"))
}

func (test *TextUtilsTest) TestFlowPicksTheStep() {
	chosen := ""
	FlowUtils{}.When(true, func() { chosen = "when" })()
	FlowUtils{}.Either(false, func() { chosen = "first" }, func() { chosen += "-other" })()
	Assert.Equals("when-other", chosen)
}

func (test *TextUtilsTest) TestSizeLabels() {
	Assert.Equals("512 B", SizeUtils{}.Label(512))
	Assert.Equals("3.4 KB", SizeUtils{}.Label(3482))
	Assert.Equals("12 KB", SizeUtils{}.Label(12288))
	Assert.Equals("1.5 MB", SizeUtils{}.Label(1572864))
}

func (test *TextUtilsTest) TestDateLabel() {
	Assert.Equals("Jan 5, 2026, 2:30 PM", DateLabelUtils{}.Stamp("2026-01-05T14:30:00Z", time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC)))
	Assert.Equals("", DateLabelUtils{}.Stamp("not a date", time.Now()))
}
