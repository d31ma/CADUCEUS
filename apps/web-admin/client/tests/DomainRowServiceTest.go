// @Test(DomainDirectoryTestSetup, {"partition":"tests"})
type DomainRowServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
}

func (test *DomainRowServiceTest) rows() *DomainRowService {
	return &DomainRowService{records: &DomainRecordService{}}
}

func (test *DomainRowServiceTest) TestAnActiveRowHasAnOkTagAndManage() {
	row := test.rows().Row(Domain{DomainId: "example.com", Name: "example.com", State: "active", Dns: DnsState{Mx: "ok", Spf: "ok", Dkim: "ok", Dmarc: "ok"}})
	Assert.Equals("example.com", row.DomainId)
	Assert.Equals("Active", row.StateText)
	Assert.Equals("success", row.StateColor)
	Assert.Equals("check", row.StateIcon)
	Assert.Equals(false, row.HasNote)
	Assert.Equals("Manage", row.ActionText)
	Assert.Equals("outlined", row.ActionVariant)
	Assert.Equals("DNS and settings for example.com", row.ActionLabel)
	Assert.Equals("DNS: all 4 records OK", row.MarksLabel)
	Assert.Equals("", row.Tone)
}

func (test *DomainRowServiceTest) TestAPendingRowAsksToSetUpDns() {
	row := test.rows().Row(Domain{DomainId: "new.example.net", Name: "new.example.net", State: "pending"})
	Assert.Equals("Pending", row.StateText)
	Assert.Equals("warning", row.StateColor)
	Assert.Equals("clock", row.StateIcon)
	Assert.Equals("Mail is not accepted yet: MX and DKIM must be OK.", row.StateNote)
	Assert.Equals(true, row.HasNote)
	Assert.Equals("Set up DNS", row.ActionText)
	Assert.Equals("flat", row.ActionVariant)
	Assert.Equals("Set up DNS for new.example.net", row.ActionLabel)
	Assert.Equals(4, len(row.Marks))
}

func (test *DomainRowServiceTest) TestADisabledRowIsMuted() {
	row := test.rows().Row(Domain{DomainId: "old.example.com", Name: "old.example.com", State: "disabled"})
	Assert.Equals("Disabled", row.StateText)
	Assert.Equals("outlined", row.StateVariant)
	Assert.Equals("Mail to this domain is refused. Messages are kept.", row.StateNote)
	Assert.Equals("muted", row.Tone)
	Assert.Equals("Manage", row.ActionText)
}

func (test *DomainRowServiceTest) TestARemovingRowTurnsManageOff() {
	row := test.rows().Row(Domain{DomainId: "gone.example.org", Name: "gone.example.org", State: "removing"})
	Assert.Equals("Removing", row.StateText)
	Assert.Equals("error", row.StateColor)
	Assert.Equals("trash", row.StateIcon)
	Assert.Equals("Mail to this domain is refused. Mesail removes its members and addresses in the background.", row.StateNote)
	Assert.Equals("muted", row.Tone)
	Assert.Equals("Manage", row.ActionText)
	Assert.Equals(true, row.ActionOff)
	Assert.Equals("Manage is off while Mesail removes gone.example.org", row.ActionLabel)
}

func (test *DomainRowServiceTest) TestAnActiveRowKeepsManageOn() {
	row := test.rows().Row(Domain{DomainId: "example.com", Name: "example.com", State: "active"})
	Assert.Equals(false, row.ActionOff)
}

func (test *DomainRowServiceTest) TestViewOfAnUnknownStateIsAnOutline() {
	view := test.rows().View(Domain{DomainId: "x.example", State: "moving"})
	Assert.Equals("Unknown", view.StateText)
	Assert.Equals("outlined", view.StateVariant)
	Assert.Equals(false, view.Pending)
}

func (test *DomainRowServiceTest) TestViewOfAPendingDomainListsFourRecords() {
	view := test.rows().View(Domain{DomainId: "new.example.net", Name: "new.example.net", State: "pending", DkimSelector: "ms2026a"})
	Assert.Equals(true, view.Pending)
	Assert.Equals(4, len(view.Records))
}
