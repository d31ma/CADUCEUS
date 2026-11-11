// @Test(NotificationTestSetup, {"partition":"tests"})
type NotificationPlaceServiceTest struct {
	behaviors *BehaviorContext
}

func (test *NotificationPlaceServiceTest) places() *NotificationPlaceService {
	sidebar := &SidebarService{
		Channels: []ConversationRow{{ConversationId: "c-general", Kind: "channel", Title: "general", Initials: "GE"}},
		Directs:  []ConversationRow{{ConversationId: "c-bo", Kind: "dm", Title: "Bo Lee", Initials: "BL"}},
		Emails:   []ConversationRow{{ConversationId: "c-jane", Kind: "email", Title: "Jane Porter", Initials: "JP"}},
	}
	return &NotificationPlaceService{sidebar: sidebar, kinds: &NotificationKindService{}}
}

func (test *NotificationPlaceServiceTest) TestNameMarksChannels() {
	places := test.places()
	Assert.Equals("#general", places.Name("c-general"))
	Assert.Equals("Bo Lee", places.Name("c-bo"))
	Assert.Equals("Jane Porter", places.Name("c-jane"))
}

func (test *NotificationPlaceServiceTest) TestUnknownConversationHasNoName() {
	places := test.places()
	Assert.Equals("", places.Name("c-gone"))
	Assert.Equals("", places.Initials("c-gone"))
}

func (test *NotificationPlaceServiceTest) TestInitialsComeFromTheSidebar() {
	Assert.Equals("BL", test.places().Initials("c-bo"))
}

func (test *NotificationPlaceServiceTest) TestWordFitsTheKind() {
	places := test.places()
	Assert.Equals("in", places.Word("mention"))
	Assert.Equals("from", places.Word("dm"))
	Assert.Equals("in", places.Word("thread_reply"))
	Assert.Equals("from", places.Word("email"))
	Assert.Equals("in", places.Word("something"))
}
