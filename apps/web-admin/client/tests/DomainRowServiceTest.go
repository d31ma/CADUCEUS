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
	Assert.Equals("ms-tag ms-tag--ok", row.StateClass)
	Assert.Equals("ms-icon ms-i-check", row.StateIcon)
	Assert.Equals(false, row.HasNote)
	Assert.Equals("Manage", row.ActionText)
	Assert.Equals("ms-btn ms-btn--sm", row.ActionClass)
	Assert.Equals("DNS and settings for example.com", row.ActionLabel)
	Assert.Equals("DNS: all 4 records OK", row.MarksLabel)
	Assert.Equals("", row.RowClass)
}

func (test *DomainRowServiceTest) TestAPendingRowAsksToSetUpDns() {
	row := test.rows().Row(Domain{DomainId: "new.example.net", Name: "new.example.net", State: "pending"})
	Assert.Equals("Pending", row.StateText)
	Assert.Equals("ms-tag ms-tag--warn", row.StateClass)
	Assert.Equals("ms-icon ms-i-clock", row.StateIcon)
	Assert.Equals("Mail is not accepted yet: MX and DKIM must be OK.", row.StateNote)
	Assert.Equals(true, row.HasNote)
	Assert.Equals("Set up DNS", row.ActionText)
	Assert.Equals("ms-btn ms-btn--sm ms-btn--primary", row.ActionClass)
	Assert.Equals("Set up DNS for new.example.net", row.ActionLabel)
	Assert.Equals(4, len(row.Marks))
}

func (test *DomainRowServiceTest) TestADisabledRowIsMuted() {
	row := test.rows().Row(Domain{DomainId: "old.example.com", Name: "old.example.com", State: "disabled"})
	Assert.Equals("Disabled", row.StateText)
	Assert.Equals("ms-tag ms-tag--outline", row.StateClass)
	Assert.Equals("Mail to this domain is refused. Messages are kept.", row.StateNote)
	Assert.Equals("is-muted", row.RowClass)
	Assert.Equals("Manage", row.ActionText)
}

func (test *DomainRowServiceTest) TestViewOfAnUnknownStateIsAnOutline() {
	view := test.rows().View(Domain{DomainId: "x.example", State: "moving"})
	Assert.Equals("Unknown", view.StateText)
	Assert.Equals("ms-tag ms-tag--outline", view.StateClass)
	Assert.Equals(false, view.Pending)
}

func (test *DomainRowServiceTest) TestViewOfAPendingDomainListsFourRecords() {
	view := test.rows().View(Domain{DomainId: "new.example.net", Name: "new.example.net", State: "pending", DkimSelector: "ms2026a"})
	Assert.Equals(true, view.Pending)
	Assert.Equals(4, len(view.Records))
}
