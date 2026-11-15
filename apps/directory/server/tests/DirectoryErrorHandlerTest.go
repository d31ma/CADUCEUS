import (
	"errors"
	"strings"
)

// @Test(DirectoryTestSetup, {"partition":"tests"})
type DirectoryErrorHandlerTest struct {
	behaviors *BehaviorContext
}

func (test *DirectoryErrorHandlerTest) handler() *DirectoryErrorHandler {
	return &DirectoryErrorHandler{}
}

func (test *DirectoryErrorHandlerTest) TestUnexpectedErrorIsTheFirstNumber() {
	answer := test.handler().Unexpected(errors.New("disk full at /var/lib"))
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1000]: Something went wrong.", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestTokenRefusedIsNumbered() {
	answer := test.handler().Unauthenticated(Unauthenticated{}.TokenRefused())
	Assert.Equals("unauthenticated", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1001]: the access token was refused", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestNotOperatorIsNumbered() {
	answer := test.handler().Forbidden(Forbidden{}.NotOperator())
	Assert.Equals("forbidden", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1002]: only an Operator may do this", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestHiddenRouteIsNumbered() {
	answer := test.handler().Missing(NotFound{}.Hidden())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1003]: not found", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestNotMemberIsNumbered() {
	answer := test.handler().Forbidden(Forbidden{}.NotMember())
	Assert.Equals("forbidden", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1004]: you are not a member of this domain", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestDomainNotServedIsNumbered() {
	answer := test.handler().Forbidden(Forbidden{}.DomainNotServed())
	Assert.Equals("forbidden", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1005]: this domain is not hosted here", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestNotAccountManagerIsNumbered() {
	answer := test.handler().Forbidden(Forbidden{}.NotAccountManager())
	Assert.Equals("forbidden", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1006]: only an Operator or an admin of every domain of this person may change the account", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestLimitOutOfRangeIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.LimitOutOfRange())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("limit", answer.Field)
	Assert.Equals("[MS1007]: limit must be between 1 and 100", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestLimitNotNumberIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.LimitNotNumber())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("limit", answer.Field)
	Assert.Equals("[MS1008]: limit must be a whole number", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestBadCursorIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.BadCursor())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("cursor", answer.Field)
	Assert.Equals("[MS1009]: the cursor is not one this service gave out", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestMissingDomainIsNumbered() {
	answer := test.handler().Missing(NotFound{}.Domain("example.org"))
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1010]: no hosted domain example.org", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestBadDomainNameIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.BadDomainName("not a domain name"))
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("name", answer.Field)
	Assert.Equals("[MS1011]: not a domain name", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestDomainHostedIsNumbered() {
	answer := test.handler().Duplicate(Duplicate{}.DomainHosted("example.com"))
	Assert.Equals("duplicate", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1012]: example.com is already hosted", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestDnsFailedIsNumbered() {
	answer := test.handler().Unavailable(Unavailable{}.Dns(errors.New("timeout")))
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1013]: the DNS lookup failed", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestAddressDomainNotHostedIsNumbered() {
	answer := test.handler().NotHosted(DomainNotHosted{}.Address())
	Assert.Equals("domain_not_hosted", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1014]: this domain is not hosted here", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestMissingAddressIsNumbered() {
	answer := test.handler().Missing(NotFound{}.Address())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1015]: no such address", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestAddressOutsideDomainIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.AddressOutsideDomain())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("address", answer.Field)
	Assert.Equals("[MS1016]: the address must be in your own domain", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestMissingMemberIsNumbered() {
	answer := test.handler().Missing(NotFound{}.Member())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1017]: no such member in your domain", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestAddressInUseIsNumbered() {
	answer := test.handler().Duplicate(Duplicate{}.AddressInUse("sales@example.com"))
	Assert.Equals("duplicate", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1018]: sales@example.com is already in use", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestBadNameIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.BadName("username", "too short"))
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("username", answer.Field)
	Assert.Equals("[MS1019]: too short", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestNameTakenIsNumbered() {
	answer := test.handler().Duplicate(Duplicate{}.NameTaken("sales is taken"))
	Assert.Equals("duplicate", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1020]: sales is taken", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestMissingChannelAddressIsNumbered() {
	answer := test.handler().Missing(NotFound{}.ChannelAddress())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1021]: no such channel address", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestNoChannelIdIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.NoChannelId())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("channelId", answer.Field)
	Assert.Equals("[MS1022]: a channel id is required", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestTooManyAddressesIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.TooManyAddresses())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("addresses", answer.Field)
	Assert.Equals("[MS1023]: at most 100 addresses can be resolved at once", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestBadAddressesIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.BadAddresses("bad address"))
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("addresses", answer.Field)
	Assert.Equals("[MS1024]: bad address", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestUsernameIsAddressIsNumbered() {
	answer := test.handler().Duplicate(Duplicate{}.UsernameIsAddress("sales"))
	Assert.Equals("duplicate", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1025]: sales is used by an alias or a channel address", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestNoNewPersonNameIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.NoNewPersonName())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("displayName", answer.Field)
	Assert.Equals("[MS1026]: a display name is required for a new person", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestNoFirstPasswordIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.NoFirstPassword())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("firstPassword", answer.Field)
	Assert.Equals("[MS1027]: a first password is required for a new person", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestUsernameRetiredIsNumbered() {
	answer := test.handler().Duplicate(Duplicate{}.UsernameRetired("bo"))
	Assert.Equals("duplicate", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1028]: bo belonged to a deleted person and is never reused", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestAlreadyMemberIsNumbered() {
	answer := test.handler().Duplicate(Duplicate{}.AlreadyMember("bo", "example.com"))
	Assert.Equals("duplicate", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1029]: bo is already a member of example.com", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestUsernameTakenIsNumbered() {
	answer := test.handler().Duplicate(Duplicate{}.UsernameTaken())
	Assert.Equals("duplicate", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1030]: this username is already taken", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestPasswordRefusedIsNumbered() {
	answer := test.handler().PasswordPolicy(PasswordPolicy{}.Refused("the password is too short"))
	Assert.Equals("password_policy", answer.Code)
	Assert.Equals("password", answer.Field)
	Assert.Equals("[MS1031]: the password is too short", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestMissingAccountIsNumbered() {
	answer := test.handler().Missing(NotFound{}.Account())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1032]: no such account", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestIdentityServerDownIsNumbered() {
	answer := test.handler().Unavailable(Unavailable{}.IdentityServer(errors.New("502")))
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1033]: the identity server is not available", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestLastAdminIsNumbered() {
	answer := test.handler().LastAdmin(LastAdmin{}.LastAdmin())
	Assert.Equals("last_admin", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1034]: a domain must keep at least one admin", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestBadStateIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.BadState())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("state", answer.Field)
	Assert.Equals("[MS1035]: state must be active or disabled", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestNoNewPasswordIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.NoNewPassword())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("newPassword", answer.Field)
	Assert.Equals("[MS1036]: a new password is required", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestNotMemberOfIsNumbered() {
	answer := test.handler().Missing(NotFound{}.NotMemberOf("example.org"))
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1037]: you are not a member of example.org", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestMissingPersonIsNumbered() {
	answer := test.handler().Missing(NotFound{}.Person())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1038]: no such person", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestPersonDeletedIsNumbered() {
	answer := test.handler().Missing(NotFound{}.PersonDeleted())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1039]: this person was deleted", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestSeedTurnedOffIsNumbered() {
	answer := test.handler().Missing(NotFound{}.SeedTurnedOff())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1040]: not found", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestNoPersonIdIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.NoPersonId())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("personId", answer.Field)
	Assert.Equals("[MS1041]: a person id is required", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestNoDisplayNameIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.NoDisplayName())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("displayName", answer.Field)
	Assert.Equals("[MS1042]: a display name is required", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestTooManyMembershipsIsNumbered() {
	answer := test.handler().Invalid(InvalidRequest{}.TooManyMemberships())
	Assert.Equals("invalid_request", answer.Code)
	Assert.Equals("memberships", answer.Field)
	Assert.Equals("[MS1043]: a seed call adds at most 2 domains; seed the person again for more", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestOutboxSettingsIsNumbered() {
	answer := test.handler().Unavailable(Unavailable{}.OutboxSettings(errors.New("missing")))
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1044]: the outbox is not configured", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestMissingDeadEventIsNumbered() {
	answer := test.handler().Missing(NotFound{}.DeadEvent())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1045]: no such dead event", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestMissingOutboxCollectionIsNumbered() {
	answer := test.handler().Missing(NotFound{}.OutboxCollection())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1046]: no such outbox collection", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestMissingOutboxDocumentIsNumbered() {
	answer := test.handler().Missing(NotFound{}.OutboxDocument())
	Assert.Equals("not_found", answer.Code)
	Assert.Equals("", answer.Field)
	Assert.Equals("[MS1047]: no such outbox document", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestAStoreFailureIsNumbered() {
	answer := test.handler().Store(YonResourceError{})
	Assert.Equals("unavailable", answer.Code)
	Assert.Equals("[MS1048]: the directory store is not available", answer.Message)
}

func (test *DirectoryErrorHandlerTest) TestUnavailableHidesTheCause() {
	answer := test.handler().Unavailable(Unavailable{}.IdentityServer(errors.New("secret detail")))
	Assert.IsTrue(!strings.Contains(answer.Message, "secret detail"), "the cause must not reach the answer")
}

func (test *DirectoryErrorHandlerTest) TestUnavailableKeepsTheCauseForLogs() {
	cause := errors.New("secret detail")
	Assert.IsTrue(errors.Is(Unavailable{}.Dns(cause), cause), "want the cause behind Unwrap")
}
