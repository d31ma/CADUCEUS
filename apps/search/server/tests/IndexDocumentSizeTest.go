import (
	"strings"

	"mesail.dev/packages/search"
)

// @Test(SearchTestSetup, {"partition":"tests"})
type IndexDocumentSizeTest struct {
	behaviors *BehaviorContext
}

func (test *IndexDocumentSizeTest) TestAWorstCaseMessageAtEveryCapIsStoredUnder512KiBAndReadsBack() {
	world := SearchTestSetup{}.World()
	message := world.Created(world.Domain, world.WorstCaseMessage("cnv_big"))
	stored := world.Stored(world.Domain, message.MessageId)
	Assert.IsTrue(len(world.StoredJson(world.Domain, message.MessageId)) < world.DocumentLimit(), "the stored document is over 512 KiB")
	Assert.IsTrue(search.DocumentBudget{}.Size(stored) <= search.DocumentBudget{}.MaxBytes(), "the document is over its budget")
	Assert.IsTrue(stored.Truncated, "the worst case text was not cut")
	Assert.Equals(message.MessageId, stored.MessageId)
	Assert.Equals(search.DocumentBudget{}.MaxFileNames(), len(stored.FileNames))
}

func (test *IndexDocumentSizeTest) TestATextJustUnderTheByteCapIsStoredWhole() {
	world := SearchTestSetup{}.World()
	message := world.Created(world.Domain, world.Message("cnv_one", strings.Repeat("<", 80000), "2026-11-10T10:00:00Z"))
	stored := world.Stored(world.Domain, message.MessageId)
	Assert.IsTrue(!stored.Truncated, "a text under the cap was cut")
	Assert.Equals(80000, len(stored.Text))
	Assert.IsTrue(len(world.StoredJson(world.Domain, message.MessageId)) < world.DocumentLimit(), "the stored document is over 512 KiB")
}

func (test *IndexDocumentSizeTest) TestATextJustOverTheByteCapIsCutToFit() {
	world := SearchTestSetup{}.World()
	message := world.Created(world.Domain, world.Message("cnv_one", strings.Repeat("<", 82000), "2026-11-10T10:00:00Z"))
	stored := world.Stored(world.Domain, message.MessageId)
	Assert.IsTrue(stored.Truncated, "a text over the cap was not cut")
	Assert.IsTrue(len(stored.Text) < 82000, "the text keeps its full length")
	Assert.IsTrue(search.DocumentBudget{}.Size(stored) <= search.DocumentBudget{}.MaxBytes(), "the document is over its budget")
	Assert.IsTrue(len(world.StoredJson(world.Domain, message.MessageId)) < world.DocumentLimit(), "the stored document is over 512 KiB")
}

func (test *IndexDocumentSizeTest) TestAnEditOverTheByteCapIsCutToFit() {
	world := SearchTestSetup{}.World()
	message := world.Created(world.Domain, world.Message("cnv_one", "short words", "2026-11-10T10:00:00Z"))
	world.Edit(world.Domain, message, world.WorstCaseText(search.DocumentBudget{}.TextCharacters()))
	stored := world.Stored(world.Domain, message.MessageId)
	Assert.IsTrue(stored.Truncated, "the long edit was not cut")
	Assert.IsTrue(len(world.StoredJson(world.Domain, message.MessageId)) < world.DocumentLimit(), "the stored document is over 512 KiB")
}
