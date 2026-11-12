package main

import (
	"strings"
	"testing"
)

func TestAProblemNamesTheFileAndTheMethodLine(t *testing.T) {
	source := `// @RequestRoute("/api/v1/sample")
type SampleController struct{}

// @GetRoute
func (controller *SampleController) GetHealth() Health {
	return Health{}
}
`
	problems, err := SourceFile{Name: "apps/sample/server/controllers/SampleController.go", Source: []byte(source)}.Problems()
	want := "apps/sample/server/controllers/SampleController.go:5: SampleController.GetHealth returns something that is not a call; it must return one Service call"
	if err != nil || len(problems) != 1 || problems[0].String() != want {
		t.Fatalf("got %v, %v", problems, err)
	}
}

func TestASourceThatDoesNotParseIsAnError(t *testing.T) {
	_, err := SourceFile{Name: "apps/sample/server/controllers/BrokenController.go", Source: []byte("type BrokenController struct {\n")}.Problems()
	if err == nil || !strings.Contains(err.Error(), "BrokenController.go:") {
		t.Fatalf("got %v", err)
	}
}
