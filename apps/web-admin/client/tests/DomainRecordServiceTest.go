import "strings"

// @Test(DomainDirectoryTestSetup, {"partition":"tests"})
type DomainRecordServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	directoryClient *DirectoryClient
}

func (test *DomainRecordServiceTest) pending() Domain {
	return Domain{DomainId: "new.example.net", Name: "new.example.net", State: "pending", DkimSelector: "ms2026a", Dns: DnsState{Mx: "ok", Spf: "ok", Dkim: "missing", Dmarc: "wrong"}}
}

func (test *DomainRecordServiceTest) TestMarksShowEachRecordInOrder() {
	marks := (&DomainRecordService{}).Marks(test.pending().Dns)
	Assert.Equals(4, len(marks))
	Assert.Equals("MX", marks[0].Name)
	Assert.Equals("SPF", marks[1].Name)
	Assert.Equals("DKIM", marks[2].Name)
	Assert.Equals("DMARC", marks[3].Name)
}

func (test *DomainRecordServiceTest) TestAnOkMarkIsGreenWithACheck() {
	mark := (&DomainRecordService{}).Marks(test.pending().Dns)[0]
	Assert.Equals("success", mark.Color)
	Assert.Equals("check", mark.Icon)
	Assert.Equals("OK", mark.Words)
}

func (test *DomainRecordServiceTest) TestAMissingMarkShowsAClose() {
	mark := (&DomainRecordService{}).Marks(test.pending().Dns)[2]
	Assert.Equals("error", mark.Color)
	Assert.Equals("close", mark.Icon)
	Assert.Equals("missing", mark.Words)
}

func (test *DomainRecordServiceTest) TestAWrongMarkShowsAnAlert() {
	mark := (&DomainRecordService{}).Marks(test.pending().Dns)[3]
	Assert.Equals("warning", mark.Color)
	Assert.Equals("alert", mark.Icon)
	Assert.Equals("wrong value", mark.Words)
}

func (test *DomainRecordServiceTest) TestAnEmptyStateIsNotChecked() {
	mark := (&DomainRecordService{}).Marks(DnsState{})[0]
	Assert.Equals("outlined", mark.Variant)
	Assert.Equals("minus-circle", mark.Icon)
	Assert.Equals("not checked", mark.Words)
}

func (test *DomainRecordServiceTest) TestAnUnknownStateIsNotChecked() {
	mark := (&DomainRecordService{}).Marks(DnsState{Mx: "later"})[0]
	Assert.Equals("outlined", mark.Variant)
}

func (test *DomainRecordServiceTest) TestMarksLabelNamesEachRecord() {
	label := (&DomainRecordService{}).MarksLabel(test.pending().Dns)
	Assert.Equals("DNS: MX OK, SPF OK, DKIM missing, DMARC wrong value", label)
}

func (test *DomainRecordServiceTest) TestMarksLabelSaysWhenAllAreOk() {
	label := (&DomainRecordService{}).MarksLabel(DnsState{Mx: "ok", Spf: "ok", Dkim: "ok", Dmarc: "ok"})
	Assert.Equals("DNS: all 4 records OK", label)
}

func (test *DomainRecordServiceTest) TestRecordsUseTheHostOfEachRecord() {
	records := (&DomainRecordService{}).Records(test.pending())
	Assert.Equals("new.example.net", records[0].Host)
	Assert.Equals("new.example.net", records[1].Host)
	Assert.Equals("ms2026a._domainkey.new.example.net", records[2].Host)
	Assert.Equals("_dmarc.new.example.net", records[3].Host)
}

func (test *DomainRecordServiceTest) TestRecordsNameTheTypeOfEachRecord() {
	records := (&DomainRecordService{}).Records(test.pending())
	Assert.Equals("MX", records[0].Type)
	Assert.Equals("TXT", records[1].Type)
	Assert.Equals("TXT", records[2].Type)
	Assert.Equals("TXT", records[3].Type)
	Assert.Equals("MX record", records[0].Title)
}

func (test *DomainRecordServiceTest) TestOnlyTheDkimRecordHasASelector() {
	records := (&DomainRecordService{}).Records(test.pending())
	Assert.Equals(false, records[0].HasSelector)
	Assert.Equals(true, records[2].HasSelector)
	Assert.Equals("ms2026a", records[2].Selector)
	Assert.Equals("Required to receive and sign mail · selector ms2026a", records[2].Need)
}

func (test *DomainRecordServiceTest) TestAnOkRecordSaysItFoundTheRightValue() {
	record := (&DomainRecordService{}).Records(test.pending())[0]
	Assert.Equals("success", record.TagColor)
	Assert.Equals("OK", record.TagText)
	Assert.Equals("record-card", record.CardClass)
	Assert.Equals("result result--ok", record.ResultClass)
	Assert.Equals("check", record.ResultIcon)
	Assert.Equals("Found the right value.", record.ResultText)
}

func (test *DomainRecordServiceTest) TestAMissingRecordSaysWhereToCreateIt() {
	record := (&DomainRecordService{}).Records(test.pending())[2]
	Assert.Equals("error", record.TagColor)
	Assert.Equals("Missing", record.TagText)
	Assert.Equals("record-card record-card--bad", record.CardClass)
	Assert.Equals("result result--bad", record.ResultClass)
	Assert.Equals("No TXT record found at ms2026a._domainkey.new.example.net. Create it, then select Check again.", record.ResultText)
}

func (test *DomainRecordServiceTest) TestAWrongRecordSaysToFixIt() {
	record := (&DomainRecordService{}).Records(test.pending())[3]
	Assert.Equals("warning", record.TagColor)
	Assert.Equals("Wrong value", record.TagText)
	Assert.Equals("record-card record-card--wrong", record.CardClass)
	Assert.Equals("The record at _dmarc.new.example.net has a different value. Fix it at your DNS provider, then select Check again.", record.ResultText)
}

func (test *DomainRecordServiceTest) TestAnUncheckedRecordAsksToCheck() {
	record := (&DomainRecordService{}).Records(Domain{Name: "a.example", DkimSelector: "s1"})[0]
	Assert.Equals("outlined", record.TagVariant)
	Assert.Equals("result", record.ResultClass)
	Assert.Equals("Not checked", record.TagText)
	Assert.Equals("Not checked yet. Select Check again.", record.ResultText)
}

func (test *DomainRecordServiceTest) TestRecordsHaveCopyLabels() {
	record := (&DomainRecordService{}).Records(test.pending())[0]
	Assert.Equals("Copy MX host", record.CopyLabel)
	Assert.Equals("Copied MX host", record.CopiedLabel)
	Assert.Equals("record-mx", record.TitleId)
}

func (test *DomainRecordServiceTest) TestStatusCountsTheOkRecords() {
	Assert.Equals("2 of 4 records are OK.", (&DomainRecordService{}).Status(test.pending().Dns))
	Assert.Equals("0 of 4 records are OK.", (&DomainRecordService{}).Status(DnsState{}))
}

func (test *DomainRecordServiceTest) TestWaitingTellsTheStateOfMxAndDkim() {
	waiting := (&DomainRecordService{}).Waiting(test.pending().Dns)
	Assert.Equals("MX is OK. DKIM is missing. Create or fix the records below, then check again. People can join this domain after mail is accepted.", waiting)
}

func (test *DomainRecordServiceTest) TestWaitingTellsAWrongOrUncheckedRecord() {
	waiting := (&DomainRecordService{}).Waiting(DnsState{Dkim: "wrong"})
	Assert.IsTrue(strings.HasPrefix(waiting, "MX is not checked yet. DKIM has a wrong value."))
}
