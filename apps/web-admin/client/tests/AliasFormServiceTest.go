import "strings"

// @Test(MemberDirectoryTestSetup, {"partition":"tests"})
type AliasFormServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
	// @Fake
	identityClient *IdentityClient
}

func (test *AliasFormServiceTest) opened() *AliasFormService {
	form := &AliasFormService{session: test.session(), problems: &AdminProblemService{}}
	test.construct(form)
	form.Opened(AdminViewOpened{View: "members", DomainId: "example.com"})
	return form
}

func (test *AliasFormServiceTest) added(address string, memberId string) *AliasFormService {
	form := test.opened()
	form.AddWith(address, memberId)
	return form
}

func (test *AliasFormServiceTest) TestOpenedOffersTheMembersAsChoices() {
	form := test.opened()
	Assert.Equals("example.com", form.DomainId)
	Assert.Equals(2, len(form.Choices))
	Assert.Equals("ana-id", form.Choices[0].MemberId)
	Assert.Equals("Ana Ruiz (ana@example.com)", form.Choices[0].Label)
	Assert.Equals("", form.Problem)
}

func (test *AliasFormServiceTest) TestAddCreatesTheAlias() {
	form := test.added("sales", "bo-id")
	Assert.Equals("", form.Problem)
	Assert.Equals("sales@example.com now reaches Bo Lee.", form.Status)
	Assert.Equals(false, form.Busy)
}

func (test *AliasFormServiceTest) TestAddListsTheAliasAsAddedJustNow() {
	form := test.added("sales", "bo-id")
	Assert.Equals(true, form.HasAdded)
	Assert.Equals(1, len(form.Added))
	Assert.Equals("sales@example.com", form.Added[0].Address)
	Assert.Equals("Bo Lee", form.Added[0].MemberName)
}

func (test *AliasFormServiceTest) TestAddClearsTheAddressAfterSuccess() {
	form := test.added("sales", "bo-id")
	Assert.Equals("", form.Address)
}

func (test *AliasFormServiceTest) TestAddAcceptsAFullAddressInTheDomain() {
	form := test.added(" Sales@Example.com ", "ana-id")
	Assert.Equals("sales@example.com now reaches Ana Ruiz.", form.Status)
}

func (test *AliasFormServiceTest) TestAddAcceptsDigitsDotsHyphensUnderscoresAndPlus() {
	form := test.added("a.b-c_d+1", "ana-id")
	Assert.Equals("", form.Problem)
	Assert.Equals("a.b-c_d+1@example.com", form.Added[0].Address)
}

func (test *AliasFormServiceTest) TestAddAcceptsTheLongestLocalPart() {
	form := test.added(strings.Repeat("a", 64), "ana-id")
	Assert.Equals("", form.Problem)
}

func (test *AliasFormServiceTest) TestAddRefusesALocalPartThatIsTooLong() {
	form := test.added(strings.Repeat("a", 65), "ana-id")
	Assert.Equals("invalidAddress", form.Problem)
	Assert.Equals(false, form.HasAdded)
}

func (test *AliasFormServiceTest) TestAddRefusesAnEmptyAddress() {
	form := test.added("", "ana-id")
	Assert.Equals("invalidAddress", form.Problem)
}

func (test *AliasFormServiceTest) TestAddRefusesALeadingDot() {
	form := test.added(".sales", "ana-id")
	Assert.Equals("invalidAddress", form.Problem)
}

func (test *AliasFormServiceTest) TestAddRefusesATrailingDot() {
	form := test.added("sales.", "ana-id")
	Assert.Equals("invalidAddress", form.Problem)
}

func (test *AliasFormServiceTest) TestAddRefusesOtherCharacters() {
	form := test.added("sales team", "ana-id")
	Assert.Equals("invalidAddress", form.Problem)
}

func (test *AliasFormServiceTest) TestAddRefusesAnAddressInAnotherDomain() {
	form := test.added("sales@other.org", "ana-id")
	Assert.Equals("invalidAddress", form.Problem)
	Assert.Equals("", form.Status)
}

func (test *AliasFormServiceTest) TestAddAsksForAMember() {
	form := test.added("sales", "")
	Assert.Equals("noMember", form.Problem)
	Assert.Equals(false, form.HasAdded)
}

func (test *AliasFormServiceTest) TestAddShowsAnAddressProblemBeforeAMemberProblem() {
	form := test.added("sales.", "")
	Assert.Equals("invalidAddress", form.Problem)
}

func (test *AliasFormServiceTest) TestATakenAddressIsAConflict() {
	form := test.added("taken", "ana-id")
	Assert.Equals("conflict", form.Problem)
	Assert.Equals("", form.Status)
	Assert.Equals("taken", form.Address)
}

func (test *AliasFormServiceTest) TestAnAddressTheServerRefusesIsInvalid() {
	form := test.added("wrong", "ana-id")
	Assert.Equals("invalid", form.Problem)
}

func (test *AliasFormServiceTest) TestABadRequestIsInvalid() {
	form := test.added("rude", "ana-id")
	Assert.Equals("invalid", form.Problem)
}

func (test *AliasFormServiceTest) TestAddWithoutAdminRightsIsForbidden() {
	form := test.added("locked", "ana-id")
	Assert.Equals("forbidden", form.Problem)
	Assert.Equals(false, form.HasAdded)
}

func (test *AliasFormServiceTest) TestAddWithoutTheServerIsUnavailable() {
	form := test.added("offline", "ana-id")
	Assert.Equals("unavailable", form.Problem)
	Assert.Equals(false, form.Busy)
}

func (test *AliasFormServiceTest) TestASecondAliasIsAddedToTheList() {
	form := test.added("sales", "bo-id")
	form.AddWith("help", "ana-id")
	Assert.Equals(2, len(form.Added))
	Assert.Equals("help@example.com now reaches Ana Ruiz.", form.Status)
}

func (test *AliasFormServiceTest) TestAddKeepsTheMemberThatWasChosen() {
	form := test.added("taken", "bo-id")
	Assert.Equals("bo-id", form.MemberId)
}

func (test *AliasFormServiceTest) TestSuffixIsTheDomainInLowercase() {
	form := &AliasFormService{DomainId: "Example.COM"}
	Assert.Equals("@example.com", form.Suffix())
}

func (test *AliasFormServiceTest) TestAddressHintNamesTheDomain() {
	form := test.opened()
	Assert.Equals("Ends with @example.com. The address must not belong to a person, an alias or another channel.", form.AddressHint())
}

func (test *AliasFormServiceTest) TestAddressErrorExplainsAnInvalidAddress() {
	form := test.opened()
	Assert.Equals("Use 1 to 64 lowercase letters, digits, dots, hyphens, underscores or plus signs. Do not start or end with a dot.", form.AddressError("invalidAddress"))
}

func (test *AliasFormServiceTest) TestAddressErrorExplainsATakenAddress() {
	form := test.opened()
	Assert.Equals("This address already belongs to a member, alias or channel.", form.AddressError("conflict"))
}

func (test *AliasFormServiceTest) TestAddressErrorExplainsAnAddressTheServerRefuses() {
	form := test.opened()
	Assert.Equals("This is not a valid address in example.com.", form.AddressError("invalid"))
}

func (test *AliasFormServiceTest) TestAddressErrorIsEmptyForAMemberProblem() {
	form := test.opened()
	Assert.Equals("", form.AddressError("noMember"))
}

func (test *AliasFormServiceTest) session() *AdminSessionService {
	session := &AdminSessionService{}
	test.construct(session)
	return session
}
