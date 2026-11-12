package main

import (
	"reflect"
	"testing"
)

const viewWithEveryExpressionKind = `<div :data-x="row.x" :title='row.title' on:click="list.open(row.id)">
  <span>{row.name}</span>
  <w-card heading="Delete this? Yes: really" label="{ plain"></w-card>
</div>
`

func TestExpressionsFindsBoundAttributesHandlersAndTextValues(t *testing.T) {
	got := ViewFile{Name: "tac.html", Source: viewWithEveryExpressionKind}.expressions()
	want := []ViewExpression{
		{Text: "row.x", Line: 1},
		{Text: "row.title", Line: 1},
		{Text: "list.open(row.id)", Line: 1},
		{Text: "row.name", Line: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestAPlainAttributeWithAQuestionMarkIsNotAProblem(t *testing.T) {
	problems := ViewFile{Name: "tac.html", Source: viewWithEveryExpressionKind}.Problems()
	if len(problems) != 0 {
		t.Fatalf("got %v", problems)
	}
}

func TestATernaryInABoundAttributeIsAProblem(t *testing.T) {
	problems := ViewFile{Name: "tac.html", Source: "<p>\n  <a :aria-current=\"row.current ? 'page' : false\"></a>\n</p>\n"}.Problems()
	if len(problems) != 1 || problems[0].Line != 2 || problems[0].File != "tac.html" {
		t.Fatalf("got %v", problems)
	}
}

func TestATernaryInASingleQuotedAttributeIsAProblem(t *testing.T) {
	problems := ViewFile{Name: "tac.html", Source: `<a :title='row.on ? "On" : "Off"'></a>`}.Problems()
	if len(problems) != 1 {
		t.Fatalf("got %v", problems)
	}
}

func TestATernaryInAHandlerIsAProblem(t *testing.T) {
	problems := ViewFile{Name: "tac.html", Source: `<a on:click="list.open(row.on ? 'a' : 'b')"></a>`}.Problems()
	if len(problems) != 1 {
		t.Fatalf("got %v", problems)
	}
}

func TestATernaryInAControlTagIsAProblem(t *testing.T) {
	problems := ViewFile{Name: "tac.html", Source: `<logic :if="row.a ? row.b : row.c"></logic>`}.Problems()
	if len(problems) != 1 {
		t.Fatalf("got %v", problems)
	}
}

func TestATernaryInATextValueIsAProblem(t *testing.T) {
	problems := ViewFile{Name: "tac.html", Source: "<label>{composer.emailMode ? 'Email body' : 'Message'}</label>"}.Problems()
	if len(problems) != 1 || problems[0].Line != 1 {
		t.Fatalf("got %v", problems)
	}
}

func TestEachTernaryIsReportedOnItsOwnLine(t *testing.T) {
	problems := ViewFile{Name: "tac.html", Source: "<a :x=\"a ? b : c\"></a>\n<b>ok</b>\n<a :y=\"d ? e : f\"></a>\n"}.Problems()
	if len(problems) != 2 || problems[0].Line != 1 || problems[1].Line != 3 {
		t.Fatalf("got %v", problems)
	}
}
