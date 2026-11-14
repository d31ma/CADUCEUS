package main

import (
	"strings"
	"testing"
)

func TestAPlainValueHasNoChoice(t *testing.T) {
	if (ViewExpression{Text: "row.ariaCurrent"}).HasChoice() {
		t.Fatal("want no choice")
	}
}

func TestATernaryHasAChoice(t *testing.T) {
	if !(ViewExpression{Text: "row.current ? 'page' : false"}).HasChoice() {
		t.Fatal("want a choice")
	}
}

func TestATernaryWithoutSpacesHasAChoice(t *testing.T) {
	if !(ViewExpression{Text: "a?b:c"}).HasChoice() {
		t.Fatal("want a choice")
	}
}

func TestATernaryWithADecimalNumberHasAChoice(t *testing.T) {
	if !(ViewExpression{Text: "a?.5:1"}).HasChoice() {
		t.Fatal("want a choice")
	}
}

func TestOptionalChainingHasNoChoice(t *testing.T) {
	if (ViewExpression{Text: "message.email?.subject"}).HasChoice() {
		t.Fatal("want no choice")
	}
}

func TestADefaultValueHasNoChoice(t *testing.T) {
	if (ViewExpression{Text: "row.title ?? 'Untitled'"}).HasChoice() {
		t.Fatal("want no choice")
	}
}

func TestAQuestionMarkInsideAStringHasNoChoice(t *testing.T) {
	if (ViewExpression{Text: "'Delete ' + row.name + '?'"}).HasChoice() {
		t.Fatal("want no choice")
	}
}

func TestAQuestionMarkInsideADoubleQuotedStringHasNoChoice(t *testing.T) {
	if (ViewExpression{Text: `"Sure? Yes: go"`}).HasChoice() {
		t.Fatal("want no choice")
	}
}

func TestAQuestionMarkInsideAnEscapedStringHasNoChoice(t *testing.T) {
	if (ViewExpression{Text: `'it\'s ok?' + x`}).HasChoice() {
		t.Fatal("want no choice")
	}
}

func TestAQuestionMarkInsideATemplateStringHasNoChoice(t *testing.T) {
	if (ViewExpression{Text: "`ready?`"}).HasChoice() {
		t.Fatal("want no choice")
	}
}

func TestAProblemNamesTheFileTheLineAndTheExpression(t *testing.T) {
	problem := ViewExpression{Text: " a ? b : c ", Line: 7}.Problem("tac.html")
	want := "tac.html:7: the view holds a ? : expression (a ? b : c); give the result a service field and show that field"
	if problem.String() != want {
		t.Fatalf("got %q", problem.String())
	}
}

func TestAPlainValueHasNoJoin(t *testing.T) {
	if (ViewExpression{Text: "accountAction.deleteHeading"}).HasJoin() {
		t.Fatal("want no join")
	}
}

func TestTextBuiltWithAPlusHasAJoin(t *testing.T) {
	if !(ViewExpression{Text: "'Delete ' + accountAction.name + '?'"}).HasJoin() {
		t.Fatal("want a join")
	}
}

func TestTwoFieldsJoinedWithoutSpacesHaveAJoin(t *testing.T) {
	if !(ViewExpression{Text: "row.first+row.last"}).HasJoin() {
		t.Fatal("want a join")
	}
}

func TestAPlusInsideAStringHasNoJoin(t *testing.T) {
	if (ViewExpression{Text: "list.open('a+b')"}).HasJoin() {
		t.Fatal("want no join")
	}
}

func TestAJoinProblemNamesTheExpression(t *testing.T) {
	problem := ViewExpression{Text: " 'Remove ' + item.name ", Line: 4}.JoinProblem("view/tac.html")
	if problem.Line != 4 || !strings.Contains(problem.Reason, "builds a value with + ('Remove ' + item.name)") {
		t.Fatalf("got %v", problem)
	}
}
