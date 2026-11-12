package search

import (
	"strings"
	"testing"
	"unicode/utf8"

	"mesail.dev/packages/events"
)

func worstCaseText(characters int) string {
	var text strings.Builder
	for index := 0; index < characters/2; index++ {
		text.WriteRune(rune(0x20000 + index%0xA6D0))
		text.WriteString("<")
	}
	return text.String()
}

func worstCaseMessage() events.MessageCreated {
	created := message("msg_big", "cnv_1", worstCaseText(DocumentBudget{}.TextCharacters()), "2026-11-05T10:00:00Z")
	created.Subject = worstCaseText(DocumentBudget{}.LabelCharacters() + 10)
	created.AuthorDisplayName = worstCaseText(DocumentBudget{}.LabelCharacters() + 10)
	created.AuthorAddress = strings.Repeat("<", DocumentBudget{}.LabelCharacters()+10)
	for index := 0; index < (DocumentBudget{}).MaxFileNames()+5; index++ {
		created.FileNames = append(created.FileNames, worstCaseText(DocumentBudget{}.FileNameCharacters()+10))
	}
	return created
}

func TestTheWorstCaseDocumentFitsTheByteBudget(t *testing.T) {
	document, _, err := DocumentBuilder{}.FromCreated(domainA, worstCaseMessage())
	if err != nil {
		t.Fatal(err)
	}
	budget := DocumentBudget{}
	if budget.Size(document) > budget.MaxBytes() || budget.MaxBytes() >= 512<<10 || !document.Truncated {
		t.Fatalf("size %d, truncated %v", budget.Size(document), document.Truncated)
	}
	if len(document.FileNames) != budget.MaxFileNames() || utf8.RuneCountInString(document.FileNames[0]) != budget.FileNameCharacters() {
		t.Fatalf("file names %d, first %d characters", len(document.FileNames), utf8.RuneCountInString(document.FileNames[0]))
	}
	labels := []string{document.Subject, document.AuthorName, document.AuthorAddress}
	for _, label := range labels {
		if utf8.RuneCountInString(label) != budget.LabelCharacters() {
			t.Fatalf("label has %d characters", utf8.RuneCountInString(label))
		}
	}
}

func TestATextOverTheCharacterCapIsCutToTheCap(t *testing.T) {
	limit := DocumentBudget{}.TextCharacters()
	document, _, _ := DocumentBuilder{}.FromCreated(domainA, message("msg_1", "cnv_1", strings.Repeat("a", limit+1), "2026-11-05T10:00:00Z"))
	if len(document.Text) != limit || !document.Truncated {
		t.Fatalf("got %d characters, truncated %v", len(document.Text), document.Truncated)
	}
}

func TestATextAtTheCharacterCapIsKeptWhole(t *testing.T) {
	limit := DocumentBudget{}.TextCharacters()
	document, _, _ := DocumentBuilder{}.FromCreated(domainA, message("msg_1", "cnv_1", strings.Repeat("a", limit), "2026-11-05T10:00:00Z"))
	if len(document.Text) != limit || document.Truncated {
		t.Fatalf("got %d characters, truncated %v", len(document.Text), document.Truncated)
	}
}

func TestAnEditIsCutAndAShortEditClearsTheCut(t *testing.T) {
	builder := DocumentBuilder{}
	created, _, _ := builder.FromCreated(domainA, message("msg_1", "cnv_1", "alpha", "2026-11-05T10:00:00Z"))
	long, _ := builder.Edited(created, events.MessageEdited{MessageId: "msg_1", Body: worstCaseText(DocumentBudget{}.TextCharacters()), EditedAt: "2026-11-05T11:00:00Z"})
	short, _ := builder.Edited(long, events.MessageEdited{MessageId: "msg_1", Body: "bravo", EditedAt: "2026-11-05T12:00:00Z"})
	if !long.Truncated || (DocumentBudget{}).Over(long) {
		t.Fatalf("long edit: size %d, truncated %v", DocumentBudget{}.Size(long), long.Truncated)
	}
	if short.Truncated || short.Text != "bravo" {
		t.Fatalf("short edit: %+v", short)
	}
}
