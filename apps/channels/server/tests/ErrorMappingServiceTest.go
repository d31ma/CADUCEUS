import (
	"errors"

	"mesail.dev/packages/channels"
)

// @Test(HealthTestSetup, {"partition":"tests"})
type ErrorMappingServiceTest struct {
	behaviors *BehaviorContext
}

func (test *ErrorMappingServiceTest) TestNoChannelErrorStaysNoError() {
	Assert.IsTrue((&ErrorMappingService{}).ChannelError(nil) == nil)
}

func (test *ErrorMappingServiceTest) TestAnUnknownChannelErrorIsUnavailableWithTheAppNumber() {
	mapped := (&ErrorMappingService{}).ChannelError(errors.New("the store is down"))
	Assert.Equals(&Unavailable{Message: "the channel service is not available", ErrorNumber: 2027}, mapped)
}

func (test *ErrorMappingServiceTest) TestAChannelStoreFailureKeepsItsNumberButNotItsCause() {
	mapped := (&ErrorMappingService{}).ChannelError(channels.Unavailable{}.StoreDown(errors.New("dial tcp")))
	Assert.Equals(&Unavailable{Message: "the channel service is not available", ErrorNumber: 2020}, mapped)
}

func (test *ErrorMappingServiceTest) TestEveryKnownChannelErrorKeepsItsMessageAndItsCaseNumber() {
	service := &ErrorMappingService{}
	Assert.Equals(&InvalidRequest{Message: "visibility is public or private", Field: "visibility", ErrorNumber: 2004}, service.ChannelError(channels.InvalidRequest{}.BadVisibility()))
	Assert.Equals(&Unauthenticated{Message: "the access token was refused", ErrorNumber: 2001}, service.ChannelError(channels.Unauthenticated{}.TokenRefused()))
	Assert.Equals(&Forbidden{Message: "join the channel to change its topic", ErrorNumber: 2014}, service.ChannelError(channels.Forbidden{}.TopicNeedsMember()))
	Assert.Equals(&NotFound{Message: "no such channel", ErrorNumber: 2007}, service.ChannelError(channels.NotFound{}.BadChannelId()))
	Assert.Equals(&Conflict{Message: "the channel name releases is already used in this domain", ErrorNumber: 2006}, service.ChannelError(channels.Conflict{}.NameUsed("releases")))
	Assert.Equals(&Duplicate{Message: "taken", ErrorNumber: 2018}, service.ChannelError(channels.Duplicate{}.AddressTaken("taken")))
}

func (test *ErrorMappingServiceTest) TestNoOutboxErrorStaysNoError() {
	Assert.IsTrue((&ErrorMappingService{}).OutboxError(nil) == nil)
}

func (test *ErrorMappingServiceTest) TestAMissingOutboxDocumentKeepsItsCaseNumber() {
	mapped := (&ErrorMappingService{}).OutboxError(channels.NotFound{}.OutboxDocument())
	Assert.Equals(&NotFound{Message: "no such outbox document", ErrorNumber: 2025}, mapped)
}

func (test *ErrorMappingServiceTest) TestAnOutboxSettingsFailureKeepsItsCaseNumber() {
	mapped := (&ErrorMappingService{}).OutboxError(channels.Unavailable{}.OutboxSettingsMissing(errors.New("no subscribers")))
	Assert.Equals(&Unavailable{Message: "the outbox is not available", ErrorNumber: 2022}, mapped)
}

func (test *ErrorMappingServiceTest) TestAnUnknownOutboxErrorIsUnavailableWithTheAppNumber() {
	mapped := (&ErrorMappingService{}).OutboxError(errors.New("the settings are missing"))
	Assert.Equals(&Unavailable{Message: "the outbox is not available", ErrorNumber: 2028}, mapped)
}
