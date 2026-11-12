package main

import (
	"strings"
	"testing"
)

const passingController = `import "net/http"

// @RequestRoute("/api/v1/sample/messages")
// @SignedIn
type MessagesController struct {
	// @Needs
	messages *MessageService
}

// @PostRoute
func (controller *MessagesController) PostMessage(
	// @FromNative
	request *http.Request,
	// @FromBody
	body NewMessage,
) (Message, error) {
	return controller.messages.Post(request, body)
}

// @GetRoute
func (controller MessagesController) GetMessages(request *http.Request) ([]Message, error) {
	return controller.messages.List(request)
}

// @GetRoute("/count")
func (controller *MessagesController) GetCount() int {
	return controller.messages.Count()
}

// @GetRoute("/stream")
func (controller *MessagesController) GetStream(writer http.ResponseWriter, request *http.Request) {
	controller.messages.Stream(writer, request)
}

func (controller *MessagesController) helper(value int) int {
	if value > 1 {
		return value
	}
	return 0
}
`

func problemsIn(t *testing.T, source string) []Problem {
	t.Helper()
	problems, err := SourceFile{Name: "apps/sample/server/controllers/SampleController.go", Source: []byte(source)}.Problems()
	if err != nil {
		t.Fatal(err)
	}
	return problems
}

func onlyReason(t *testing.T, method string) string {
	t.Helper()
	source := `type SampleController struct {
	// @Needs
	service *SampleService
	// @Needs
	other, third *OtherService
	plain int
}

` + method
	problems := problemsIn(t, source)
	if len(problems) != 1 {
		t.Fatalf("want one problem, got %v", problems)
	}
	return problems[0].Reason
}

func assertReason(t *testing.T, method string, want string) {
	t.Helper()
	if got := onlyReason(t, method); !strings.HasPrefix(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestAPassingControllerHasNoProblems(t *testing.T) {
	if problems := problemsIn(t, passingController); len(problems) != 0 {
		t.Fatalf("got %v", problems)
	}
}

func TestACallOnAFieldNamedWithOthersPasses(t *testing.T) {
	source := `type SampleController struct {
	other, third *OtherService
}

func (controller *SampleController) GetThird(id string) (Item, error) {
	return controller.third.Find(id)
}
`
	if problems := problemsIn(t, source); len(problems) != 0 {
		t.Fatalf("got %v", problems)
	}
}

func TestTwoStatementsFail(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetItem(id string) (Item, error) {
	clean := id
	return controller.service.Find(clean)
}
`, "has 2 statements")
}

func TestAnEmptyBodyFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) DeleteItem() {
}
`, "has 0 statements")
}

func TestAnIfStatementFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetItem(id string) error {
	if id == "" {
		return nil
	}
}
`, "is not a return statement")
}

func TestReturningTwoValuesFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetItem(id string) (Item, error) {
	return Item{}, nil
}
`, "returns 2 values")
}

func TestABareReturnFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) DeleteItem(id string) (err error) {
	return
}
`, "returns 0 values")
}

func TestReturningALiteralFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetHealth() Health {
	return Health{Status: "ok"}
}
`, "returns something that is not a call")
}

func TestCallingAPackageFunctionFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetName(name string) string {
	return strings.ToLower(name)
}
`, "does not call a method on one of the controller's own fields")
}

func TestCallingTheControllersOwnMethodFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetItem(id string) (Item, error) {
	return controller.find(id)
}
`, "does not call a method on one of the controller's own fields")
}

func TestCallingAFunctionValueFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetItem(id string) (Item, error) {
	return find(id)
}
`, "does not call a method on one of the controller's own fields")
}

func TestCallingAFieldThatDoesNotExistFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetItem(id string) (Item, error) {
	return controller.missing.Find(id)
}
`, "does not call a method on one of the controller's own fields")
}

func TestCallingAFieldOfAnotherValueFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetItem(id string, other SampleController) (Item, error) {
	return other.service.Find(id)
}
`, "does not call a method on one of the controller's own fields")
}

func TestAChainedCallFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetItem(id string) (Item, error) {
	return controller.service.Items().Find(id)
}
`, "does not call a method on one of the controller's own fields")
}

func TestAnUnnamedReceiverFails(t *testing.T) {
	assertReason(t, `func (*SampleController) GetItem(id string) (Item, error) {
	return service.Find(id)
}
`, "does not call a method on one of the controller's own fields")
}

func TestANestedCallInAnArgumentFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) PostItem(authorization string, body NewItem) (Item, error) {
	return controller.service.Add(controller.other.Admit(authorization), body)
}
`, "passes argument 1 that is not one of the method's own parameters")
}

func TestALiteralArgumentFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetItem(id string) (Item, error) {
	return controller.service.Find(id, 10)
}
`, "passes argument 2 that is not one of the method's own parameters")
}

func TestAFieldArgumentFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetItem(id string) (Item, error) {
	return controller.service.Find(controller.plain)
}
`, "passes argument 1 that is not one of the method's own parameters")
}

func TestABareCallInAMethodWithResultsFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) GetItem(id string) error {
	controller.service.Find(id)
}
`, "is not a return statement")
}

func TestABareExpressionThatIsNotACallFails(t *testing.T) {
	assertReason(t, `func (controller *SampleController) DeleteItem(id string) {
	<-controller.done
}
`, "is not a return statement")
}

func TestUnexportedMethodsAndFunctionsAreNotChecked(t *testing.T) {
	source := `type SampleController struct{}

func (controller *SampleController) helper() int {
	x := 1
	return x
}

func Build() *SampleController {
	value := &SampleController{}
	return value
}
`
	if problems := problemsIn(t, source); len(problems) != 0 {
		t.Fatalf("got %v", problems)
	}
}

func TestMethodsOfTypesFromOtherFilesAreNotChecked(t *testing.T) {
	source := `func (service *SampleService) Find(id string) (Item, error) {
	clean := id
	return service.store.Find(clean)
}
`
	if problems := problemsIn(t, source); len(problems) != 0 {
		t.Fatalf("got %v", problems)
	}
}

func TestEveryBadMethodIsReported(t *testing.T) {
	source := `type SampleController struct {
	service *SampleService
}

func (controller *SampleController) GetOne() Item {
	return Item{}
}

func (controller *SampleController) GetTwo() Item {
	return Item{}
}
`
	if problems := problemsIn(t, source); len(problems) != 2 {
		t.Fatalf("got %v", problems)
	}
}
