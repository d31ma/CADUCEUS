// @Test(ComposerTestSetup, {"partition":"tests"})
type ComposerRecipientServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ComposerRecipientServiceTest) recipients() *ComposerRecipientService {
	return ComposerTestSetup{}.Recipients(test.behaviors)
}

func (test *ComposerRecipientServiceTest) TestAddResolvesAMemberAsChat() {
	recipients := test.recipients()
	recipients.Add("bo@example.com", "to")
	Assert.Equals(1, len(recipients.Recipients))
	Assert.Equals("member", recipients.Recipients[0].Kind)
	Assert.Equals("chat", recipients.Recipients[0].Delivery)
	Assert.Equals("Chat", recipients.Recipients[0].Label)
	Assert.Equals("Bo Lee", recipients.Recipients[0].DisplayName)
}

func (test *ComposerRecipientServiceTest) TestAddResolvesAChannelAsChatOnly() {
	recipients := test.recipients()
	recipients.Add("releases@example.com", "to")
	recipients.Choose("releases@example.com", "to", "email")
	Assert.Equals("chat", recipients.Recipients[0].Delivery)
}

func (test *ComposerRecipientServiceTest) TestAddMarksAnotherHostedDomainAsEmailOnly() {
	recipients := test.recipients()
	recipients.Add("eve@example.org", "to")
	eve := recipients.Recipients[0]
	Assert.Equals("otherDomain", eve.Kind)
	Assert.Equals("Email · other domain", eve.Label)
	Assert.IsTrue(!eve.ChatAllowed)
	Assert.Equals("Chat stays inside example.com", eve.ChatReason)
}

func (test *ComposerRecipientServiceTest) TestAddGivesTheReasonForAnOutsideAddress() {
	recipients := test.recipients()
	recipients.Add("carol@partner.org", "to")
	Assert.Equals("Email", recipients.Recipients[0].Label)
	Assert.Equals("carol@partner.org has no Mesail account", recipients.Recipients[0].ChatReason)
}

func (test *ComposerRecipientServiceTest) TestAddTakesSeveralAddressesAtOnce() {
	recipients := test.recipients()
	recipients.Add("bo@example.com, carol@partner.org", "to")
	Assert.Equals(2, len(recipients.Recipients))
}

func (test *ComposerRecipientServiceTest) TestAddIgnoresADuplicate() {
	recipients := test.recipients()
	recipients.Add("bo@example.com", "to")
	recipients.Add("BO@example.com", "to")
	Assert.Equals(1, len(recipients.Recipients))
}

func (test *ComposerRecipientServiceTest) TestAddMarksAnIncompleteAddressAsNotValid() {
	recipients := test.recipients()
	recipients.Add("carol@partner", "to")
	Assert.Equals("invalid", recipients.Recipients[0].State)
	Assert.Equals("Not valid", recipients.Recipients[0].Label)
	Assert.Equals(1, recipients.CountInState("invalid"))
}

func (test *ComposerRecipientServiceTest) TestChooseSwitchesAMemberToEmail() {
	recipients := test.recipients()
	recipients.Add("bo@example.com", "to")
	recipients.Choose("bo@example.com", "to", "email")
	Assert.Equals("email", recipients.Recipients[0].Delivery)
	Assert.Equals("Email", recipients.Recipients[0].Label)
}

func (test *ComposerRecipientServiceTest) TestChooseOffersBothToAMember() {
	recipients := test.recipients()
	recipients.Add("bo@example.com", "to")
	recipients.Choose("bo@example.com", "to", "both")
	Assert.Equals("Chat + email", recipients.Recipients[0].Label)
}

func (test *ComposerRecipientServiceTest) TestChooseRefusesChatForAnotherDomain() {
	recipients := test.recipients()
	recipients.Add("eve@example.org", "to")
	recipients.Choose("eve@example.org", "to", "chat")
	recipients.Choose("eve@example.org", "to", "both")
	Assert.Equals("email", recipients.Recipients[0].Delivery)
}

func (test *ComposerRecipientServiceTest) TestChooseRefusesChatForAnOutsideAddress() {
	recipients := test.recipients()
	recipients.Add("carol@partner.org", "to")
	recipients.Choose("carol@partner.org", "to", "chat")
	Assert.Equals("email", recipients.Recipients[0].Delivery)
}

func (test *ComposerRecipientServiceTest) TestRemoveTakesTheChipAway() {
	recipients := test.recipients()
	recipients.Add("bo@example.com carol@partner.org", "to")
	recipients.Remove("bo@example.com", "to")
	Assert.Equals(1, len(recipients.Recipients))
	Assert.Equals("carol@partner.org", recipients.Recipients[0].Address)
}

func (test *ComposerRecipientServiceTest) TestRemoveLastTakesTheLastChipOfTheRow() {
	recipients := test.recipients()
	recipients.Add("bo@example.com", "to")
	recipients.Add("carol@partner.org", "cc")
	recipients.RemoveLast("to")
	Assert.Equals(1, len(recipients.Recipients))
	Assert.Equals("cc", recipients.Recipients[0].Role)
}

func (test *ComposerRecipientServiceTest) TestRemoveLastOfAnEmptyRowChangesNothing() {
	recipients := test.recipients()
	recipients.Add("bo@example.com", "to")
	recipients.RemoveLast("bcc")
	Assert.Equals(1, len(recipients.Recipients))
}



func (test *ComposerRecipientServiceTest) TestRefreshReportsAKindThatChanged() {
	recipients := test.recipients()
	recipients.Add("eve@example.org bo@example.com", "to")
	recipients.Recipients[0] = recipients.resolve.RecipientOf(ResolvedRecipient{Address: "eve@example.org", DisplayName: "Eve", Kind: "member", DefaultDelivery: "chat", AllowedDeliveries: []string{"chat", "email", "both"}}, "to", "chat")
	changes := recipients.Refresh()
	Assert.Equals(1, len(changes))
	Assert.Equals("Chat", changes[0].Before)
	Assert.Equals("Email · other domain", changes[0].After)
	Assert.Equals("email", recipients.Recipients[0].Delivery)
}

func (test *ComposerRecipientServiceTest) TestRefreshKeepsAChoiceThatIsStillAllowed() {
	recipients := test.recipients()
	recipients.Add("bo@example.com", "to")
	recipients.Choose("bo@example.com", "to", "both")
	Assert.Equals(0, len(recipients.Refresh()))
	Assert.Equals("both", recipients.Recipients[0].Delivery)
}

func (test *ComposerRecipientServiceTest) TestInRoleListsOneRow() {
	recipients := test.recipients()
	recipients.Add("bo@example.com", "to")
	recipients.Add("carol@partner.org", "bcc")
	Assert.Equals(1, len(recipients.InRole("bcc")))
	Assert.Equals("carol@partner.org", recipients.InRole("bcc")[0].Address)
}

func (test *ComposerRecipientServiceTest) TestChoicesCarryTheKindTheUserSaw() {
	recipients := test.recipients()
	recipients.Add("bo@example.com", "to")
	recipients.Add("eve@example.org", "cc")
	Assert.Equals([]RecipientChoice{
		{Address: "bo@example.com", Role: "to", Delivery: "chat", ExpectedKind: "member"},
		{Address: "eve@example.org", Role: "cc", Delivery: "email", ExpectedKind: "otherDomain"},
	}, recipients.Choices())
}

func (test *ComposerRecipientServiceTest) TestClearEmptiesTheComposer() {
	recipients := test.recipients()
	recipients.Add("bo@example.com", "to")
	recipients.Clear()
	Assert.Equals(0, len(recipients.Recipients))
}

func (test *ComposerRecipientServiceTest) TestSummaryAndLabelsFollowTheRecipients() {
	recipients := test.recipients()
	recipients.Add("bo@example.com carol@partner.org", "to")
	Assert.Equals([]string{"Bo Lee gets a chat message.", "carol@partner.org gets an email."}, recipients.Summary())
	Assert.Equals("Send · 1 chat, 1 email", recipients.SendLabel())
	Assert.IsTrue(recipients.EmailMode())
	Assert.Equals("This is a chat and an email. Chat stays inside example.com, so carol@partner.org gets an email.", recipients.ModeText())
}

func (test *ComposerRecipientServiceTest) TestResolvePendingWithNothingPendingAsksNobody() {
	recipients := test.recipients()
	recipients.ResolvePending()
	Assert.Equals("", recipients.Problem)
}

func (test *ComposerRecipientServiceTest) TestFindReturnsTheChip() {
	recipients := test.recipients()
	recipients.Add("bo@example.com", "to")
	Assert.Equals("Bo Lee", recipients.Find("bo@example.com").DisplayName)
	Assert.Equals("", recipients.Find("nobody@example.com").Address)
}

func (test *ComposerRecipientServiceTest) TestSenderAddressIsTheSignedInAddress() {
	Assert.Equals("ana@example.com", test.recipients().SenderAddress())
}

