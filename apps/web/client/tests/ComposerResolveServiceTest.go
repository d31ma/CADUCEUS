// @Test(ComposerTestSetup, {"partition":"tests"})
type ComposerResolveServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ComposerResolveServiceTest) resolve() *ComposerResolveService {
	return ComposerTestSetup{}.Resolve(test.behaviors)
}

func (test *ComposerResolveServiceTest) TestResolveAsksTheDirectoryWithTheToken() {
	resolved, failure := test.resolve().Resolve([]string{"bo@example.com", "eve@example.org"})
	Assert.Equals(nil, failure)
	Assert.Equals("member", resolved[0].Kind)
	Assert.Equals("otherDomain", resolved[1].Kind)
}

func (test *ComposerResolveServiceTest) TestRecipientOfUsesTheAddressWhenThereIsNoName() {
	recipient := test.resolve().RecipientOf(ResolvedRecipient{Address: "carol@partner.org", Kind: "external", DefaultDelivery: "email", AllowedDeliveries: []string{"email"}}, "cc", "chat")
	Assert.Equals("carol@partner.org", recipient.DisplayName)
	Assert.Equals("email", recipient.Delivery)
	Assert.Equals("cc", recipient.Role)
}

func (test *ComposerResolveServiceTest) TestCheckingAndInvalidChips() {
	resolve := test.resolve()
	Assert.Equals("Checking…", resolve.Checking("x@y.z", "to").Label)
	Assert.Equals("Not valid", resolve.Invalid("x@y", "to").Label)
}

func (test *ComposerResolveServiceTest) TestLabelledWritesTheChipText() {
	recipient := test.resolve().Labelled(Recipient{DisplayName: "Bo Lee", Address: "bo@example.com", Kind: "member", Delivery: "both", State: "ready"})
	Assert.Equals("Chat + email", recipient.Label)
	Assert.Equals("Bo Lee, Chat + email, bo@example.com. Open menu", recipient.AriaLabel)
}
