// @Test(SessionTestSetup, {"partition":"tests"})
type SessionProofServiceTest struct {
	behaviors *BehaviorContext
}

func (test *SessionProofServiceTest) TestNewKeyFollowsTheStandardExample() {
	BrowserFakes{}.Answer("secureRandom", "bytes", []int{116, 24, 223, 180, 151, 153, 224, 37, 79, 250, 96, 125, 216, 173, 187, 186, 22, 212, 37, 77, 105, 214, 191, 240, 91, 88, 5, 88, 83, 132, 141, 121})
	key, failure := (&SessionProofService{}).NewKey()
	Assert.Equals(nil, failure)
	Assert.Equals("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk", key.Verifier)
	Assert.Equals("E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", key.Challenge)
}

func (test *SessionProofServiceTest) TestNewKeyMakesAStateOfTheSameLength() {
	BrowserFakes{}.Answer("secureRandom", "bytes", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32})
	key, _ := (&SessionProofService{}).NewKey()
	Assert.Equals(43, len(key.State))
}
