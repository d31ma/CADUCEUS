// @Test(ShellTestSetup, {"partition":"tests"})
type ExtensionRuntimeServiceTest struct {
	behaviors *BehaviorContext
	// @Fake
	extensionsClient *ExtensionsClient
}

func (test *ExtensionRuntimeServiceTest) readyRuntime() *ExtensionRuntimeService {
	runtime := &ExtensionRuntimeService{}
	test.construct(runtime)
	runtime.reset()
	runtime.plugins = []InstalledExtension{
		{ID: "one", Manifest: ExtensionManifest{Contributions: []ExtensionContribution{{ID: "command-one", Label: "First command", Slot: "toolbar"}}}},
		{ID: "two", Manifest: ExtensionManifest{Contributions: []ExtensionContribution{{ID: "command-two", Label: "Second command", Slot: "toolbar"}}}},
	}
	runtime.sandboxTokens = map[string]string{"one": "current-one", "two": "current-two"}
	runtime.HearAnswer(ExtensionAnswered{Message: `{"extensionId":"one","sessionNonce":"current-one","type":"ready"}`})
	runtime.HearAnswer(ExtensionAnswered{Message: `{"extensionId":"two","sessionNonce":"current-two","type":"ready"}`})
	Assert.Equals(2, len(runtime.Toolbar))
	Assert.Equals(false, runtime.Toolbar[0].Disabled)
	Assert.Equals(false, runtime.Toolbar[1].Disabled)
	return runtime
}

func (test *ExtensionRuntimeServiceTest) TestStoppedDisablesOnlyTheStoppedExtension() {
	runtime := test.readyRuntime()
	runtime.HearAnswer(ExtensionAnswered{Message: `{"extensionId":"one","sessionNonce":"current-one","type":"stopped","requestId":"timed-out-command","error":"Extension command timed out"}`})
	Assert.IsTrue(runtime.Toolbar[0].Disabled)
	Assert.Equals(false, runtime.Toolbar[1].Disabled)
}

func (test *ExtensionRuntimeServiceTest) TestRecoverableCommandErrorKeepsCommandsReady() {
	runtime := test.readyRuntime()
	runtime.HearAnswer(ExtensionAnswered{Message: `{"extensionId":"one","sessionNonce":"current-one","type":"error","requestId":"failed-command","error":"Command failed"}`})
	Assert.Equals(false, runtime.Toolbar[0].Disabled)
	Assert.Equals(false, runtime.Toolbar[1].Disabled)
}

func (test *ExtensionRuntimeServiceTest) TestStoppedFromThePreviousNonceIsIgnored() {
	runtime := test.readyRuntime()
	runtime.HearAnswer(ExtensionAnswered{Message: `{"extensionId":"one","sessionNonce":"previous-one","type":"stopped","error":"Extension worker failed"}`})
	Assert.Equals(false, runtime.Toolbar[0].Disabled)
	Assert.Equals(false, runtime.Toolbar[1].Disabled)
}

func (test *ExtensionRuntimeServiceTest) TestStoppedFromAnUnknownExtensionIsIgnored() {
	runtime := test.readyRuntime()
	runtime.HearAnswer(ExtensionAnswered{Message: `{"extensionId":"missing","sessionNonce":"current-one","type":"stopped","error":"Extension worker failed"}`})
	Assert.Equals(false, runtime.Toolbar[0].Disabled)
	Assert.Equals(false, runtime.Toolbar[1].Disabled)
}

func (test *ExtensionRuntimeServiceTest) TestStoppedWithoutANonceIsIgnored() {
	runtime := test.readyRuntime()
	runtime.HearAnswer(ExtensionAnswered{Message: `{"extensionId":"one","type":"stopped","error":"Extension worker failed"}`})
	Assert.Equals(false, runtime.Toolbar[0].Disabled)
	Assert.Equals(false, runtime.Toolbar[1].Disabled)
}
