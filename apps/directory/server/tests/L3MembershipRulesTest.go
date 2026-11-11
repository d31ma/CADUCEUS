import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type L3MembershipRulesTest struct {
	behaviors *BehaviorContext
}

func (test *L3MembershipRulesTest) TestWithAddsTheDomainAndMakesTheFirstDomainTheDefault() {
	person := PersonRules{}.With(PersonDocument{}, "a.test")
	Assert.Equals("a.test", strings.Join(person.DomainIds, ","))
	Assert.Equals("a.test", person.DefaultDomainId)
}

func (test *L3MembershipRulesTest) TestWithKeepsTheDefaultAndAddsNoSecondCopy() {
	person := PersonDocument{DomainIds: []string{"a.test", "b.test"}, DefaultDomainId: "a.test"}
	same := PersonRules{}.With(person, "b.test")
	more := PersonRules{}.With(person, "c.test")
	Assert.Equals("a.test,b.test", strings.Join(same.DomainIds, ","))
	Assert.Equals("a.test,b.test,c.test", strings.Join(more.DomainIds, ","))
	Assert.Equals("a.test", more.DefaultDomainId)
	Assert.Equals("a.test,b.test", strings.Join(person.DomainIds, ","))
}

func (test *L3MembershipRulesTest) TestWithAdminAddsTheDomainOnce() {
	person := PersonDocument{AdminDomainIds: []string{"a.test"}}
	changed := PersonRules{}.WithAdmin(person, "a.test", true)
	added := PersonRules{}.WithAdmin(person, "b.test", true)
	Assert.Equals("a.test", strings.Join(changed.AdminDomainIds, ","))
	Assert.Equals("a.test,b.test", strings.Join(added.AdminDomainIds, ","))
}

func (test *L3MembershipRulesTest) TestWithAdminFalseRemovesTheDomain() {
	person := PersonDocument{AdminDomainIds: []string{"a.test", "b.test"}}
	changed := PersonRules{}.WithAdmin(person, "a.test", false)
	Assert.Equals("b.test", strings.Join(changed.AdminDomainIds, ","))
	Assert.Equals("a.test,b.test", strings.Join(person.AdminDomainIds, ","))
}

func (test *L3MembershipRulesTest) TestWithoutTheDefaultDomainMovesTheDefaultToTheNextDomain() {
	person := PersonDocument{DomainIds: []string{"a.test", "b.test"}, AdminDomainIds: []string{"a.test"}, DefaultDomainId: "a.test"}
	changed := PersonRules{}.Without(person, "a.test")
	Assert.Equals("b.test", strings.Join(changed.DomainIds, ","))
	Assert.Equals("", strings.Join(changed.AdminDomainIds, ","))
	Assert.Equals("b.test", changed.DefaultDomainId)
}

func (test *L3MembershipRulesTest) TestWithoutAnotherDomainKeepsTheDefault() {
	person := PersonDocument{DomainIds: []string{"a.test", "b.test"}, DefaultDomainId: "a.test"}
	changed := PersonRules{}.Without(person, "b.test")
	Assert.Equals("a.test", strings.Join(changed.DomainIds, ","))
	Assert.Equals("a.test", changed.DefaultDomainId)
}

func (test *L3MembershipRulesTest) TestWithoutTheLastDomainClearsTheDefault() {
	person := PersonDocument{DomainIds: []string{"a.test"}, DefaultDomainId: "a.test"}
	changed := PersonRules{}.Without(person, "a.test")
	Assert.Equals(0, len(changed.DomainIds))
	Assert.Equals("", changed.DefaultDomainId)
}

func (test *L3MembershipRulesTest) TestMemberTellsWhetherThePersonHoldsTheDomain() {
	person := PersonDocument{DomainIds: []string{"a.test"}}
	Assert.IsTrue(PersonRules{}.Member(person, "a.test"), "want a member")
	Assert.IsTrue(!PersonRules{}.Member(person, "b.test"), "want no member")
}

func (test *L3MembershipRulesTest) TestAnOperatorManagesEveryAccount() {
	manager := AccountManager{Operator: true}
	Assert.IsTrue(manager.Manages([]string{"a.test"}), "want managed")
	Assert.IsTrue(manager.Manages([]string{}), "want managed without domains")
}

func (test *L3MembershipRulesTest) TestAnAdminManagesOnlyAccountsWhollyInTheirDomains() {
	manager := AccountManager{AdminDomainIds: []string{"a.test", "b.test"}}
	Assert.IsTrue(manager.Manages([]string{"a.test", "b.test"}), "want managed")
	Assert.IsTrue(!manager.Manages([]string{"a.test", "c.test"}), "want not managed")
	Assert.IsTrue(!manager.Manages([]string{}), "want not managed without domains")
}

func (test *L3MembershipRulesTest) TestPageSizeDefaultsToTwenty() {
	size, err := PageWindow{}.Size(0)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(20, size)
}

func (test *L3MembershipRulesTest) TestPageSizeKeepsAnAllowedLimit() {
	size, err := PageWindow{}.Size(100)
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(100, size)
}

func (test *L3MembershipRulesTest) TestPageSizeRefusesALimitAboveOneHundred() {
	_, err := PageWindow{}.Size(101)
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("limit", invalid.Field)
}

func (test *L3MembershipRulesTest) TestPageSizeRefusesANegativeLimit() {
	_, err := PageWindow{}.Size(-1)
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
}

func (test *L3MembershipRulesTest) TestAnEmptyCursorStartsAtTheTop() {
	offset, err := PageWindow{}.Offset("")
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(0, offset)
}

func (test *L3MembershipRulesTest) TestACursorGivesItsOffset() {
	offset, err := PageWindow{}.Offset("7")
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(7, offset)
}

func (test *L3MembershipRulesTest) TestACursorThatIsNotANumberIsRefused() {
	_, err := PageWindow{}.Offset("abc")
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("cursor", invalid.Field)
}

func (test *L3MembershipRulesTest) TestANegativeCursorIsRefused() {
	_, err := PageWindow{}.Offset("-3")
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
}

func (test *L3MembershipRulesTest) TestNextIsEmptyOnTheLastPage() {
	Assert.Equals("", PageWindow{}.Next(5, 5))
	Assert.Equals("3", PageWindow{}.Next(3, 5))
}

func (test *L3MembershipRulesTest) TestQueryReadsTheTextCursorAndLimit() {
	query, err := PageWindow{}.Query(url.Values{"query": {"ana"}, "cursor": {"2"}, "limit": {"5"}})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals("ana", query.Text)
	Assert.Equals("2", query.Cursor)
	Assert.Equals(5, query.Limit)
}

func (test *L3MembershipRulesTest) TestQueryWithoutALimitLeavesItZero() {
	query, err := PageWindow{}.Query(url.Values{})
	Assert.Equals("<nil>", fmt.Sprint(err))
	Assert.Equals(0, query.Limit)
}

func (test *L3MembershipRulesTest) TestQueryRefusesALimitThatIsNotANumber() {
	_, err := PageWindow{}.Query(url.Values{"limit": {"many"}})
	var invalid *InvalidRequest
	Assert.IsTrue(errors.As(err, &invalid), "want InvalidRequest")
	Assert.Equals("limit", invalid.Field)
}

func (test *L3MembershipRulesTest) TestPlainSummaryHidesTheUsernameAndAdminFlag() {
	summary := MemberSummaries{}.Plain(test.member())
	Assert.Equals("kc-ana", summary.MemberId)
	Assert.Equals("", summary.Username)
	Assert.Equals("Ana", summary.DisplayName)
	Assert.Equals("ana@a.test", summary.PrimaryAddress)
	Assert.IsTrue(!summary.Admin, "want no admin flag")
}

func (test *L3MembershipRulesTest) TestAdminSummaryShowsTheUsernameAdminFlagAndManagement() {
	summary := MemberSummaries{}.Admin(test.member(), true)
	Assert.Equals("ana", summary.Username)
	Assert.IsTrue(summary.Admin, "want the admin flag")
	Assert.IsTrue(summary.AccountManaged, "want the account managed")
}

func (test *L3MembershipRulesTest) member() MemberDocument {
	return MemberDocument{MemberId: "kc-ana", Username: "ana", DisplayName: "Ana", PrimaryAddress: "ana@a.test", Admin: true, State: "active"}
}
