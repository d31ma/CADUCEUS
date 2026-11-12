import (
	"mesail.dev/packages/core"
)

// @Test(StorageTestSetup, {"partition":"tests"})
type MessageSizeTest struct {
	behaviors *BehaviorContext
}

func (test *MessageSizeTest) TestAMessageOfQuotesAtTheBudgetIsStoredWithItsEventUnderTheDocumentLimit() {
	world := StorageTestSetup{}.World()
	body, blocks := world.ContentAtTheBudget(`"`)
	Assert.IsTrue(core.MessageBudget{}.ContentBytes(body, blocks) > core.MessageBudget{}.MaxContentBytes()-world.BudgetSlack(), "the content is not at the cap")
	stored := world.StoredWithCreatedEvent(body, blocks)
	Assert.Equals(body, stored.Body)
	Assert.Equals(1, len(stored.PendingEvents))
	Assert.IsTrue(world.EncodedSize(stored) < world.MaxDocumentBytes(), "the stored message is over 512 KiB")
}

func (test *MessageSizeTest) TestAMessageOfAngleBracketsAtTheBudgetIsStoredWithItsEventUnderTheDocumentLimit() {
	world := StorageTestSetup{}.World()
	body, blocks := world.ContentAtTheBudget("<")
	Assert.IsTrue(core.MessageBudget{}.ContentBytes(body, blocks) > core.MessageBudget{}.MaxContentBytes()-world.BudgetSlack(), "the content is not at the cap")
	stored := world.StoredWithCreatedEvent(body, blocks)
	Assert.Equals(blocks, stored.BlocksJson)
	Assert.IsTrue(world.EncodedSize(stored) < world.MaxDocumentBytes(), "the stored message is over 512 KiB")
}

func (test *MessageSizeTest) TestAMessageOfWideCharactersAtTheBudgetIsStoredWithItsEventUnderTheDocumentLimit() {
	world := StorageTestSetup{}.World()
	body, blocks := world.ContentAtTheBudget("\U0001F600")
	Assert.IsTrue(core.MessageBudget{}.ContentBytes(body, blocks) > core.MessageBudget{}.MaxContentBytes()-world.BudgetSlack(), "the content is not at the cap")
	stored := world.StoredWithCreatedEvent(body, blocks)
	Assert.Equals(body, stored.Body)
	Assert.IsTrue(world.EncodedSize(stored) < world.MaxDocumentBytes(), "the stored message is over 512 KiB")
}
