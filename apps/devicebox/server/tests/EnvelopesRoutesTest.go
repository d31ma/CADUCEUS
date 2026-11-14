import "time"

// @Test(DeviceboxTestSetup, {"partition":"tests"})
type EnvelopesRoutesTest struct {
	behaviors *BehaviorContext
}

func (test *EnvelopesRoutesTest) TestAPutIsListedForEachDeviceAndAcknowledgedOneDeviceAtATime() {
	world := DeviceboxTestSetup{}.World()
	defer world.Close()
	world.Owns("ana", "a1")
	world.Owns("bo", "b1", "b2")
	request := world.Request("a1", "b1", "b2")
	receipt, err := world.Envelopes.PostEnvelope(world.Member("ana"), request)
	Assert.IsTrue(err == nil, "put failed")
	Assert.Equals(request.EnvelopeId, receipt.EnvelopeId)
	page, err := world.Envelopes.GetEnvelopes(world.Member("bo"), "b1", "", "")
	Assert.IsTrue(err == nil, "list failed")
	Assert.Equals(1, len(page.Envelopes))
	Assert.Equals(request.Ciphertext, page.Envelopes[0].Ciphertext)
	Assert.Equals("ana", page.Envelopes[0].SenderPersonId)
	Assert.IsTrue(world.Envelopes.DeleteEnvelopes(world.Member("bo"), "b1", EnvelopeAck{EnvelopeIds: []string{request.EnvelopeId}}) == nil, "ack failed")
	Assert.Equals(0, len(world.Listed("bo", "b1")))
	Assert.Equals([]string{request.EnvelopeId}, world.Listed("bo", "b2"))
}

func (test *EnvelopesRoutesTest) TestAPutStagesEnvelopeStoredForRealtimeAndNotify() {
	world := DeviceboxTestSetup{}.World()
	defer world.Close()
	world.Owns("ana", "a1")
	world.Owns("bo", "b1")
	world.Put("ana", "a1", "b1")
	Assert.Equals(1, len(world.Pusher.PushedTo("realtime")))
	Assert.Equals(1, len(world.Pusher.PushedTo("notify")))
	Assert.Equals("envelope.stored", world.Pusher.PushedTo("notify")[0].Type)
}

func (test *EnvelopesRoutesTest) TestTheSameIdWithOtherContentIsADuplicate() {
	world := DeviceboxTestSetup{}.World()
	defer world.Close()
	world.Owns("ana", "a1")
	world.Owns("bo", "b1")
	request := world.Request("a1", "b1")
	world.Envelopes.PostEnvelope(world.Member("ana"), request)
	request.Ciphertext = world.Ciphertext(48)
	_, err := world.Envelopes.PostEnvelope(world.Member("ana"), request)
	Assert.Equals(6012, world.NumberOf(err))
}

func (test *EnvelopesRoutesTest) TestAnotherDomainsMemberCannotUseTheBox() {
	world := DeviceboxTestSetup{}.World()
	defer world.Close()
	world.Owns("ana", "a1")
	world.Owns("bo", "b1")
	id := world.Put("ana", "a1", "b1")
	_, listError := world.Envelopes.GetEnvelopes(world.Stranger("bo"), "b1", "", "")
	ackError := world.Envelopes.DeleteEnvelopes(world.Stranger("bo"), "b1", EnvelopeAck{EnvelopeIds: []string{id}})
	_, putError := world.Envelopes.PostEnvelope(world.Stranger("ana"), world.Request("a1", "b1"))
	Assert.Equals(6014, world.NumberOf(listError))
	Assert.Equals(6014, world.NumberOf(ackError))
	Assert.Equals(6010, world.NumberOf(putError))
	Assert.Equals([]string{id}, world.Listed("bo", "b1"))
}

func (test *EnvelopesRoutesTest) TestLimitsAnswerTheirNumbers() {
	world := DeviceboxTestSetup{}.World()
	defer world.Close()
	world.Owns("ana", "a1")
	world.Owns("bo", "b1")
	large := world.Request("a1", "b1")
	large.Ciphertext = world.Ciphertext(256*1024 + 1)
	_, tooLarge := world.Envelopes.PostEnvelope(world.Member("ana"), large)
	_, badLimit := world.Envelopes.GetEnvelopes(world.Member("bo"), "b1", "", "101")
	tooMany := world.Envelopes.DeleteEnvelopes(world.Member("bo"), "b1", EnvelopeAck{EnvelopeIds: make([]string, 11)})
	Assert.Equals(6009, world.NumberOf(tooLarge))
	Assert.Equals(6016, world.NumberOf(badLimit))
	Assert.Equals(6017, world.NumberOf(tooMany))
}

func (test *EnvelopesRoutesTest) TestAForgedTokenIsRefused() {
	world := DeviceboxTestSetup{}.World()
	defer world.Close()
	_, err := world.Envelopes.PostEnvelope("Bearer forged", world.Request("a1", "b1"))
	Assert.Equals(6001, world.NumberOf(err))
}

func (test *EnvelopesRoutesTest) TestTheExpiryJobLeavesAnUndeliveredNoticeForTheSender() {
	world := DeviceboxTestSetup{}.World()
	defer world.Close()
	world.Owns("ana", "a1")
	world.Owns("bo", "b1")
	id := world.Put("ana", "a1", "b1")
	world.Later(31 * 24 * time.Hour)
	report, err := world.Jobs.PostExpire(world.Service())
	Assert.IsTrue(err == nil, "expire failed")
	Assert.IsTrue(report.Writes > 0, "the job wrote nothing")
	Assert.IsTrue(!report.More, "the job wants more calls")
	Assert.Equals(0, len(world.Listed("bo", "b1")))
	page, _ := world.Envelopes.GetEnvelopes(world.Member("ana"), "a1", "", "")
	Assert.Equals(1, len(page.Envelopes))
	Assert.Equals(id, page.Envelopes[0].EnvelopeId)
	Assert.Equals("undelivered", page.Envelopes[0].Kind)
}
